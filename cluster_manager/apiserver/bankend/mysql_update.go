package bankend

import (
	"context"
	"fmt"
	stderror "github.com/pkg/errors"
	"github.com/upmio/dbscale-kube/pkg/structs"
	"strconv"
	// "strconv"
	//"strconv"
	"strings"
	"time"

	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/api"
	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/model"
	unitv4 "github.com/upmio/dbscale-kube/pkg/apis/unit/v1alpha4"
	podutil "github.com/upmio/dbscale-kube/pkg/utils/pod"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
)

var defaultRetries = 3

var appUpdateRequestsChan = make(chan string)

const appUpdateRequestsChanTake = "take"
const appUpdateRequestsChanPut = "put"

type resourceRecord struct {
	site     string
	hostname string
	cpuReq   int64
	cpuFree  int64
	memReq   int64
	memFree  int64
	storage  map[string]struct {
		level    string
		diskReq  int64
		diskFree int64
	}
	originalUnschedulable bool
	pods                  []string
}

func init() {
	go func() {
		inUse := false
		for {
			select {
			case req := <-appUpdateRequestsChan:
				{
					if req == appUpdateRequestsChanTake {
						if inUse {
							appUpdateRequestsChan <- "nok"
						} else {
							inUse = true
							appUpdateRequestsChan <- "ok"
						}
					} else {
						inUse = false
					}
				}
			}
		}

	}()
}

func NewResourceRecord(site, hostName string, cpu, memory int64, unschedulable bool) resourceRecord {
	return resourceRecord{
		site:     site,
		hostname: hostName,
		cpuReq:   0,
		cpuFree:  cpu,
		memReq:   0,
		memFree:  memory,
		storage: make(map[string]struct {
			level    string
			diskReq  int64
			diskFree int64
		}),
		originalUnschedulable: unschedulable,
		pods:                  []string{},
	}
}

func updateResourcesFit2groups(groups map[string]api.ResourceRequirements, appName, serviceType string, dbSpec *api.GroupSpec, optSpec *struct {
	Services struct {
		Units struct {
			Resources struct {
				Requests api.ResourceRequirementsOptions `json:"requests,omitempty"`
			} `json:"resources,omitempty"`
		} `json:"units,omitempty"`
	} `json:"services,omitempty"`
}) error {
	origin := dbSpec.Services.Units.Resources.Requests
	requests := optSpec.Services.Units.Resources.Requests
	origin, err := mergeAppResourceRequest(appName, origin, requests)
	if err != nil {
		return err
	}

	groups[serviceType] = origin
	return nil
}

func checkResourceSufficiency(hostsAndResourceRecord map[string]resourceRecord) []error {
	var errs []error
	for _, hostAndResRecord := range hostsAndResourceRecord {
		klog.Info(hostAndResRecord)

		// -1 is due to if assign all allocatable to pod, it will report outOfCPU after pod is started
		// so I change all types to free - 1
		if hostAndResRecord.cpuReq >= hostAndResRecord.cpuFree {
			errs = append(errs, stderror.Errorf("CPU on Host %s is not enough, has %d Free but requires %d, Pod(s) on that host is [%s]",
				hostAndResRecord.hostname, hostAndResRecord.cpuFree-1, hostAndResRecord.cpuReq,
				strings.Join(hostAndResRecord.pods, ",")))
		}
		if hostAndResRecord.memReq >= hostAndResRecord.memFree {
			errs = append(errs, stderror.Errorf("Memory on Host %s is not enough, has %d Free but requires %d, Pod(s) on that host is [%s]",
				hostAndResRecord.hostname, hostAndResRecord.memFree-1, hostAndResRecord.memReq,
				strings.Join(hostAndResRecord.pods, ",")))
		}

		for _, storage := range hostAndResRecord.storage {
			if storage.diskReq >= storage.diskFree {
				errs = append(errs, stderror.Errorf("Storage on Host %s is not enough, storage performance is %s ,"+
					"has %d Free but requires %d, Pod(s) on that host is [%s]",
					hostAndResRecord.hostname, storage.level, storage.diskFree-1, storage.diskReq,
					strings.Join(hostAndResRecord.pods, ",")))
			}
		}
	}

	return errs
}

func (beApp *bankendApp) CheckAppModel(id string) (model.Application, model.Site, api.AppSpec, error) {
	app, err := beApp.m.Get(id)
	if err != nil {
		return model.Application{}, model.Site{}, api.AppSpec{}, err
	}

	site, _ := beApp.sites.Get(beApp.GetSiteStr())
	if err != nil {
		return model.Application{}, model.Site{}, api.AppSpec{}, err
	}

	spec, err := decodeAppSpec(app.Spec)
	if err != nil {
		return model.Application{}, model.Site{}, api.AppSpec{}, err
	}

	return app, site, spec, nil
}

func (beApp *bankendApp) UpdateArch(ctx context.Context, id string, opts api.AppArchOptions) (api.TaskObjectResponse, error) {
	return api.TaskObjectResponse{}, nil
}

func (beApp *bankendApp) UpdateState(ctx context.Context, id string, opts api.AppStateOptions) (api.TaskObjectResponse, error) {
	app, _, spec, err := beApp.CheckAppModel(id)
	if err != nil {
		return api.TaskObjectResponse{}, err
	}

	spec.Database.Services.Units.ReadinessState = opts.State

	data, err := encodeAppSpec(spec)
	if err != nil {
		return api.TaskObjectResponse{}, err
	}

	task, err := beApp.m.UpdateSpec(app.ID, data, model.ActionAppStateEdit, opts.User, nil, nil)
	if err != nil {
		return api.TaskObjectResponse{}, err
	}

	wt := beApp.waits.NewWaitTask(app.ID, time.Second*30, func(err error) error {

		tk := taskUpdate(task, err)
		return beApp.m.UpdateAppTask(nil, tk)
	})
	go wt.WithTimeout(time.Minute*2, func() (bool, error) {

		units, err := beApp.syncAppUnits(app.ID, app.Units)
		if err != nil {
			return false, err
		}

		done := 0
		var errs []error

		for _, unit := range units {

			ok, err := beApp.zone.updateUnitAction(beApp.GetSiteStr(), unit.Namespace, unit.Name, opts.State)
			if ok {
				done++
			}
			if err != nil {
				errs = append(errs, err)
			}
		}

		if done == len(units) {
			return true, nil
		}

		return false, utilerrors.NewAggregate(errs)
	})

	return api.TaskObjectResponse{
		ObjectID:   app.ID,
		ObjectName: app.Name,
		TaskID:     task,
	}, nil
}

func (beApp *bankendApp) UpdateUnitState(ctx context.Context, id, unitID string, opts api.AppStateOptions) (api.TaskObjectResponse, error) {
	app, _, _, err := beApp.CheckAppModel(id)
	if err != nil {
		return api.TaskObjectResponse{}, err
	}

	var mu *model.Unit
	for i := range app.Units {
		if app.Units[i].ID == unitID {
			mu = &app.Units[i]
			break
		}
	}

	if mu == nil {
		return api.TaskObjectResponse{}, fmt.Errorf("not found unit %s in App %s", unitID, app.ID)
	}

	task, err := beApp.m.InsertUnitTask(*mu, model.ActionAppUnitStateEdit)
	if err != nil {
		return api.TaskObjectResponse{}, err
	}

	wt := beApp.waits.NewWaitTask(app.ID, time.Second*30, func(err error) error {
		tk := taskUpdate(task, err)
		return beApp.m.UpdateAppTask(nil, tk)
	})
	go wt.WithTimeout(time.Minute*3, func() (bool, error) {

		return beApp.zone.updateUnitAction(mu.Site, mu.Namespace, mu.ObjectName(), opts.State)
	})

	return api.TaskObjectResponse{
		ObjectID:   app.ID,
		ObjectName: app.Name,
		TaskID:     task,
	}, nil
}

func (beApp *bankendApp) UpdateImage(ctx context.Context, id string, opts api.AppImageOptions) (api.TaskObjectResponse, error) {
	app, site, spec, err := beApp.CheckAppModel(id)
	if err != nil {
		return api.TaskObjectResponse{}, err
	}

	_, err = beApp.syncAppUnits(app.ID, app.Units)
	if err != nil {
		return api.TaskObjectResponse{}, err
	}

	ims := make(map[string]model.Image)
	images := make(map[string]string)
	unitImages := make(map[string]string)

	fit2Groups := func(serviceType string, optSpec *struct {
		Image *api.ImageVersion `json:"image,omitempty"`
	}) error {
		if optSpec == nil {
			return nil
		}

		var im model.Image
		var err error
		if strings.ToLower(optSpec.Image.ID) == structs.ImageLatestTag {
			im, err = beApp.images.GetLatest(serviceType, optSpec.Image.Arch)
		} else {
			im, err = beApp.images.Get(optSpec.Image.ID)
		}
		if err != nil {
			return err
		}

		ims[serviceType] = im
		images[serviceType] = im.ImageVersion.ImageWithArch()
		unitImages[serviceType] = fmt.Sprintf("%s/%s/%s", site.ImageRegistry, site.ProjectName, im.ImageVersion.ImageWithArch())
		return nil
	}

	err = fit2Groups(structs.MysqlServiceType, opts.Spec.Database)
	if err != nil {
		return api.TaskObjectResponse{}, err
	}

	err = fit2Groups(structs.CmhaServiceType, opts.Spec.Cmha)
	if err != nil {
		return api.TaskObjectResponse{}, err
	}

	err = fit2Groups(structs.ProxysqlServiceType, opts.Spec.Proxy)
	if err != nil {
		return api.TaskObjectResponse{}, err
	}

	//some sanity check
	if spec.Proxy == nil && opts.Spec.Proxy != nil {
		return api.TaskObjectResponse{}, stderror.New("proxySQL is not used in this app")
	}
	if spec.Cmha == nil && opts.Spec.Cmha != nil {
		return api.TaskObjectResponse{}, stderror.New("cmha is not used in this app")
	}

	if opts.Spec.Database != nil {
		spec.Database.Image = api.ImageVersion(ims[structs.MysqlServiceType].ImageVersion)
	}
	if opts.Spec.Cmha != nil {
		spec.Cmha.Image = api.ImageVersion(ims[structs.CmhaServiceType].ImageVersion)
	}
	if opts.Spec.Proxy != nil {
		spec.Proxy.Image = api.ImageVersion(ims[structs.ProxysqlServiceType].ImageVersion)
	}

	data, err := encodeAppSpec(spec)
	if err != nil {
		return api.TaskObjectResponse{}, err
	}

	task, err := beApp.m.UpdateSpec(app.ID, data, model.ActionAppImageEdit, "", nil, nil)
	if err != nil {
		return api.TaskObjectResponse{}, err
	}

	app.Spec = data

	wt := beApp.waits.NewWaitTask(app.ID, time.Second*30, func(err error) error {
		tk := taskUpdate(task, err)

		return beApp.m.UpdateAppTask(nil, tk)
	})

	go wt.WithTimeout(time.Minute*5, func() (bool, error) {
		var errs []error

		for _, serviceType := range []string{structs.ProxysqlServiceType, structs.MysqlServiceType, structs.CmhaServiceType} {

			_, ok := images[serviceType]
			if !ok {
				continue
			}

			units, err := beApp.syncAppUnitsByType(app.ID, app.Units, serviceType, true)
			if err != nil {
				errs = append(errs, err)
			}

			for i := range units {

				for j, container := range units[i].Spec.Template.Spec.Containers {

					if container.Name != units[i].Spec.MainContainerName {
						continue
					}

					if !strings.HasSuffix(container.Image, images[serviceType]) {

						clone := units[i].DeepCopy()
						clone.Spec.Template.Spec.Containers[j].Image = unitImages[serviceType]

						err := beApp.zone.updateUnit(clone)
						if err != nil {
							errs = append(errs, err)
						}
					}
				}
			}
		}

		return true, utilerrors.NewAggregate(errs)
	})

	return api.TaskObjectResponse{
		ObjectID:   app.ID,
		ObjectName: app.Name,
		TaskID:     task,
	}, nil
}

func (beApp *bankendApp) UpdateAppResourceRequests(ctx context.Context, id string, opts api.AppResourcesOptions) (api.TaskObjectResponse, error) {
	app, _, spec, err := beApp.CheckAppModel(id)
	if err != nil {
		return api.TaskObjectResponse{}, err
	}
	var errs []error

	requestGroups := make(map[string]api.ResourceRequirements)

	if opts.Spec.Database != nil {
		err := updateResourcesFit2groups(requestGroups, app.Name, structs.MysqlServiceType, spec.Database, opts.Spec.Database)
		if err != nil {
			errs = append(errs, err)
		}
	}
	if opts.Spec.Proxy != nil {
		err := updateResourcesFit2groups(requestGroups, app.Name, structs.ProxysqlServiceType, spec.Proxy, opts.Spec.Proxy)
		if err != nil {
			errs = append(errs, err)
		}
	}
	if opts.Spec.Cmha != nil {
		err := updateResourcesFit2groups(requestGroups, app.Name, structs.CmhaServiceType, spec.Cmha, opts.Spec.Cmha)
		if err != nil {
			errs = append(errs, err)
		}
	}

	//some sanity check
	if spec.Proxy == nil && opts.Spec.Proxy != nil {
		return api.TaskObjectResponse{}, stderror.New("proxySQL is not used in this app")
	}
	if spec.Cmha == nil && opts.Spec.Cmha != nil {
		return api.TaskObjectResponse{}, stderror.New("cmha is not used in this app")
	}

	if opts.Spec.Database != nil {
		spec.Database.Services.Units.Resources.Requests = requestGroups[structs.MysqlServiceType]
	}
	if opts.Spec.Cmha != nil {
		spec.Cmha.Services.Units.Resources.Requests = requestGroups[structs.CmhaServiceType]
	}
	if opts.Spec.Proxy != nil {
		spec.Proxy.Services.Units.Resources.Requests = requestGroups[structs.ProxysqlServiceType]
	}

	if len(errs) > 0 {
		return api.TaskObjectResponse{}, utilerrors.NewAggregate(errs)
	}

	// try to get pass
	appUpdateRequestsChan <- appUpdateRequestsChanTake
	passOrNot := <-appUpdateRequestsChan
	if passOrNot == "nok" {
		return api.TaskObjectResponse{}, stderror.Errorf("there is ongoing expansion")
	}

	// pre-check and set hosts to unschedulable = true
	var hostsAndResourceRecord = make(map[string]resourceRecord)
	var allowRestore = true
	defer func() {
		klog.Infof("defer is called with allowRestore = %t", allowRestore)
		if allowRestore {
			for _, host := range hostsAndResourceRecord {
				klog.Infof("host = %s unschedulable = %t", host.hostname, host.originalUnschedulable)
				beApp.zone.setHostUnschedulable(host.site, host.hostname, host.originalUnschedulable)
			}
			appUpdateRequestsChan <- appUpdateRequestsChanPut
		}
	}()

	for i := range app.Units {
		_, err := beApp.zone.calculateUnitResourceDelta(requestGroups[app.Units[i].GetServiceType()],
			app.Units[i].Site, app.Units[i].Namespace, app.Units[i].ObjectName(), hostsAndResourceRecord)

		if err != nil {
			errs = append(errs, err)
		}
	}

	checkResourceErrs := checkResourceSufficiency(hostsAndResourceRecord)
	for _, err := range checkResourceErrs {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return api.TaskObjectResponse{}, utilerrors.NewAggregate(errs)
	}

	// do update only when hosts conditions are fulfilled
	app.Spec, err = encodeAppSpec(spec)
	if err != nil {
		return api.TaskObjectResponse{}, err
	}

	task, err := beApp.m.UpdateSpec(app.ID, app.Spec, model.ActionAppResourceEdit, "", nil, nil)
	if err != nil {
		return api.TaskObjectResponse{}, err
	}

	wt := beApp.waits.NewWaitTask(app.ID, time.Second*30, func(err error) error {
		tk := taskUpdate(task, err)

		return beApp.m.UpdateAppTask(nil, tk)
	})

	allowRestore = false
	go wt.WithTimeout(time.Minute*5, func() (bool, error) {

		done := 0
		var errs []error

		for i := range app.Units {
			if _, ok := requestGroups[app.Units[i].GetServiceType()]; ok {

				_, err := beApp.zone.ensureUnitResourceEqual(requestGroups[app.Units[i].GetServiceType()],
					app.Units[i].Site, app.Units[i].Namespace, app.Units[i].ObjectName())

				if err != nil {
					errs = append(errs, err)
				}
			}
			done += 1
		}

		for _, host := range hostsAndResourceRecord {
			klog.Infof("host = %s unschedulable = %t", host.hostname, host.originalUnschedulable)
			_, err := beApp.zone.setHostUnschedulable(host.site, host.hostname, host.originalUnschedulable)
			if err != nil {
				errs = append(errs, err)
			}
			appUpdateRequestsChan <- appUpdateRequestsChanPut
		}

		if done == len(app.Units) {
			return true, nil
		} else {
			return false, utilerrors.NewAggregate(errs)
		}
	})

	return api.TaskObjectResponse{
		ObjectID:   app.ID,
		ObjectName: app.Name,
		TaskID:     task,
	}, nil
}

func mergeAppResourceRequest(name string, origin api.ResourceRequirements, requests api.ResourceRequirementsOptions) (api.ResourceRequirements, error) {

	if requests.Bandwidth != nil {
		if origin.Bandwidth == nil {
			return origin, fmt.Errorf("App %s is forbided update net_bandwidth resource", name)
		}

		if *origin.Bandwidth != *requests.Bandwidth {
			origin.Bandwidth = requests.Bandwidth
		}
	}

	if requests.CPU != nil && *requests.CPU != origin.CPU {
		origin.CPU = *requests.CPU
	}

	if requests.Memory != nil && *requests.Memory != origin.Memory {
		origin.Memory = *requests.Memory
	}

	if requests.Storage == nil {
		return origin, nil
	}

	if origin.Storage == nil {
		return origin, fmt.Errorf("App %s is forbided update storage resource", name)
	}

	if (requests.Storage.Performance != api.PerformanceNone &&
		requests.Storage.Performance != origin.Storage.Performance) ||
		(requests.Storage.Type != "" &&
			requests.Storage.Type != origin.Storage.Type) {

		return origin, fmt.Errorf("App %s is update resource conflicts:storage (%s,%s)!= (%s,%s)",
			name, requests.Storage.Performance, requests.Storage.Type,
			origin.Storage.Performance, origin.Storage.Type)
	}

	for i := range origin.Storage.Volumes {
		for _, v := range requests.Storage.Volumes {

			if origin.Storage.Volumes[i].Type == v.Type {
				if origin.Storage.Volumes[i].Capacity < v.Capacity {
					origin.Storage.Volumes[i].Capacity = v.Capacity
					break
				} else if origin.Storage.Volumes[i].Capacity > v.Capacity {
					return origin, fmt.Errorf("App %s is update resource conflicts:storage shrink detected on type '%s' %d > %d)",
						name, requests.Storage.Type, origin.Storage.Volumes[i].Capacity, v.Capacity)
				}
			}
		}
	}

	return origin, nil
}

func (beApp *bankendApp) UnitRebuild(ctx context.Context, appID, unitID string, opts api.UnitRebuildOptions) (api.TaskObjectResponse, error) {

	app, site, spec, err := beApp.CheckAppModel(appID)
	if err != nil {
		return api.TaskObjectResponse{}, err
	}

	iface, err := beApp.zone.siteInterface(beApp.GetSiteStr())
	if err != nil {
		return api.TaskObjectResponse{}, err
	}
	var unitsMap map[string][]unitv4.Unit = make(map[string][]unitv4.Unit)
	unitsMap[structs.MysqlServiceType] = []unitv4.Unit{}
	unitsMap[structs.CmhaServiceType] = []unitv4.Unit{}
	unitsMap[structs.ProxysqlServiceType] = []unitv4.Unit{}

	var mu *model.Unit
	for i := range app.Units {
		theUnit, err := iface.Units().Get(metav1.NamespaceDefault, app.Units[i].ID)
		if err != nil {
			return api.TaskObjectResponse{}, err
		}
		unitsMap[app.Units[i].GetServiceType()] = append(unitsMap[app.Units[i].GetServiceType()], *theUnit)

		if app.Units[i].ID == unitID {
			mu = &app.Units[i]
		}
	}

	if mu == nil {
		return api.TaskObjectResponse{}, fmt.Errorf("not found unit %s in App %s", unitID, app.ID)
	}

	var (
		image    *model.Image
		requests *api.ResourceRequirements
	)

	if opts.Node != nil && *opts.Node != "" {

		host, err := beApp.hosts.GetHostBrief(*opts.Node)
		if err != nil {
			return api.TaskObjectResponse{}, err
		}
		node := host.ID
		opts.Node = &node
	}

	if opts.Image != nil {
		im, err := beApp.images.Get(*opts.Image)
		if err != nil {
			return api.TaskObjectResponse{}, err
		}
		image = &im

		if !mu.IsServiceType(image.Type) {
			return api.TaskObjectResponse{}, stderror.New("cannot rebuild unit with a different image type")
		}
	}

	if opts.Resources != nil {
		var req api.ResourceRequirements
		err := stderror.New("cannot find related spec")
		switch mu.GetServiceType() {
		case structs.MysqlServiceType:
			{
				if spec.Database != nil {
					req, err = mergeAppResourceRequest(app.ID, spec.Database.Services.Units.Resources.Requests, *opts.Resources)
				}
			}
		case structs.CmhaServiceType:
			{
				if spec.Cmha != nil {
					req, err = mergeAppResourceRequest(app.ID, spec.Cmha.Services.Units.Resources.Requests, *opts.Resources)
				}
			}
		case structs.ProxysqlServiceType:
			{
				if spec.Proxy != nil {
					req, err = mergeAppResourceRequest(app.ID, spec.Proxy.Services.Units.Resources.Requests, *opts.Resources)
				}
			}
		}

		if err != nil {
			return api.TaskObjectResponse{}, err
		}

		requests = &req
	}

	task, err := beApp.m.InsertUnitTask(*mu, model.ActionAppUnitRebuild)
	if err != nil {
		return api.TaskObjectResponse{}, err
	}

	rebuild := false

	wt := beApp.waits.NewWaitTask(app.ID, time.Second*30, func(err error) error {
		tk := taskUpdate(task, err)

		return beApp.m.UpdateAppTask(nil, tk)
	})

	count := 0
	const maxCount = 10*60/30 - 1

	go wt.WithTimeout(time.Minute*10, func() (bool, error) {

		unit, err := iface.Units().Get(mu.Namespace, mu.ObjectName())
		if err != nil {
			return false, err
		}

		if !rebuild {

			clone := unit.DeepCopy()

			if image != nil {
				for i, container := range clone.Spec.Template.Spec.Containers {
					if container.Name != clone.Spec.MainContainerName {
						continue
					}

					clone.Spec.Template.Spec.Containers[i].Name = image.Type
					clone.Spec.Template.Spec.Containers[i].Image = fmt.Sprintf("%s/%s/%s", site.ImageRegistry, site.ProjectName, image.ImageWithArch())
				}

				clone.Spec.MainContainerName = image.Type
			}

			if requests != nil {
				_, _, err := mergeUnitResources(clone, *requests)
				if err != nil {
					return false, err
				}
			}

			clone.Spec.Action.Rebuild = &unitv4.RebuildAction{
				//Storage:  true,
				NodeName: opts.Node,
			}

			unit, err = iface.Units().Update(clone.Namespace, clone)
			if err == nil && unit.Spec.Action.Rebuild != nil {
				rebuild = true
			}

			return false, err
		}

		if unit.Spec.Action.Rebuild != nil {
			return false, nil
		}

		pod, err := iface.Pods().Get(unit.Namespace, unit.PodName())
		if err != nil {
			return false, err
		}

		if podutil.IsRunningAndReady(pod) {
			err = beApp.doLinkInitForUnit(unitsMap, spec, unit.Namespace, unit.Name)
			if err != nil {
				return false, err
			}
			return true, nil
		} else {
			count += 1
			if count > maxCount-1 {
				return false, fmt.Errorf("pod %s/%s is not ready,current phase is %s", pod.Namespace, pod.Name, pod.Status.Phase)
			} else {
				return false, nil
			}

		}

	})

	return api.TaskObjectResponse{
		ObjectID:   app.ID,
		ObjectName: app.Name,
		TaskID:     task,
	}, nil
}

func (beApp *bankendApp) UnitRestore(ctx context.Context, appID, unitID string, opts api.UnitRestoreOptions) (api.TaskObjectResponse, error) {

	app, site, _, err := beApp.CheckAppModel(appID)
	if err != nil {
		return api.TaskObjectResponse{}, err
	}

	var mu *model.Unit
	for i := range app.Units {
		if app.Units[i].ID == unitID {
			mu = &app.Units[i]
			break
		}
	}

	if mu == nil {
		return api.TaskObjectResponse{}, fmt.Errorf("not found unit %s in App %s", unitID, app.ID)
	}

	iface, err := beApp.zone.siteInterface(mu.Site)
	if err != nil {
		return api.TaskObjectResponse{}, err
	}

	file, err := beApp.files.GetFile(opts.File)
	if err != nil {
		return api.TaskObjectResponse{}, err
	}

	task, err := beApp.m.InsertUnitTask(*mu, model.ActionAppUnitRestore)
	if err != nil {
		return api.TaskObjectResponse{}, err
	}

	endpoint, err := beApp.endpoints.GetEndpoint(file.EndpointId)
	if err != nil {
		return api.TaskObjectResponse{}, err
	}

	timestamp := strconv.Itoa(int(time.Now().Unix()))

	jr := &restoreJob{
		mu:       *mu,
		site:     site,
		file:     file,
		endpoint: endpoint,
		app:      app,
		zone:     beApp.zone,
	}

	cmd := []string{
		"sh",
		shell,
		"replication",
		"gtid_purge",
	}

	wt := beApp.waits.NewWaitTask(app.ID, time.Second*30, func(err error) error {
		tk := taskUpdate(task, err)

		return beApp.m.UpdateAppTask(nil, tk)
	})

	timeout := time.Minute*15 + time.Minute*3*time.Duration(file.Size>>10)

	if opts.Timeout != nil {
		timeout = time.Minute*15 + time.Duration(*opts.Timeout)*time.Minute
	}

	go wt.WithTimeout(timeout, func() (bool, error) {

		if jr.job == nil {
			_, err = jr.zone.updateUnitAction(jr.site.ID, mu.Namespace, mu.ObjectName(), api.StateRestoring)
			if err != nil {
				return false, err
			}
		}
		ok, err := jr.Run(iface, file.File, timestamp)
		if err != nil {
			return ok, err
		}
		if !ok {
			return false, err
		}
		ok, _, err = beApp.zone.runInContainer(mu.Site, mu.Namespace, mu.ObjectName(), cmd)
		if err != nil {
			return false, err
		}

		if !ok {
			return false, err
		}
		err = deleteJob(iface, jr.job)
		if err != nil {
			return false, err
		}
		return true, nil
	})

	return api.TaskObjectResponse{
		ObjectID:   app.ID,
		ObjectName: app.Name,
		TaskID:     task,
	}, nil
}

func (beApp *bankendApp) UnitMigrate(ctx context.Context, appID, unitID string, opts api.UnitMigrateOptions) (api.TaskObjectResponse, error) {

	app, _, _, err := beApp.CheckAppModel(appID)
	if err != nil {
		return api.TaskObjectResponse{}, err
	}

	var mu *model.Unit
	for i := range app.Units {
		if app.Units[i].ID == unitID {
			mu = &app.Units[i]
			break
		}
	}

	if mu == nil {
		return api.TaskObjectResponse{}, fmt.Errorf("not found unit %s in App %s", unitID, app.ID)
	}

	tries := 0

	if opts.MaxRetries == nil {
		opts.MaxRetries = &defaultRetries
	}

	if opts.Node != nil && *opts.Node != "" {
		node, err := beApp.hosts.GetHostBrief(*opts.Node)
		if err != nil {
			return api.TaskObjectResponse{}, err
		}

		name := node.ObjectName()
		opts.Node = &name
	}

	task, err := beApp.m.InsertUnitTask(*mu, model.ActionAppUnitRestore)
	if err != nil {
		return api.TaskObjectResponse{}, err
	}

	wt := beApp.waits.NewWaitTask(app.ID, time.Second*30, func(err error) error {
		tk := taskUpdate(task, err)

		return beApp.m.UpdateAppTask(nil, tk)
	})

	go wt.WithTimeout(time.Minute*10, func() (bool, error) {

		iface, err := beApp.zone.siteInterface(mu.Site)
		if err != nil {
			return false, err
		}

		unit, err := iface.Units().Get(mu.Namespace, mu.ObjectName())
		if err != nil {
			return false, err
		}

		pod, err := iface.Pods().Get(unit.Namespace, unit.PodName())
		if err != nil {
			return false, err
		}

		if pod.GetDeletionTimestamp() != nil {
			return false, nil
		}

		if tries > 0 && (opts.Node == nil ||
			*opts.Node == "" ||
			pod.Spec.NodeName == *opts.Node) &&
			podutil.IsRunning(pod) {

			return true, nil
		}

		if tries > 0 && time.Since(pod.GetCreationTimestamp().Time) < time.Minute*3 {
			return false, nil
		}

		if tries >= *opts.MaxRetries {
			return false, fmt.Errorf("unit migrate %d/%d times,times out", tries, *opts.MaxRetries)
		}

		err = iface.Pods().Delete(pod.Namespace, pod.Name, metav1.DeleteOptions{})
		if err != nil {
			return false, err
		}

		unit = unit.DeepCopy()

		if opts.Node != nil {
			unit.Spec.Template.Spec.NodeName = *opts.Node
		}

		unit, err = iface.Units().Update(unit.Namespace, unit)
		if err != nil {
			return false, err
		}

		tries++

		return false, nil
	})

	return api.TaskObjectResponse{
		ObjectID:   app.ID,
		ObjectName: app.Name,
		TaskID:     task,
	}, nil
}

func (beApp *bankendApp) UnitRoleSet(ctx context.Context, appID, unitID string, opts api.RoleSetOptions) error {
	app, _, _, err := beApp.CheckAppModel(appID)
	if err != nil {
		return err
	}

	appSpec, _ := decodeAppSpec(app.Spec)
	archMode := appSpec.Database.Services.Arch.Mode

	var mu model.Unit
	for _, unit := range app.Units {
		if unit.ID == unitID {
			mu = unit
			break
		}
	}

	masterID := opts.MasterID
	var masterUnit model.Unit
	for _, unit := range app.Units {
		if unit.ID == masterID {
			masterUnit = unit
			break
		}
	}

	cmd := []string{
		"sh",
		shell,
		"replication",
	}

	switch opts.Role {
	case api.ServiceRoleSlave:
		if unitID == opts.MasterID {
			return fmt.Errorf("Cannot set master unit: %s to slave.", unitID)
		}

		klog.Info("Set unit as slave...")
		useraddcmd := append(cmd, "user_add")

		unitList, err := beApp.listAppUnits(appID, app.Units, false)
		if err != nil {
			return fmt.Errorf("Error to find units in this app.")
		}

		slaveIP := ""
		for i := range unitList {
			if unitList[i].ID == unitID {
				slaveIP = unitList[i].IP.String()
				break
			}
		}

		var userAdd api.UserAddOptions
		userAdd.IP = append(userAdd.IP, slaveIP)
		userAddDate, err := encodeJson(userAdd)
		if err != nil {
			return err
		}

		userAddCmd := append(useraddcmd, string(userAddDate))
		ok, _, err := beApp.zone.runInContainer(masterUnit.Site, masterUnit.Namespace, masterUnit.ObjectName(), userAddCmd)
		if !ok {
			return fmt.Errorf("Fail to add user.")
		}

		if err != nil {
			return fmt.Errorf("Run scripts [replication user_add] err: %s.", err)
		}

		slavecmd := append(cmd, "reset_slave")

		masterIP := ""
		for i := range unitList {
			if unitList[i].ID == masterID {
				masterIP = unitList[i].IP.String()
				break
			}

		}
		data, err := encodeJson(api.RoleSlaveOptions{archMode, masterIP})
		if err != nil {
			return err
		}

		slaveCmd := append(slavecmd, string(data))
		ok, _, err = beApp.zone.runInContainer(mu.Site, mu.Namespace, mu.ObjectName(), slaveCmd)
		if !ok {
			return fmt.Errorf("Set unit %s as slave fail.", unitID)
		}

		if err != nil {
			return fmt.Errorf("Run scripts [replication reset_slave] in unit: %s err: %s.", unitID, err)
		}

	case api.ServiceRoleMaster:
		klog.Info("Set unit as master...")
		cmd = append(cmd, "reset_master")

		masterData, err := encodeJson(api.RoleMasterOptions{archMode})
		if err != nil {
			return err
		}
		masterCmd := append(cmd, string(masterData))

		ok, _, err := beApp.zone.runInContainer(mu.Site, mu.Namespace, mu.ObjectName(), masterCmd)
		if !ok {
			return fmt.Errorf("Set unit %s as master fail.", unitID)
		}

		if err != nil {
			return fmt.Errorf("Run scripts [replication reset_master] in unit: %s err: %s.", unitID, err)
		}

	default:
		return fmt.Errorf("Unknown role: %s, please check again. ", opts.Role)
	}

	return nil
}

func (beApp *bankendApp) UpdateUnitResourceRequests(ctx context.Context, appID, unitID string, opts api.AppResourcesOptions) (api.TaskObjectResponse, error) {
	app, _, spec, err := beApp.CheckAppModel(appID)
	if err != nil {
		return api.TaskObjectResponse{}, err
	}

	var mu *model.Unit
	for i := range app.Units {
		if app.Units[i].ID == unitID {
			mu = &app.Units[i]
			break
		}
	}

	if mu == nil {
		return api.TaskObjectResponse{}, fmt.Errorf("not found unit %s in App %s", unitID, app.ID)
	}

	var errs []error

	requestGroups := make(map[string]api.ResourceRequirements)

	if mu.IsServiceType(structs.MysqlServiceType) && opts.Spec.Database != nil {
		err := updateResourcesFit2groups(requestGroups, app.Name, structs.MysqlServiceType, spec.Database, opts.Spec.Database)
		if err != nil {
			errs = append(errs, err)
		}
	} else if mu.IsServiceType(structs.ProxysqlServiceType) && opts.Spec.Proxy != nil {
		err := updateResourcesFit2groups(requestGroups, app.Name, structs.ProxysqlServiceType, spec.Proxy, opts.Spec.Proxy)
		if err != nil {
			errs = append(errs, err)
		}
	} else if mu.IsServiceType(structs.CmhaServiceType) && opts.Spec.Cmha != nil {
		err := updateResourcesFit2groups(requestGroups, app.Name, structs.CmhaServiceType, spec.Cmha, opts.Spec.Cmha)
		if err != nil {
			errs = append(errs, err)
		}
	} else {
		return api.TaskObjectResponse{}, fmt.Errorf("unit type is %s but related resources request not found", mu.GetServiceType())
	}

	if len(errs) > 0 {
		return api.TaskObjectResponse{}, utilerrors.NewAggregate(errs)
	}

	// try to get pass
	appUpdateRequestsChan <- appUpdateRequestsChanTake
	passOrNot := <-appUpdateRequestsChan
	if passOrNot == "nok" {
		return api.TaskObjectResponse{}, stderror.Errorf("there is ongoing expansion")
	}

	// pre-check and set hosts to unschedulable = true
	var hostsAndResourceRecord = make(map[string]resourceRecord)
	var allowRestore = true
	defer func() {
		klog.Infof("defer is called with allowRestore = %t", allowRestore)
		if allowRestore {
			for _, host := range hostsAndResourceRecord {
				klog.Infof("host = %s unschedulable = %t", host.hostname, host.originalUnschedulable)
				beApp.zone.setHostUnschedulable(host.site, host.hostname, host.originalUnschedulable)
			}
			appUpdateRequestsChan <- appUpdateRequestsChanPut
		}
	}()

	_, err = beApp.zone.calculateUnitResourceDelta(requestGroups[mu.GetServiceType()],
		mu.Site, mu.Namespace, mu.ObjectName(), hostsAndResourceRecord)

	if err != nil {
		errs = append(errs, err)
	}

	checkResourceErrs := checkResourceSufficiency(hostsAndResourceRecord)
	for _, err := range checkResourceErrs {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return api.TaskObjectResponse{}, utilerrors.NewAggregate(errs)
	}
	//
	//// do update only when hosts conditions are fulfilled
	//app.Spec, err = encodeAppSpec(spec)
	//if err != nil {
	//	return api.TaskObjectResponse{}, err
	//}
	//
	task, err := beApp.m.UpdateSpec(app.ID, app.Spec, model.ActionAppResourceEdit, "", nil, nil)
	if err != nil {
		return api.TaskObjectResponse{}, err
	}

	wt := beApp.waits.NewWaitTask(app.ID, time.Second*30, func(err error) error {
		tk := taskUpdate(task, err)

		return beApp.m.UpdateAppTask(nil, tk)
	})

	allowRestore = false
	go wt.WithTimeout(time.Minute*5, func() (bool, error) {

		var errs []error

		_, err := beApp.zone.ensureUnitResourceEqual(requestGroups[mu.GetServiceType()],
			mu.Site, mu.Namespace, mu.ObjectName())

		if err != nil {
			errs = append(errs, err)
		}

		for _, host := range hostsAndResourceRecord {
			klog.Infof("host = %s unschedulable = %t", host.hostname, host.originalUnschedulable)
			_, err := beApp.zone.setHostUnschedulable(host.site, host.hostname, host.originalUnschedulable)
			if err != nil {
				errs = append(errs, err)
			}
			appUpdateRequestsChan <- appUpdateRequestsChanPut
		}

		if len(errs) == 0 {
			return true, nil
		} else {
			return false, utilerrors.NewAggregate(errs)
		}
	})

	return api.TaskObjectResponse{
		ObjectID:   app.ID,
		ObjectName: app.Name,
		TaskID:     task,
	}, nil
}

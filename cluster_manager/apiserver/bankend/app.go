package bankend

import (
	"context"
	"encoding/json"
	"fmt"
	networkv1 "github.com/upmio/dbscale-kube/pkg/apis/networking/v1alpha1"
	lvmv1 "github.com/upmio/dbscale-kube/pkg/apis/volumepath/v1alpha1"
	"sort"
	"strings"
	"sync"
	"time"

	stderror "github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/upmio/dbscale-kube/pkg/structs"
	"github.com/upmio/dbscale-kube/pkg/vars"

	"github.com/beego/beego/config"

	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/api"
	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/model"
	unitv4 "github.com/upmio/dbscale-kube/pkg/apis/unit/v1alpha4"
	podutil "github.com/upmio/dbscale-kube/pkg/utils/pod"
	"github.com/upmio/dbscale-kube/pkg/zone"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
)

type mapBool struct {
	units       map[string][]unitv4.Unit
	serviceType string
	ok          bool
}

type UnitInfoOrErr struct {
	info api.UnitInfo
	err  error
}

func NewAppBankend(zone zone.ZoneInterface,
	m modelApp,
	images imageGetter,
	sites siteGetter,
	clusters clusterGetter,
	networks networkGetter,
	hosts hostGetter,
	files backupFileGetter,
	endpoints endpointGetter,
	storages storageGetter,
	pools poolGetter) *bankendApp {
	return &bankendApp{
		m:      m,
		images: images,
		zone:   zoneIface{zone: zone},
		waits:  NewWaitTasks(),

		sites:     sites,
		clusters:  clusters,
		networks:  networks,
		hosts:     hosts,
		storages:  storages,
		pools:     pools,
		files:     files,
		endpoints: endpoints,
	}
}

type bankendApp struct {
	m         modelApp
	images    imageGetter
	sites     siteGetter
	clusters  clusterGetter
	networks  networkGetter
	hosts     hostGetter
	storages  storageGetter
	pools     poolGetter
	files     backupFileGetter
	endpoints endpointGetter

	zone zoneIface

	waits *waitTasks
}

type modelApp interface {
	appGetter

	Insert(model.Application) (string, string, error)
	InsertUnits(units []model.Unit) error
	InsertAppTask(app model.Application, action string) (string, error)
	InsertUnitTask(unit model.Unit, action string) (string, error)
	Update(app model.Application, action string) (string, error)
	UpdateSpec(app, spec, action, user string, add, remove []model.Unit) (string, error)
	UpdateStatus(app, newStatus, targetService, user string) error
	UpdateAppTask(app *model.Application, tk model.Task) error
	Delete(name string) error
}

type appGetter interface {
	Get(name string) (model.Application, error)
	List(selector map[string]string) ([]model.Application, error)
	ListWithPagination(selector map[string]string, pagination api.PaginationReq) ([]model.Application, api.PaginationResp, error)
}

func (beApp *bankendApp) GetSite() zone.Site {
	sites := beApp.zone.zone.ListSites()
	return sites[0]
}

func (beApp *bankendApp) GetSiteStr() string {
	sites := beApp.GetSite()
	return sites.Name()
}

func (beApp *bankendApp) CheckAppAndSubscription(_ context.Context, appId, subscriptionId string) error {
	app, err := beApp.m.Get(appId)
	if err != nil && !model.IsNotExist(err) {
		return err
	}

	if strings.TrimSpace(subscriptionId) == "" {
		if model.IsNotExist(err) {
			return stderror.New("app not found")
		}
	} else {
		if model.IsNotExist(err) {
			return stderror.New("app not found or permission denied")
		}
		if app.SubscriptionId != subscriptionId {
			return stderror.New("app not found or permission denied")
		}
	}
	return nil
}

func (beApp *bankendApp) AddApp(ctx context.Context, config api.AppConfig, subscriptionId string) (api.Application, error) {
	//name duplication check
	dupFound, err := beApp.CheckAppExists(ctx, config.Name, subscriptionId)
	if err != nil {
		return api.Application{}, err
	}
	if dupFound {
		return api.Application{}, stderror.New("name is duplicated")
	}

	//validation and do some prepare work
	err = beApp.prepareAppConfig(ctx, &config)
	if err != nil {
		return api.Application{}, err
	}

	err = beApp.preResourceCheck(ctx, config)
	if err != nil {
		return api.Application{}, err
	}

	//create some return values
	app, err := convertToAppModel(config)
	if err != nil {
		return api.Application{}, err
	}

	app.SubscriptionId = subscriptionId
	appId, task, err := beApp.m.Insert(app)
	if err != nil {
		return api.Application{}, err
	}

	//create db, cmha, proxy and wait for ready
	readyTimeout := time.Minute*5 + time.Duration(config.Spec.Database.Services.Num)*time.Minute*2
	if config.Spec.Database.Services.Units.Resources.Requests.Storage != nil && config.Spec.Database.Services.Units.Resources.Requests.Storage.Type == api.StorageTypeRemote {
		readyTimeout *= time.Duration(len(config.Spec.Database.Services.Units.Resources.Requests.Storage.Volumes))
	}

	var allUnits map[string][]unitv4.Unit = make(map[string][]unitv4.Unit)
	var failedGroups map[string][]unitv4.Unit = make(map[string][]unitv4.Unit)

	//orchestration
	orchestration := func() error {
		//firstly replication init
		for i := 0; i < config.Spec.Database.Services.Num; i++ {
			err = beApp.dbReplication(ctx, allUnits[groupName(config.Name, structs.MysqlServiceType, i)], config.Spec.Database.Services.Arch)
			if err != nil {
				return err
			}
		}

		//set units status to StatePassing
		err = beApp.m.UpdateStatus(appId, api.StatePassing.ToString(), structs.MysqlServiceType, config.User)
		if err != nil {
			return err
		}

		//secondly link init
		if config.Spec.Cmha != nil {
			err = beApp.doLinkInit(allUnits, config)
			if err != nil {
				return err
			}

			err = beApp.m.UpdateStatus(appId, api.StatePassing.ToString(), structs.CmhaServiceType, config.User)
			if err != nil {
				return err
			}

			err = beApp.m.UpdateStatus(appId, api.StatePassing.ToString(), structs.ProxysqlServiceType, config.User)
			if err != nil {
				return err
			}
		}

		return nil
	}

	groupType := structs.MysqlServiceType
	if config.Spec.Cmha != nil {
		groupType = structs.CmhaServiceType
	}

	taskCtx, taskCancel := context.WithCancel(context.Background())

	wt := beApp.waits.NewWaitTask(appId, 3*time.Minute, func(err error) error {
		tk := taskUpdate(task, err)
		return beApp.m.UpdateAppTask(nil, tk)
	})
	wt.AddCancel(taskCancel)
	wt.SetId(task)

	go wt.WithTimeoutAndCancel(readyTimeout*3, taskCtx, taskCancel, func() (bool, error) {
		groupNameChan := make(chan mapBool)
		expectedCount := 0
		failedGroupsArg := make(map[string][]unitv4.Unit)
		for k, v := range failedGroups {
			failedGroupsArg[k] = v
		}

		if config.Spec.Proxy != nil {
			_, exists := allUnits[groupName(config.Name, structs.ProxysqlServiceType, 0)]
			if !exists {
				go beApp.deploySpec(taskCtx, groupNameChan, appId, config.Name, config.Arch,
					groupName(config.Name, structs.ProxysqlServiceType, 0), groupType, structs.ProxysqlServiceType, failedGroupsArg,
					*config.Spec.Proxy, readyTimeout)
				expectedCount += 1
			}
		}
		if config.Spec.Cmha != nil {
			_, exists := allUnits[groupName(config.Name, structs.CmhaServiceType, 0)]
			if !exists {
				go beApp.deploySpec(taskCtx, groupNameChan, appId, config.Name, config.Arch,
					groupName(config.Name, structs.CmhaServiceType, 0), groupType, structs.CmhaServiceType, failedGroupsArg,
					*config.Spec.Cmha, readyTimeout)
				expectedCount += 1
			}
		}
		for i := 0; i < config.Spec.Database.Services.Num; i++ {
			_, exists := allUnits[groupName(config.Name, structs.MysqlServiceType, i)]
			if !exists {
				go beApp.deploySpec(taskCtx, groupNameChan, appId, config.Name, config.Arch,
					groupName(config.Name, structs.MysqlServiceType, i), groupType, structs.MysqlServiceType, failedGroupsArg,
					*config.Spec.Database, readyTimeout)
				expectedCount += 1
			}
		}

		failedGroups = make(map[string][]unitv4.Unit)
		receivedNum := 0
		receivedNok := false
		for {
			select {
			case <-taskCtx.Done():
				klog.Errorf("task %s is called", task)
				return false, stderror.New("Cancelled")
			case <-time.After(readyTimeout*3 - time.Minute):
				klog.Errorf("task %s is timed out", task)
				return false, stderror.New("Timeout")
			case receivedMapBool := <-groupNameChan:
				{
					if receivedMapBool.ok {
						for k, v := range receivedMapBool.units {
							allUnits[k] = v
						}

						beApp.m.UpdateStatus(appId, api.StateOrch.ToString(), receivedMapBool.serviceType, config.User)
					} else {
						for k, v := range receivedMapBool.units {
							failedGroups[k] = v
						}
						receivedNok = true

						beApp.m.UpdateStatus(appId, api.StateWarning.ToString(), receivedMapBool.serviceType, config.User)
					}
					receivedNum += 1
					if receivedNum == expectedCount {
						if !receivedNok {
							klog.Info("All units are ready, now start to orchestration")
							err := orchestration()
							if err != nil {
								return false, err
							}

							return true, nil
						} else {
							return false, stderror.New("some units are not ready")
						}
					}
				}
			}
		}
	})

	//return
	return api.Application{
		ID:   appId,
		Name: app.Name,
		Task: api.TaskBrief{ID: task},
	}, nil
}

func (beApp *bankendApp) prepareAppConfig(ctx context.Context, config *api.AppConfig) error {
	_, err := beApp.sites.Get(beApp.GetSiteStr())
	if err != nil {
		return stderror.New("cannot find site")
	}

	if config.Spec.Database == nil {
		return stderror.New("database cannot be null")
	}

	validateGroupSpec := func(name string, spec *api.GroupSpec) error {
		var im model.Image
		var err error
		// check arch is same with image.ID's arch
		if strings.Contains(spec.Image.ID, "-") {
			parts := strings.Split(spec.Image.ID, "-")
			if strings.ToLower(parts[len(parts)-1]) != config.Arch {
				return stderror.New("image's version arch is not equal with arch, please check again ")
			}
		}
		if strings.ToLower(spec.Image.ID) == structs.ImageLatestTag {
			im, err = beApp.images.GetLatest(name, config.Arch)
		} else {
			im, err = beApp.images.Get(spec.Image.ID)
		}
		if err != nil {
			return stderror.Errorf("%s image cannot be found, casused by %s", name, err)
		}

		//image = im
		spec.Image = api.ImageVersion(im.ImageVersion)
		return nil
	}

	//check mysql
	if config.Spec.Database != nil {
		err = validateGroupSpec(structs.MysqlServiceType, config.Spec.Database)
		if err != nil {
			return err
		}
	} else {
		return stderror.Errorf("%s is mandatory in request", structs.MysqlServiceType)
	}
	databaseNum := config.Spec.Database.Services.Num

	//check proxy
	if config.Spec.Proxy != nil {
		err = validateGroupSpec(structs.ProxysqlServiceType, config.Spec.Proxy)
		if err != nil {
			return err
		}
		if databaseNum > 1 && config.Spec.Proxy.Services.Num != 1 {
			return stderror.New("database num must be 1 if proxy or cmha is used")
		}
		config.Spec.Proxy.Services.Num = 1
		if config.Spec.Cmha == nil {
			return stderror.New("cmha and proxy must be used together")
		}
	}

	//check cmha
	if config.Spec.Cmha != nil {
		err = validateGroupSpec(structs.CmhaServiceType, config.Spec.Cmha)
		if err != nil {
			return err
		}
		if databaseNum > 1 && config.Spec.Cmha.Services.Num != 1 {
			return stderror.New("database num must be 1 if proxy or cmha is used")
		}
		config.Spec.Cmha.Services.Num = 1
		if config.Spec.Cmha.Services.Arch.Replicas < 3 {
			return stderror.New("cmha replicas num must be 3 or greater")
		}
		if config.Spec.Proxy == nil {
			return stderror.New("cmha and proxy must be used together")
		}
	}

	return nil
}

//用来记录每个host的资源情况
type resRecord struct {
	hostName            string
	cpuAllocatable      int64
	memAllocatable      int64
	vgMediumAllocatable int64
	vgHighAllocatable   int64
	podAllocatable      int64
	cpuFree             int64
	memFree             int64
	vgMediumFree        int64
	vgHighFree          int64
	podFree             int64
	usedBy              []string
}

//用来记录请求的情况
type resRequest struct {
	serviceType string
	cpu         int64
	mem         int64
	vgMedium    int64
	vgHigh      int64
}

func getResRequestsFromConfig(config api.AppConfig) ([]resRequest, int64, int64, int64, int64) {
	var ret []resRequest
	var cpuTotal int64
	var memTotal int64
	var vgMediumTotal int64
	var vgHighTotal int64

	if config.Spec.Cmha != nil {
		for i := 0; i < config.Spec.Cmha.Services.Arch.Replicas; i++ {
			ret = append(ret, resRequest{
				structs.CmhaServiceType,
				config.Spec.Cmha.Services.Units.Resources.Requests.CPU,
				config.Spec.Cmha.Services.Units.Resources.Requests.Memory,
				mergeStorageValue(config.Spec.Cmha.Services.Units.Resources.Requests.Storage, "medium"),
				mergeStorageValue(config.Spec.Cmha.Services.Units.Resources.Requests.Storage, "high"),
			})
			cpuTotal += config.Spec.Cmha.Services.Units.Resources.Requests.CPU
			memTotal += config.Spec.Cmha.Services.Units.Resources.Requests.Memory
			vgMediumTotal += mergeStorageValue(config.Spec.Cmha.Services.Units.Resources.Requests.Storage, "medium")
			vgHighTotal += mergeStorageValue(config.Spec.Cmha.Services.Units.Resources.Requests.Storage, "high")
		}
	}

	if config.Spec.Proxy != nil {
		for i := 0; i < config.Spec.Proxy.Services.Arch.Replicas; i++ {
			ret = append(ret, resRequest{
				structs.ProxysqlServiceType,
				config.Spec.Proxy.Services.Units.Resources.Requests.CPU,
				config.Spec.Proxy.Services.Units.Resources.Requests.Memory,
				mergeStorageValue(config.Spec.Proxy.Services.Units.Resources.Requests.Storage, "medium"),
				mergeStorageValue(config.Spec.Proxy.Services.Units.Resources.Requests.Storage, "high"),
			})
			cpuTotal += config.Spec.Proxy.Services.Units.Resources.Requests.CPU
			memTotal += config.Spec.Proxy.Services.Units.Resources.Requests.Memory
			vgMediumTotal += mergeStorageValue(config.Spec.Proxy.Services.Units.Resources.Requests.Storage, "medium")
			vgHighTotal += mergeStorageValue(config.Spec.Proxy.Services.Units.Resources.Requests.Storage, "high")
		}
	}

	for i := 0; i < config.Spec.Database.Services.Num; i++ {
		for j := 0; j < config.Spec.Database.Services.Arch.Replicas; j++ {
			ret = append(ret, resRequest{
				structs.MysqlServiceType,
				config.Spec.Database.Services.Units.Resources.Requests.CPU,
				config.Spec.Database.Services.Units.Resources.Requests.Memory,
				mergeStorageValue(config.Spec.Database.Services.Units.Resources.Requests.Storage, "medium"),
				mergeStorageValue(config.Spec.Database.Services.Units.Resources.Requests.Storage, "high"),
			})
			cpuTotal += config.Spec.Database.Services.Units.Resources.Requests.CPU
			memTotal += config.Spec.Database.Services.Units.Resources.Requests.Memory
			vgMediumTotal += mergeStorageValue(config.Spec.Database.Services.Units.Resources.Requests.Storage, "medium")
			vgHighTotal += mergeStorageValue(config.Spec.Database.Services.Units.Resources.Requests.Storage, "high")
		}
	}

	sort.Slice(ret, func(i, j int) bool {
		if ret[i].cpu < ret[j].cpu {
			return true
		}
		if ret[i].cpu > ret[j].cpu {
			return false
		}
		return ret[i].mem < ret[j].mem
	})

	return ret, cpuTotal, memTotal, vgMediumTotal, vgHighTotal
}

func mergeStorageValue(storage *api.StorageRequirement, requestLevel string) int64 {
	if string(storage.Performance) != requestLevel {
		return 0
	}

	var ret int64 = 0
	for _, y := range storage.Volumes {
		ret += y.Capacity
	}
	return ret
}

//roughly check if resource is enough to build the app
func (beApp *bankendApp) preResourceCheck(ctx context.Context, config api.AppConfig) error {
	var (
		resRecordHost []resRecord
		cpuTotal      int64
		memTotal      int64
		vgMediumTotal int64
		vgHighTotal   int64
	)

	hostIface, err := beApp.zone.zone.HostInterface(beApp.GetSiteStr())
	if err != nil {
		return err
	}

	nodeSelector := labels.Set{labelRole: vars.NodeRolenode}
	hosts, err := hostIface.List(metav1.ListOptions{
		LabelSelector: nodeSelector.String(),
	})
	if err != nil {
		return err
	}

	for _, host := range hosts {
		if !host.Status.NodeReady {
			continue
		}

		// lijj32: mark each host with appropriate label
		resourceAllocatableLevel := host.GetResourceAllocatableLevel()
		labelData := map[string]map[string]map[string]string{
			"metadata": {
				"labels": {
					labelResourceAllocatable: resourceAllocatableLevel,
				},
			},
		}
		labelDataBytes, err := json.Marshal(labelData)
		if err != nil {
			return err
		}
		_, err = hostIface.Patch(host.ObjectMeta.Name, types.MergePatchType, labelDataBytes)
		if err != nil {
			return err
		}

		var vgMediumAllocatable int64 = 0
		var vgHighAllocatable int64 = 0
		var vgMediumFree int64 = 0
		var vgHighFree int64 = 0

		for _, allocatableLocalVg := range host.Status.Allocatable.LocalVGs {
			if allocatableLocalVg.Level == "medium" {
				vgMediumAllocatable = allocatableLocalVg.Size.Value() >> 20
			} else if allocatableLocalVg.Level == "high" {
				vgHighAllocatable = allocatableLocalVg.Size.Value() >> 20
			}
		}

		for _, capacityLocalVg := range host.Status.Capacity.LocalVGs {
			if capacityLocalVg.Level == "medium" {
				vgMediumFree = capacityLocalVg.Size.Value() >> 20
			} else if capacityLocalVg.Level == "high" {
				vgHighFree = capacityLocalVg.Size.Value() >> 20
			}
		}

		resRecordHost = append(resRecordHost, resRecord{
			host.Name,
			host.Status.Allocatable.Cpu.MilliValue(),
			host.Status.Allocatable.Memery.Value() >> 20,
			vgMediumAllocatable,
			vgHighAllocatable,
			host.Status.Allocatable.Pods.Value(),
			host.Status.Allocatable.Cpu.MilliValue(),
			host.Status.Allocatable.Memery.Value() >> 20,
			vgMediumFree,
			vgHighFree,
			host.Status.Allocatable.Pods.Value(),
			[]string{},
		})
		cpuTotal += host.Status.Allocatable.Cpu.MilliValue()
		memTotal += host.Status.Allocatable.Memery.Value() >> 20
		vgMediumTotal += vgMediumAllocatable
		vgHighTotal += vgHighAllocatable
	}

	resReqs, cpuReqTotal, memReqTotal, vgMediumReqTotal, vgHighReqTotal := getResRequestsFromConfig(config)

	klog.Info(resRecordHost)

	klog.Info(resReqs)

	if cpuReqTotal > cpuTotal || memReqTotal > memTotal ||
		(vgMediumReqTotal > 0 && vgMediumReqTotal > vgMediumTotal) ||
		(vgHighReqTotal > 0 && vgHighReqTotal > vgHighTotal) {
		//klog.Info(cpuReqTotal > cpuTotal)
		//klog.Info(memReqTotal > memTotal)
		//klog.Info(vgMediumReqTotal > 0 && vgMediumReqTotal > vgMediumTotal)
		//klog.Info(vgHighReqTotal > 0 && vgHighReqTotal > vgHighTotal)
		return stderror.Errorf("cluster does not have enough resource, "+
			"has %d cpu, %d mem, %d medium %d high storage in total, "+
			"but requires %d cpu, %d mem, %d medium %d high storage in total",
			cpuTotal, memTotal, vgMediumTotal, vgHighTotal,
			cpuReqTotal, memReqTotal, vgMediumReqTotal, vgHighReqTotal)
	}

	for _, req := range resReqs {
		sort.Slice(resRecordHost, func(i, j int) bool {
			if resRecordHost[i].cpuFree < resRecordHost[j].cpuFree {
				return true
			}
			if resRecordHost[i].cpuFree > resRecordHost[j].cpuFree {
				return false
			}
			return resRecordHost[i].memFree < resRecordHost[j].memFree
		})

		foundHost := false
		for i := 0; i < len(resRecordHost); i++ {
			if req.cpu <= resRecordHost[i].cpuFree &&
				req.mem <= resRecordHost[i].memFree &&
				resRecordHost[i].podFree > 0 &&
				(req.vgMedium <= 0 || req.vgMedium <= resRecordHost[i].vgMediumFree) &&
				(req.vgHigh <= 0 || req.vgHigh <= resRecordHost[i].vgHighFree) {
				resRecordHost[i].cpuFree -= req.cpu
				resRecordHost[i].memFree -= req.mem
				resRecordHost[i].vgMediumFree -= req.vgMedium
				resRecordHost[i].vgHighFree -= req.vgHigh
				resRecordHost[i].podFree -= 1
				resRecordHost[i].usedBy = append(resRecordHost[i].usedBy, req.serviceType)
				foundHost = true
				break
			}
		}

		if !foundHost {
			klog.Error(resRecordHost)
			return stderror.Errorf("cluster does not have enough resource for %s,"+
				" it requires %d cpu, %d mem, %d medium %d high storage",
				req.serviceType, req.cpu, req.mem, req.vgMedium, req.vgHigh)
		}
	}

	return nil
}

func (beApp *bankendApp) deploySpec(ctx context.Context, resChan chan mapBool,
	appId, appName, arch, groupName, groupType, serviceType string, failedGroups map[string][]unitv4.Unit,
	spec api.GroupSpec, readyTimeout time.Duration) {

	ctrl := NewPlanController(beApp.zone)
	units, ok := failedGroups[groupName]
	returnGroups := make(map[string][]unitv4.Unit)
	replicas := spec.Services.Arch.Replicas
	site, _ := beApp.sites.Get(beApp.GetSiteStr())
	image, _ := beApp.images.Get(spec.Image.ID)

	klog.Infof("deploySpec %s %s %s len(units)=%d replicas=%d ok=%t", appId, groupName, groupType, len(units), replicas, ok)
	deploy := func() error {
		if ok {
			dead, _ := beApp.checkUnitStatusOK(units, readyTimeout, false)
			klog.Info("len(dead) ", len(dead))

			if len(dead) > 0 {
				err := ctrl.deleteUnits(dead)
				if err != nil {
					klog.Info(err)
					ctrl.units = dead
					klog.Error("delete dead unit error:", err)
					return err
				} else {
					// remove dead unit in groups
					// returnGroups[groupName] = removeDeadUnit(units, dead)
					ctrl.units = removeDeadUnit(units, dead)
				}
			}
		}

		tmpl, err := convertGroupSpecToUnit(appId, appName, groupName, groupType, serviceType, spec.Services, image, site.ImageRegistry, site.ProjectName, site.NetworkMode)
		if err != nil {
			return err
		}

		err = beApp.injectSchedulerInfo(&tmpl, arch, replicas, spec.Services.Conditions, image, spec.Services.Units.Resources.Requests.Storage)
		if err != nil {
			return err
		}

		err = ctrl.DeployService(appName, groupName, groupType, replicas, tmpl)
		if err != nil {
			klog.Errorf("deploy %s error:%s", groupName, err)
		}

		if len(ctrl.units) < replicas {
			_err := ctrl.deleteUnits(ctrl.units)
			return fmt.Errorf("%s want %d units,got %d,%v", groupName, replicas, len(ctrl.units), _err)
		}

		returnGroups[groupName] = ctrl.units
		ctrl.units = nil

		return err
	}

	err := deploy()
	if err != nil {
		klog.Info(err)
		resChan <- mapBool{returnGroups, serviceType, false}
		return
	}

	modelUnits, err := beApp.checkSpecUnitsStatusOK(ctx, appId, groupName, replicas, returnGroups, readyTimeout)
	if err != nil {
		klog.Info(err)
		resChan <- mapBool{returnGroups, serviceType, false}
		return
	}

	err = beApp.m.InsertUnits(modelUnits)
	if err != nil {
		klog.Info(err)
	}

	//create monitor
	//(ctx context.Context, spec api.GroupSpec, appId, appName, groupname, grouptype string)
	err = beApp.RegisterMonitor(ctx, spec, appId, appName, groupName, groupType)
	if err != nil {
		klog.Info(err)
	}

	resChan <- mapBool{returnGroups, serviceType, err == nil}
}

func (beApp *bankendApp) checkSpecUnitsStatusOK(ctx context.Context, appId, groupName string,
	num int, groups map[string][]unitv4.Unit, readyTimeout time.Duration) ([]model.Unit, error) {

	units := make([]model.Unit, 0, num)
	startTime := time.Now()
	waitDuration := 60

loop1:
	for {
		if time.Since(startTime) > readyTimeout {
			klog.Infof("Timeout for %s, not all units of %s are ready", appId, groupName)
			return nil, stderror.New("Timeout")
		}

	loop2:
		for key, list := range groups {

			dead, err := beApp.checkUnitStatusOK(list, readyTimeout, true)
			if err != nil {
				break loop2
			}

			list = removeDeadUnit(list, dead)
			groups[key] = list

			for _, unit := range list {
				units = append(units, model.Unit{
					ID:        unit.Name,
					Namespace: unit.Namespace,
					Site:      beApp.GetSiteStr(),
					App:       appId,
					Group:     key,
				})
			}
		}

		if len(units) < num {
			waitDuration -= 5
			if waitDuration <= 10 {
				waitDuration = 10
			}
			timer := time.NewTimer(time.Duration(waitDuration) * time.Second)
			select {
			case <-ctx.Done():
				return nil, stderror.New("cancelled")
			case <-timer.C:
				continue loop1
			}
		}

		klog.Infof("%s all units of %s are ready", appId, groupName)
		return units, nil
	}
}

func (beApp *bankendApp) dbReplication(ctx context.Context, units []unitv4.Unit, arch api.Arch) error {
	body := struct {
		Mode     string  `json:"arch_mode"`
		Replicas int     `json:"arch_replicas"`
		Master   api.IPS `json:"master"`
		Slave    api.IPS `json:"slave"`
	}{
		Mode:     arch.Mode,
		Replicas: arch.Replicas,
	}

	sortUnitsByNameOrdinal(units)

	ip, err := beApp.zone.getUnitIP(units[0])
	if err != nil {
		return err
	}

	body.Master = api.IPS{IP: []string{ip}}
	ips := api.IPS{IP: make([]string, 0, len(units)-1)}

	for i := 1; i < len(units); i++ {
		ip, err := beApp.zone.getUnitIP(units[i])
		if err != nil {
			return err
		}

		ips.IP = append(ips.IP, ip)
	}
	body.Slave = ips

	data, err := encodeJson(body)
	if err != nil {
		return err
	}

	cmd := []string{
		"sh", shell,
		"replication", "init",
		string(data),
	}

	for _, unit := range units {
		ok, _, err := beApp.zone.runInContainer(beApp.GetSiteStr(), unit.Namespace, unit.Name, cmd)
		if ok && err == nil {
			continue
		} else {
			return stderror.Errorf("%s %s", ok, err)
		}
	}

	return nil
}

func (beApp *bankendApp) doLinkInit(unitsMap map[string][]unitv4.Unit, config api.AppConfig) error {
	var linkCmd api.LinkCmdOptions

	cmhaUnits, ok := unitsMap[groupName(config.Name, structs.CmhaServiceType, 0)]
	if ok {
		ips, err := beApp.getUnitsIPs(cmhaUnits)
		if err != nil {
			return err
		}

		linkCmd.Services.Cmha = api.IpsPort{
			Ips:  ips,
			Port: config.Spec.Cmha.Services.Ports[0].Port,
		}
	}

	proxyUnits, ok := unitsMap[groupName(config.Name, structs.ProxysqlServiceType, 0)]
	if ok {
		ips, err := beApp.getUnitsIPs(proxyUnits)
		if err != nil {
			return err
		}

		linkCmd.Services.Proxysql = api.IpsPort{
			Ips:  ips,
			Port: config.Spec.Proxy.Services.Ports[0].Port,
		}
	}

	// if mysql num > 1, then cmha, proxy is not used ==> link init is not used
	mysqlUnits, ok := unitsMap[groupName(config.Name, structs.MysqlServiceType, 0)]
	if ok {
		ips, err := beApp.getUnitsIPs(mysqlUnits)
		if err != nil {
			return err
		}

		linkCmd.Services.Mysql = api.IpsPort{
			Ips:  ips,
			Port: config.Spec.Proxy.Services.Ports[0].Port,
		}
	}

	data, err := encodeJson(linkCmd)
	if err != nil {
		return err
	}

	cmd := []string{
		"sh", shell,
		"link", "init",
		string(data),
	}

	for _, unit := range mysqlUnits {
		ok, _, err := beApp.zone.runInContainer(beApp.GetSiteStr(), unit.Namespace, unit.Name, cmd)
		if ok && err == nil {
			continue
		} else {
			return stderror.Errorf("%s %s", ok, err)
		}
	}

	for _, unit := range cmhaUnits {
		ok, _, err := beApp.zone.runInContainer(beApp.GetSiteStr(), unit.Namespace, unit.Name, cmd)
		if ok && err == nil {
			continue
		} else {
			return stderror.Errorf("%s %s", ok, err)
		}
	}

	return nil
}

func (beApp *bankendApp) doLinkInitForUnit(unitsMap map[string][]unitv4.Unit, config api.AppSpec, unitNS, unitName string) error {
	var linkCmd api.LinkCmdOptions
	var cmhaUsed bool = false

	cmhaUnits, ok := unitsMap[structs.CmhaServiceType]
	if ok {
		cmhaUsed = true

		ips, err := beApp.getUnitsIPs(cmhaUnits)
		if err != nil {
			return err
		}

		linkCmd.Services.Cmha = api.IpsPort{
			Ips:  ips,
			Port: config.Cmha.Services.Ports[0].Port,
		}
	}

	if !cmhaUsed {
		klog.Info("cmha is not used so link init will not be performed")
		return nil
	}

	proxyUnits, ok := unitsMap[structs.ProxysqlServiceType]
	if ok {
		ips, err := beApp.getUnitsIPs(proxyUnits)
		if err != nil {
			return err
		}

		linkCmd.Services.Proxysql = api.IpsPort{
			Ips:  ips,
			Port: config.Proxy.Services.Ports[0].Port,
		}
	}

	// if mysql num > 1, then cmha, proxy is not used ==> link init is not used
	mysqlUnits, ok := unitsMap[structs.MysqlServiceType]
	if ok {
		ips, err := beApp.getUnitsIPs(mysqlUnits)
		if err != nil {
			return err
		}

		linkCmd.Services.Mysql = api.IpsPort{
			Ips:  ips,
			Port: config.Proxy.Services.Ports[0].Port,
		}
	}

	data, err := encodeJson(linkCmd)
	if err != nil {
		return err
	}

	cmd := []string{
		"sh", shell,
		"link", "init",
		string(data),
	}

	ok, _, err = beApp.zone.runInContainer(beApp.GetSiteStr(), unitNS, unitName, cmd)
	if ok && err == nil {
		klog.Info("cmha is used and link init succeeded")
		return nil
	} else {
		return stderror.Errorf("%s %s", ok, err)
	}
}

func decodeAppSpec(spec string) (api.AppSpec, error) {
	out := api.AppSpec{}

	err := json.Unmarshal([]byte(spec), &out)

	return out, err
}

func encodeAppSpec(spec api.AppSpec) (string, error) {
	data, err := json.Marshal(spec)

	return string(data), err
}

func convertToAppModel(config api.AppConfig) (model.Application, error) {

	data, err := encodeAppSpec(config.Spec)

	ret := model.Application{
		Name:           config.Name,
		Desc:           config.Desc,
		Spec:           data,
		StatusCmha:     api.StateNotused.ToString(),
		StatusDatabase: api.StatePreparing.ToString(),
		StatusProxysql: api.StateNotused.ToString(),
		Editor:         newCreateEditor(config.User),
	}

	if config.Spec.Cmha != nil {
		ret.StatusCmha = api.StatePreparing.ToString()
		ret.StatusProxysql = api.StatePreparing.ToString()
	}

	return ret, err
}

func newCreateEditor(user string) model.Editor {
	return model.Editor{
		CreatedUser: user,
		CreatedAt:   time.Now(),
	}
}

func removeDeadUnit(units, dead []unitv4.Unit) []unitv4.Unit {
	out := make([]unitv4.Unit, 0, len(units))

	for i := range units {
		skip := false

		for j := range dead {
			if units[i].Name == dead[j].Name &&
				units[i].Namespace == dead[j].Namespace {

				skip = true
				break
			}
		}

		if !skip {
			out = append(out, units[i])
		}
	}

	return out
}

func (beApp *bankendApp) CheckAppExists(ctx context.Context, name, subscriptionId string) (bool, error) {
	selector := make(map[string]string)

	if name != "" {
		selector["name"] = name
	}

	selector["subscription_id"] = subscriptionId

	list, err := beApp.m.List(selector)
	if err != nil {
		return false, err
	}

	return len(list) > 0, nil
}

func (beApp *bankendApp) ListApps(ctx context.Context, id, name, subscriptionId string, detail bool) (api.AppsResponse, error) {
	selector := make(map[string]string)

	if id != "" {
		selector["id"] = id
	}

	if name != "" {
		selector["name"] = name
	}

	if subscriptionId != "" {
		selector["subscription_id"] = subscriptionId
	}

	list, err := beApp.m.List(selector)
	if err != nil {
		return nil, err
	}

	ch := make(chan api.Application, len(list)/2+1)
	out := make([]api.Application, 0, len(list))

	for i := range list {

		go func(b *bankendApp, ma model.Application, ch chan api.Application, detail bool) {

			defer func(start time.Time) {
				klog.Infof("convertToAppAPI: name=%s detail=%t time=%s", ma.Name, detail, time.Since(start))
			}(time.Now())

			units, err := b.listAppUnits(ma.ID, ma.Units, detail)
			if err != nil {
				klog.Errorf("App %s listAppUnits,%s", ma.Name, err)
			}

			ch <- convertToAppAPI(ma, units)
		}(beApp, list[i], ch, detail)
	}

	for i, n := 0, len(list); i < n; i++ {

		out = append(out, <-ch)
	}

	close(ch)

	return out, nil
}

func (beApp *bankendApp) ListAppsWithPagination(ctx context.Context, id, name, subscriptionId string, detail bool, pagination api.PaginationReq) (api.PaginationResp, error) {
	selector := make(map[string]string)

	if id != "" {
		selector["id"] = id
	}

	if name != "" {
		selector["name"] = name
	}

	if subscriptionId != "" {
		selector["subscription_id"] = subscriptionId
	}

	list, paginationRes, err := beApp.m.ListWithPagination(selector, pagination)
	if err != nil {
		return paginationRes, err
	}

	ch := make(chan api.Application, len(list)/2+1)
	out := make([]api.Application, 0, len(list))

	for i := range list {

		go func(b *bankendApp, ma model.Application, ch chan api.Application, detail bool) {

			defer func(start time.Time) {
				klog.Info("convertToAppAPI:", ma.Name, detail, time.Since(start))
			}(time.Now())

			units, err := b.listAppUnits(ma.ID, ma.Units, detail)
			if err != nil {
				klog.Errorf("App %s listAppUnits,%s", ma.Name, err)
			}

			ch <- convertToAppAPI(ma, units)
		}(beApp, list[i], ch, detail)
	}

	for i, n := 0, len(list); i < n; i++ {

		out = append(out, <-ch)
	}

	close(ch)

	paginationRes.Data = out

	return paginationRes, nil
}

func convertToAppAPI(ma model.Application, units []api.UnitInfo) api.Application {
	spec := api.AppSpec{}
	json.Unmarshal([]byte(ma.Spec), &spec)

	app := api.Application{
		ID:             ma.ID,
		SubscriptionId: ma.SubscriptionId,
		Name:           ma.Name,
		Desc:           ma.Desc,
		Spec:           spec,
		Task:           convertToTaskBrief(ma.Task),
		Created:        api.NewEditor(ma.CreatedUser, ma.CreatedAt),
		Modified:       api.NewEditor(ma.ModifiedUser, ma.ModifiedAt),
	}

	databaseGroups := make(map[string][]api.UnitInfo)
	cmhaGroups := make(map[string][]api.UnitInfo)
	proxyGroups := make(map[string][]api.UnitInfo)

	add2group := func(units []api.UnitInfo, key string, targetGroups map[string][]api.UnitInfo) {
		for i := range units {

			//valid name the expected word must be in pos[1]
			//www-cmha00-5de-0     1/1     Running   0          8m10s
			//www-cmha00-72b-2     1/1     Running   0          7m53s
			parts := strings.Split(units[i].ID, "-")

			if strings.HasPrefix(parts[1], key) {
				key = parts[1]

				if exists, ok := targetGroups[key]; !ok {
					targetGroups[key] = []api.UnitInfo{units[i]}
				} else {
					exists = append(exists, units[i])
					targetGroups[key] = exists
				}

				continue
			}
		}
	}

	convert2GroupStatus := func(groups map[string][]api.UnitInfo, groupSpec *api.GroupSpec) []api.GroupStatus {
		if groupSpec == nil {
			return nil
		}

		groupStatus := make([]api.GroupStatus, groupSpec.Services.Num)

		for i := range groupStatus {

			groupStatus[i] = api.GroupStatus{
				Arch:  groupSpec.Services.Arch,
				Ports: groupSpec.Services.Ports,
				Units: []api.UnitInfo{},
			}

			for key, value := range groups {
				if strings.HasSuffix(key, fmt.Sprintf(GroupNamePattern, i)) {
					groupStatus[i].Units = value
					groupStatus[i].Name = key
				}
			}
		}

		return groupStatus
	}

	add2group(units, structs.MysqlServiceType, databaseGroups)
	add2group(units, structs.CmhaServiceType, cmhaGroups)
	add2group(units, structs.ProxysqlServiceTypeSN, proxyGroups)

	state := api.StatePassing
	if len(databaseGroups) == 0 {
		state = api.StateCritical
	}

	databaseServices := convert2GroupStatus(databaseGroups, app.Spec.Database)
	cmhaServices := convert2GroupStatus(cmhaGroups, app.Spec.Cmha)
	proxyServices := convert2GroupStatus(proxyGroups, app.Spec.Proxy)

	app.Status.Databases.Services = databaseServices
	if cmhaServices != nil {
		app.Status.Cmha.Services = cmhaServices
		app.Status.Proxy.Services = proxyServices
	}

	var statetemp []string
	var statetempCmha []string
	var statetempProxy []string
	var statetempDatabase []string

	for _, service := range app.Status.Cmha.Services {
		for i := range service.Units {
			statetempCmha = append(statetempCmha, string(service.Units[i].ReadinessState))
			statetemp = append(statetemp, string(service.Units[i].ReadinessState))
		}
	}

	for _, service := range app.Status.Proxy.Services {
		for i := range service.Units {
			statetempProxy = append(statetempProxy, string(service.Units[i].ReadinessState))
			statetemp = append(statetemp, string(service.Units[i].ReadinessState))
		}
	}

	for _, service := range app.Status.Databases.Services {
		for i := range service.Units {
			statetempDatabase = append(statetempDatabase, string(service.Units[i].ReadinessState))
			statetemp = append(statetemp, string(service.Units[i].ReadinessState))
		}
	}

	mergeState := func(states []string) api.State {
		stateTemp := strings.Join(states, " ")
		if strings.Contains(stateTemp, string(api.StatePassing)) && strings.Contains(stateTemp, string(api.StateCritical)) {
			return api.StateWarning
		} else if strings.Contains(stateTemp, string(api.StatePassing)) && !strings.Contains(stateTemp, string(api.StateCritical)) {
			return api.StatePassing
		} else if !strings.Contains(stateTemp, string(api.StatePassing)) && strings.Contains(stateTemp, string(api.StateCritical)) {
			return api.StateCritical
		} else {
			return api.StateUnknown
		}
	}

	if statetemp != nil && len(statetemp) != 0 {
		state = mergeState(statetemp)
	} else {
		state = api.StateUnknown
	}

	if app.Spec.Proxy != nil {
		app.Spec.Proxy.Services.Units.ReadinessState = mergeState(statetempProxy)
	}
	if app.Spec.Cmha != nil {
		app.Spec.Cmha.Services.Units.ReadinessState = mergeState(statetempCmha)
	}
	app.Spec.Database.Services.Units.ReadinessState = mergeState(statetempDatabase)

	app.State = state

	return app
}

func (beApp *bankendApp) syncAppUnitsByType(id string, units []model.Unit, serviceType string, returnMissing bool) ([]unitv4.Unit, error) {
	var errs []error
	var out []unitv4.Unit

	set := labels.Set{labelAppID: id}
	if serviceType != structs.AllServiceType {
		set[labelServiceType] = serviceType
	}

	for _, site := range beApp.zone.listSites() {

		iface, err := site.SiteInterface()
		if err != nil {
			errs = append(errs, err)
			continue
		}

		list, err := iface.Units().List(metav1.NamespaceDefault,
			metav1.ListOptions{
				LabelSelector: set.String(),
			})
		if err != nil {
			errs = append(errs, err)
			continue
		}

		if len(units) == 0 {
			out = append(out, list...)
			continue
		}

		for i := range list {
			for _, unit := range units {

				if list[i].Name == unit.ObjectName() &&
					list[i].Namespace == unit.Namespace {
					out = append(out, list[i])
					break
				}
			}
		}
	}

	if !returnMissing {
		return out, utilerrors.NewAggregate(errs)
	}

	for i := range units {
		exist := false

		for k := range out {
			if units[i].ObjectName() == out[k].Name &&
				units[i].Namespace == out[k].Namespace {

				exist = true
				break
			}
		}

		if exist {
			continue
		}

		unit, err := beApp.zone.getUnit(units[i].Site, units[i].Namespace, units[i].ObjectName())
		if err != nil || unit == nil {
			errs = append(errs, err)
			continue
		}

		if units[i].IsServiceType(serviceType) {
			out = append(out, *unit)
		}
	}

	return out, utilerrors.NewAggregate(errs)
}

func (beApp *bankendApp) syncAppUnits(id string, units []model.Unit) ([]unitv4.Unit, error) {
	return beApp.syncAppUnitsByType(id, units, structs.AllServiceType, true)
}

func (beApp *bankendApp) listAppUnitsByType(id string, units []model.Unit, serviceType string, detail bool) ([]api.UnitInfo, error) {

	var errs []error

	list, err := beApp.syncAppUnitsByType(id, units, serviceType, false)
	if err != nil {
		errs = append(errs, err)
	}

	out := make([]api.UnitInfo, 0, len(list))
	resChan := make(chan UnitInfoOrErr, len(list))
	var wg sync.WaitGroup

	iface, err := beApp.zone.siteInterface(beApp.GetSiteStr())
	if err != nil {
		errs = append(errs, err)
	}

	allVps, err := iface.VolumePaths().List(metav1.ListOptions{})
	if err != nil {
		errs = append(errs, err)
	}

	allNcs, err := iface.NetworkClaims().List(metav1.ListOptions{})
	if err != nil {
		errs = append(errs, err)
	}

	allPods, err := iface.Pods().List(metav1.NamespaceDefault, metav1.ListOptions{})
	if err != nil {
		errs = append(errs, err)
	}

	for i := range list {
		wg.Add(1)

		go beApp.convertToUnitInfo(list[i], allVps, allNcs, allPods, detail, resChan, &wg)
	}

	wg.Wait()
	for _ = range list {
		res := <-resChan
		if res.err != nil {
			errs = append(errs, err)
		}
		out = append(out, res.info)
	}

loop:
	for i := range units {
		if !units[i].IsServiceType(serviceType) {
			continue
		}

		for j := range out {
			if units[i].ID == out[j].ID {

				if units[i].Task.ID != "" {
					out[j].Task = convertToTaskBrief(units[i].Task)
				}

				continue loop
			}
		}

		// not found unit
		info := api.UnitInfo{
			ID:             units[i].ID,
			Namespace:      units[i].Namespace,
			ReadinessState: api.StateUnknown,
			PodState:       string(api.StateUnknown),
		}

		if units[i].Task.ID != "" {
			info.Task = convertToTaskBrief(units[i].Task)
		}

		out = append(out, info)
	}

	err = utilerrors.NewAggregate(errs)
	if err != nil {
		klog.Error(err)
	}

	return out, nil
}

func (beApp *bankendApp) listAppUnits(id string, units []model.Unit, detail bool) ([]api.UnitInfo, error) {
	return beApp.listAppUnitsByType(id, units, structs.AllServiceType, detail)
}

func (beApp *bankendApp) convertToUnitInfo(unit unitv4.Unit,
	allVps []lvmv1.VolumePath, allNcs []networkv1.NetworkClaim, allPods []v1.Pod,
	detail bool, resChan chan UnitInfoOrErr, wg *sync.WaitGroup) {
	defer wg.Done()

	var errs []error
	info := api.UnitInfo{
		ID:        unit.Name,
		Namespace: unit.Namespace,
	}

	iface, err := beApp.zone.siteInterface(beApp.GetSiteStr())
	if err != nil {
		resChan <- UnitInfoOrErr{info, err}
		klog.Info(err.Error())
		return
	}

	foundNc := false
	ncName := unitv4.GetNetworkClaimName(&unit)
	if ncName != "" {
		for _, oneNc := range allNcs {
			if oneNc.Name == ncName {
				info.IP = api.IP(oneNc.Status.BindIP)
				foundNc = true
				break
			}
		}
	}
	if !foundNc {
		errs = append(errs, stderror.New("cannot find IP"))
	}

	foundPod := false
	var pod v1.Pod
	for _, onePod := range allPods {
		if onePod.Name == unit.PodName() {
			pod = onePod
			foundPod = true
			break
		}
	}

	if !foundPod {
		err = stderror.New("cannot find pod")
		errs = append(errs, err)

		resChan <- UnitInfoOrErr{info, utilerrors.NewAggregate(errs)}
		klog.Info(err.Error())
		return
	}

	var dataVP *lvmv1.VolumePath
	var logVP *lvmv1.VolumePath

	for _, oneVp := range allVps {
		if oneVp.Name == unit.GetName()+"-data" {
			dataVP = &oneVp
			continue
		}
		if oneVp.Name == unit.GetName()+"-log" {
			logVP = &oneVp
			continue
		}
	}

	if dataVP == nil || logVP == nil {
		errs = append(errs, stderror.Errorf("data or log vp not found for %s ", unit.Name))
		resChan <- UnitInfoOrErr{info, utilerrors.NewAggregate(errs)}
		return
	}
	sr := api.StorageRequirement{
		Type:        dataVP.Spec.Type,
		Performance: api.Performance(dataVP.Labels["upm.io.storage.performance"]),
	}

	var vr []api.VolumeRequirement
	dvr := api.VolumeRequirement{
		Capacity: dataVP.Status.CurSize.Value() >> 20,
		Type:     "data",
	}
	vr = append(vr, dvr)
	lvr := api.VolumeRequirement{
		Capacity: logVP.Status.CurSize.Value() >> 20,
		Type:     "log",
	}
	vr = append(vr, lvr)

	sr.Volumes = vr
	if info.IP == "" {
		info.IP = api.IP(pod.Status.PodIP)
	}

	info.PodState = string(pod.Status.Phase)
	info.CreatedAt = api.Time(pod.GetCreationTimestamp().Time)

	info.ReadinessState = api.StateCritical
	if podutil.IsRunningAndReady(&pod) {
		info.ReadinessState = api.StatePassing
	}
	rs := api.ResourceRequirements{}
	image := ""

	for _, container := range pod.Spec.Containers {
		if container.Name == unit.Spec.MainContainerName {
			image = container.Image
		}

		q := container.Resources.Requests.Cpu()
		rs.CPU += q.MilliValue()
		rs.Memory = container.Resources.Requests.Memory().Value()
	}

	rs.Memory = rs.Memory >> 20 // MB
	info.Resources = rs
	if image != "" {
		info.Image, err = api.ParseImageVersion(image)
		if err != nil {
			info.Image.Type = image
			//info.Image.ID = info.Image.String()
			errs = append(errs, err)
		}
	}
	host, err := beApp.hosts.GetHostBrief(pod.Spec.NodeName)
	if err != nil {
		errs = append(errs, err)
	} else {
		cluster, _ := beApp.clusters.Get(host.ClusterID)
		site, _ := beApp.sites.Get(cluster.SiteID)

		info.Node = api.NodeBrief{
			ID:      host.ID,
			Name:    host.HostName,
			IP:      host.IP,
			Cluster: api.NewIDName(host.ClusterID, host.Cluster.Name),
			Site:    api.NewIDName(cluster.SiteID, site.Domain),
		}
	}
	if detail {
		resp, err := getUnitReplication(iface.PodExec(), unit)
		if err == nil {
			info.Replication = &resp
		} else {
			errs = append(errs, err)
		}
	}
	resChan <- UnitInfoOrErr{info, utilerrors.NewAggregate(errs)}
}

func (beApp *bankendApp) DeleteApp(ctx context.Context, id string) (api.TaskObjectResponse, error) {
	app, err := beApp.m.Get(id)
	if model.IsNotExist(err) {
		return api.TaskObjectResponse{}, stderror.Errorf("app %s not found", id)
	}
	if err != nil {
		return api.TaskObjectResponse{}, err
	}

	beApp.waits.Delete(id)

	task, err := beApp.m.InsertAppTask(app, model.ActionAppDelete)
	if err != nil {
		return api.TaskObjectResponse{}, err
	}

	wt := NewWaitTaskWithId(id+"-deletion", time.Minute, func(err error) error {
		if err == nil {
			err = beApp.m.Delete(app.ID)
		}

		tk := taskUpdate(task, err)
		return beApp.m.UpdateAppTask(nil, tk)
	})

	go wt.WithTimeout(time.Minute*10, func() (bool, error) {

		var errs []error
		expectedCount := 0
		receivedCount := 0
		doneChan := make(chan error)

		for _, site := range beApp.zone.listSites() {

			iface, err := site.SiteInterface()
			if err != nil {
				errs = append(errs, err)
				continue
			}

			units, err := iface.Units().List(metav1.NamespaceDefault,
				metav1.ListOptions{
					LabelSelector: labels.Set{labelAppID: id}.String(),
				})
			if errors.IsNotFound(err) {
				continue
			}
			if err != nil {
				errs = append(errs, err)
				continue
			}

			expectedCount += len(units)

			_deleteUnit := func(unit unitv4.Unit) {
				err := deleteUnit(iface.Units(), unit)
				if errors.IsNotFound(err) {
					doneChan <- nil
					return
				}
				doneChan <- err
			}

			for _, unit := range units {
				go _deleteUnit(unit)
			}
		}

		//in case all units have been deleted, not possible to go to select statement, so we end here
		if expectedCount < 1 {
			return true, nil
		}

		for {
			select {
			case err := <-doneChan:
				{
					receivedCount += 1
					//TODO REMOVE this log
					klog.Info(receivedCount)
					errs = append(errs, err)
					if receivedCount == expectedCount {
						err = utilerrors.NewAggregate(errs)
						return err == nil, err
					}
				}
			case <-time.After(time.Minute*10 - time.Minute):
				{
					klog.Info("Time out")
					return false, stderror.New("Timeout ..")
				}
			}
		}
	})

	//delete service monitor
	err = beApp.UnregisterMonitor(ctx, app.Name)
	if err != nil {
		return api.TaskObjectResponse{}, stderror.New("please remove monitor manually")
	}

	return api.TaskObjectResponse{
		ObjectID:   app.ID,
		ObjectName: app.Name,
		TaskID:     task,
	}, nil
}

func (beApp *bankendApp) ListConfig(ctx context.Context, appID string) (api.ConfigMapResponse, error) {
	cm := api.ConfigMapResponse{}
	app, err := beApp.m.Get(appID)
	if err != nil {
		return api.ConfigMapResponse{}, err
	}

	unitList, err := beApp.syncAppUnitsByType(appID, app.Units, structs.MysqlServiceType, true)
	if err != nil {
		return api.ConfigMapResponse{}, fmt.Errorf("Get unit in this service err: %s", err)
	}

	if unitList == nil || len(unitList) == 0 {
		return api.ConfigMapResponse{}, fmt.Errorf("There is no unit in this service.")
	}

	unit := unitList[0]
	ud := unit.DeepCopy()

	appSpec, err := decodeAppSpec(app.Spec)
	if err != nil {
		return api.ConfigMapResponse{}, err
	}

	image, err := beApp.images.Get(appSpec.Database.Image.ID)
	if err != nil {
		return api.ConfigMapResponse{}, err
	}

	ks, err := image.ConvertToKeySets()
	if err != nil {
		return api.ConfigMapResponse{}, err
	}

	iface, err := beApp.zone.siteInterface(beApp.GetSiteStr())
	if err != nil {
		return api.ConfigMapResponse{}, err
	}

	configmap, err := iface.ConfigMaps().Get(unit.Namespace, unitv4.GetUnitConfigName(ud))
	if err != nil {
		return api.ConfigMapResponse{}, err
	}

	content, ok := configmap.Data[unitv4.ConfigDataTab]
	if !ok {
		return api.ConfigMapResponse{}, fmt.Errorf("not find config data")
	}

	configer, err := config.NewConfigData("ini", []byte(content))
	if err != nil {
		return api.ConfigMapResponse{}, err
	}

	for _, keyset := range ks {
		value, exist := beegoConfigString(configer, keyset.Key)
		if !exist {
			continue
		}
		keyset.Value = value
		cm = append(cm, keyset)
	}

	return cm, nil
}

func (beApp *bankendApp) UpdateConfig(ctx context.Context, appID string, opts api.ConfigMapOptions) error {
	app, err := beApp.m.Get(appID)
	if err != nil {
		return err
	}

	unitlist, err := beApp.syncAppUnitsByType(appID, app.Units, structs.MysqlServiceType, true)
	if err != nil {
		return fmt.Errorf("Get unit in this service err: %s", err)
	}

	if unitlist == nil || len(unitlist) == 0 {
		return fmt.Errorf("There is no unit in this service.")
	}

	iface, err := beApp.zone.siteInterface(beApp.GetSiteStr())
	if err != nil {
		return err
	}

	//cmd := []string{
	//	"sh",
	//	shell,o
	//	"config",
	//	"effect",
	//}

	cmd, err := structs.GetExecCmd(structs.CommonType, structs.ConfigEffectCmd)
	if err != nil {
		return nil
	}

	for i := range unitlist {
		unit := unitlist[i]
		ud := unit.DeepCopy()
		//if unit.Status.ConfigMapRef == nil {
		//	return fmt.Errorf("not find related configmap")
		//}

		configmap, err := iface.ConfigMaps().Get(unit.Namespace, unitv4.GetUnitConfigName(ud))
		if err != nil {
			return err
		}

		content, ok := configmap.Data[unitv4.ConfigDataTab]
		if !ok {
			return fmt.Errorf("not find config data")
		}

		configer, err := config.NewConfigData("ini", []byte(content))
		if err != nil {
			return err
		}

		err = configer.Set(opts.Key, opts.Value)
		if err != nil {
			return fmt.Errorf("update key: %s err: %s", opts.Key, err)
		}

		temp, err := marshal(configer)
		if err != nil {
			return fmt.Errorf("marshal configer err: %s", err)
		}

		configmap.Data[unitv4.ConfigDataTab] = string(temp)

		_, err = iface.ConfigMaps().Update(unit.Namespace, configmap)
		if err != nil {
			return fmt.Errorf("iface.ConfigMaps().Update Err: %s", err)
		}

		data, err := encodeJson(api.ConfigMapOptions{opts.Key, opts.Value})
		if err != nil {
			return fmt.Errorf("Encode script json data err: %s", err)
		}

		cmd = append(cmd, string(data))

		ok, _, err = beApp.zone.runInContainer(beApp.GetSiteStr(), unit.Namespace, unit.Name, cmd)
		if !ok {
			return fmt.Errorf("Fail to effect config in unit: %s", unit.Name)
		}

		if err != nil {
			return fmt.Errorf("Run script in unit: %s err: %s", unit.Name, err)
		}
	}

	return nil
}

func (beApp *bankendApp) listAppK8sUnits(appId, serviceType string) ([]unitv4.Unit, error) {
	iface, err := beApp.zone.siteInterface(beApp.GetSiteStr())
	if err != nil {
		return nil, err
	}

	ops := metav1.ListOptions{}
	set := labels.Set{}
	if appId != "" {
		set[labelAppID] = appId
	}

	if serviceType != structs.AllServiceType {
		set[labelServiceType] = serviceType
	}

	if len(set) > 0 {
		ops.LabelSelector = set.String()
	}

	list, err := iface.Units().List(metav1.NamespaceDefault, ops)
	if err != nil && errors.IsNotFound(err) {
		return []unitv4.Unit{}, nil
	}

	return list, err
}

func (beApp *bankendApp) getMasterK8sMysqlUnit(appId string) (unitv4.Unit, bool, error) {
	var kUnit unitv4.Unit
	find := false

	ks8Units, err := beApp.listAppK8sUnits(appId, structs.MysqlServiceType)
	if err != nil {
		return kUnit, find, err
	}

	iface, err := beApp.zone.siteInterface(beApp.GetSiteStr())
	if err != nil {
		return kUnit, find, err
	}
	masterCount := 0

	for _, unit := range ks8Units {
		resp, err := getUnitReplication(iface.PodExec(), unit)
		if err != nil {
			return kUnit, find, err
		}
		if resp.Role == structs.MasterRole {
			kUnit = unit
			masterCount += 1
		}
	}

	if masterCount == 1 {
		return kUnit, true, nil
	} else {
		return kUnit, true, stderror.New("found more than one master pods")
	}
}

func (beApp *bankendApp) getUnitsIPs(units []unitv4.Unit) ([]string, error) {
	var ips []string

	for _, unit := range units {
		ip, err := beApp.zone.getUnitIP(unit)
		if err != nil {
			return ips, nil
		}
		ips = append(ips, ip)
	}

	return ips, nil
}

func (beApp *bankendApp) GetCmhaTopology(ctx context.Context, appId string) (api.CmhaTopology, error) {
	topologyInfo := api.CmhaTopology{}

	k8sUnits, err := beApp.listAppK8sUnits(appId, structs.CmhaServiceType)
	if err != nil {
		return topologyInfo, err
	}

	if len(k8sUnits) == 0 {
		return topologyInfo, stderror.New("cmha is not used in this deployment")
	}

	var kUnit unitv4.Unit
	find := false
	for _, unit := range k8sUnits {
		p, err := beApp.zone.getPod(beApp.GetSiteStr(), unit.Namespace, unit.Name)
		if err != nil {
			continue
		}

		ok := podutil.IsContainerReady(p, unit.Spec.MainContainerName)
		if ok {
			find = true
			kUnit = unit
			break
		}
	}

	if !find {
		return topologyInfo, fmt.Errorf("not find ready pod of application:[%s]", appId)
	}

	cmd, err := structs.GetExecCmd(structs.CmhaServiceType, structs.TopologyShowCmd)
	if err != nil {
		return topologyInfo, err
	}
	cmd = append(cmd, fmt.Sprintf(`{"service_name":"%s"}`, appId))
	klog.Infof("GetCmhaTopology cmd: %v", cmd)

	ok, r, err := beApp.zone.runInContainer(beApp.GetSiteStr(), kUnit.Namespace, kUnit.GetName(), cmd)
	if err != nil {
		return topologyInfo, err
	}

	if ok && err == nil {

		err = decodeJson(r, &topologyInfo)
		if err != nil {
			return topologyInfo, err
		}
	}

	return topologyInfo, nil
}

func (beApp *bankendApp) SetCmhaReplMode(ctx context.Context, appId string, opts api.CmhaReplModeOptions) error {
	iface, err := beApp.zone.siteInterface(beApp.GetSiteStr())
	if err != nil {
		return err
	}

	p, err := iface.Pods().Get(v1.NamespaceDefault, opts.UnitID)
	if err != nil {
		return err
	}
	execOpts := replModeSetOpts{}
	//execOpts.ArchMode = opts.Mode
	execOpts.IP = p.Status.PodIP

	u, err := iface.Units().Get(v1.NamespaceDefault, opts.UnitID)
	if err != nil {
		return err
	}
	execOpts.Port = int(u.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort)

	k8sUnits, err := beApp.listAppK8sUnits(appId, structs.CmhaServiceType)
	if err != nil {
		return err
	}

	if len(k8sUnits) == 0 {
		return stderror.New("cmha is not used in this deployment")
	}

	var execUnit unitv4.Unit
	find := false
	for _, unit := range k8sUnits {
		p, err := beApp.zone.getPod(beApp.GetSiteStr(), unit.Namespace, unit.PodName())
		if err != nil {
			continue
		}

		ok := podutil.IsContainerReady(p, unit.Spec.MainContainerName)
		if ok {
			find = true
			execUnit = unit
			break
		}
	}

	if !find {
		return fmt.Errorf("not find ready pod of service:[%s]", appId)
	}

	data, err := encodeJson(execOpts)
	if err != nil {
		return err
	}

	cmd, err := structs.GetExecCmd(structs.CmhaServiceType, structs.ReplModeSetCmd)
	if err != nil {
		return err
	}
	cmd = append(cmd, string(data))
	klog.Infof("cmd:[%v]", cmd)

	ok, _, err := beApp.zone.runInContainer(beApp.GetSiteStr(), execUnit.Namespace, execUnit.GetName(), cmd)
	if err != nil {
		return err
	}

	if !ok {
		return fmt.Errorf("fail to set cmha replication mode")
	}

	return nil
}

type replModeSetOpts struct {
	//ArchMode string `json:"arch_mode"`
	IP   string `json:"ip"`
	Port int    `json:"port"`
}

func (beApp *bankendApp) SetCmhaMaintenance(ctx context.Context, appId string, opts api.CmhaMaintenanceOptions) error {
	iface, err := beApp.zone.siteInterface(beApp.GetSiteStr())
	if err != nil {
		return err
	}

	p, err := iface.Pods().Get(v1.NamespaceDefault, opts.UnitID)
	if err != nil {
		return err
	}
	execOpts := maintenanceSetOpts{}
	execOpts.Maintenance = *opts.Maintenance
	execOpts.IP = p.Status.PodIP

	u, err := iface.Units().Get(v1.NamespaceDefault, opts.UnitID)
	if err != nil {
		return err
	}
	execOpts.Port = int(u.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort)

	k8sUnits, err := beApp.listAppK8sUnits(appId, structs.CmhaServiceType)
	if err != nil {
		return err
	}

	if len(k8sUnits) == 0 {
		return stderror.New("cmha is not used in this deployment")
	}

	for _, unit := range k8sUnits {
		klog.Info(unit.PodName())
	}

	var execUnit unitv4.Unit
	find := false
	for _, unit := range k8sUnits {
		p, err := beApp.zone.getPod(beApp.GetSiteStr(), unit.Namespace, unit.PodName())
		if err != nil {
			continue
		}

		ok := podutil.IsContainerReady(p, unit.Spec.MainContainerName)
		if ok {
			find = true
			execUnit = unit
			break
		}
	}

	if !find {
		return fmt.Errorf("not find ready pod of service:[%s]", appId)
	}

	data, err := encodeJson(execOpts)
	if err != nil {
		return err
	}

	cmd, err := structs.GetExecCmd(structs.CmhaServiceType, structs.MaintenanceSetCmd)
	if err != nil {
		return err
	}
	cmd = append(cmd, string(data))
	klog.Infof("cmd:[%v]", cmd)

	ok, _, err := beApp.zone.runInContainer(beApp.GetSiteStr(), execUnit.Namespace, execUnit.GetName(), cmd)
	if err != nil {
		return err
	}

	if !ok {
		return fmt.Errorf("fail to set cmha maintenance")
	}

	return nil
}

type maintenanceSetOpts struct {
	Maintenance bool   `json:"maintenance"`
	IP          string `json:"ip"`
	Port        int    `json:"port"`
}

func (beApp *bankendApp) CmhaReplSetSource(ctx context.Context, appId string, opts api.CmhaReplModeOptions) error {
	iface, err := beApp.zone.siteInterface(beApp.GetSiteStr())
	if err != nil {
		return err
	}

	p, err := iface.Pods().Get(v1.NamespaceDefault, opts.UnitID)
	if err != nil {
		return err
	}
	execOpts := replModeSetOpts{}
	//execOpts.ArchMode = opts.Mode
	execOpts.IP = p.Status.PodIP

	u, err := iface.Units().Get(v1.NamespaceDefault, opts.UnitID)
	if err != nil {
		return err
	}
	execOpts.Port = int(u.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort)

	k8sUnits, err := beApp.listAppK8sUnits(appId, structs.CmhaServiceType)
	if err != nil {
		return err
	}

	if len(k8sUnits) == 0 {
		return stderror.New("cmha is not used in this deployment")
	}

	var execUnit unitv4.Unit
	find := false
	for _, unit := range k8sUnits {
		p, err := beApp.zone.getPod(beApp.GetSiteStr(), unit.Namespace, unit.PodName())
		if err != nil {
			continue
		}

		ok := podutil.IsContainerReady(p, unit.Spec.MainContainerName)
		if ok {
			find = true
			execUnit = unit
			break
		}
	}

	if !find {
		return fmt.Errorf("not find ready pod of service:[%s]", appId)
	}

	data, err := encodeJson(execOpts)
	if err != nil {
		return err
	}

	cmd, err := structs.GetExecCmd(structs.CmhaServiceType, structs.ReplSourceSetCmd)
	if err != nil {
		return err
	}
	cmd = append(cmd, string(data))
	klog.Infof("cmd:[%v]", cmd)

	ok, _, err := beApp.zone.runInContainer(beApp.GetSiteStr(), execUnit.Namespace, execUnit.GetName(), cmd)
	if err != nil {
		return err
	}

	if !ok {
		return fmt.Errorf("fail to set source")
	}

	return nil
}

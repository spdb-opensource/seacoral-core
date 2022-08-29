package bankend

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/upmio/dbscale-kube/pkg/vars"
	"strings"
	"time"

	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/api"
	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/model"
	"github.com/upmio/dbscale-kube/pkg/apis/host/v1alpha1"

	execAPI "github.com/upmio/dbscale-kube/cluster_engine/plugin/execservice/api"
	cryptoutil "github.com/upmio/dbscale-kube/pkg/utils/crypto"
	"github.com/upmio/dbscale-kube/pkg/zone"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

func NewHostBankend(zone zone.ZoneInterface,
	m modelHost,
	clusters clusterGetter,
	sites siteGetter,
	storages storageGetter,
	key string) *bankendHost {

	return &bankendHost{
		key:      key,
		m:        m,
		clusters: clusters,
		sites:    sites,
		storages: storages,
		zone:     zone,
		waits:    NewWaitTasks(),
	}
}

type bankendHost struct {
	key string

	m        modelHost
	clusters clusterGetter
	sites    siteGetter
	storages storageGetter

	zone zone.ZoneInterface

	waits *waitTasks
}

type modelHost interface {
	hostGetter

	Insert(model.Host) (string, string, error)
	InsertHostTask(model.Host, string) (string, error)
	Update(h model.Host, storageMaxUsage *int) (string, error)
	UpdateHostTask(h *model.Host, tk model.Task) error
	Delete(name string) error
	ListUnits() ([]model.Unit, error)
}

type hostGetter interface {
	GetHostBrief(id string) (model.HostBrief, error)
	Get(name string) (model.Host, error)
	List(selector map[string]string) ([]model.Host, error)
}

func (b *bankendHost) Add(ctx context.Context, config api.HostConfig) (api.Host, error) {
	cluster, err := b.clusters.Get(config.Cluster)
	if err != nil {
		return api.Host{}, err
	}

	site, err := b.zone.GetSite(cluster.SiteID)
	if err != nil {
		return api.Host{}, err
	}

	modelSite, err := b.sites.Get(cluster.SiteID)
	if err != nil {
		return api.Host{}, err
	}

	siteInterface, err := site.SiteInterface()
	if err != nil {
		return api.Host{}, err
	}

	storage := model.RemoteStorage{}
	config.Cluster = cluster.ID
	config.SSHConfig.User = cryptoutil.AesEncrypto(config.SSHConfig.User, b.key)
	config.SSHConfig.Password = cryptoutil.AesEncrypto(config.SSHConfig.Password, b.key)

	if config.RemoteStorage != nil && *config.RemoteStorage != "" {
		storage, err = b.storages.Get(*config.RemoteStorage)
		if err != nil {
			return api.Host{}, err
		}

		id := storage.ObjectName()
		config.RemoteStorage = &id
	}

	node := convertToModelHost(config, cluster)

	siteInterface, err = site.SiteInterface()
	if err != nil {
		return api.Host{}, err
	}

	klog.Info("list node of k8s...")
	allNodes, err := siteInterface.Nodes().List(metav1.ListOptions{})
	if err != nil {
		klog.Errorf("list node err:%s", err)
		return api.Host{}, err
	}

	if len(allNodes) == 0 && allNodes == nil {
		klog.Errorf("there are no nodes in k8s cluster !!!")
		return api.Host{}, fmt.Errorf("there are no nodes in k8s cluster !!! ")
	}

	nodeName := ""
	find := false
	for _, each := range allNodes {
		for _, addr := range each.Status.Addresses {
			if addr.Type == corev1.NodeInternalIP && addr.Address == config.SSHConfig.IP.String() {
				find = true
				nodeName = each.Name
				break
			}
		}
	}

	if !find {
		return api.Host{}, fmt.Errorf("not find node which ip=[%s]", config.SSHConfig.IP.String())
	}

	ok := false
	currentCond := ""
	for _, each := range allNodes {
		if each.Name == nodeName {
			for _, cond := range each.Status.Conditions {
				if cond.Type == corev1.NodeReady {
					if cond.Status == corev1.ConditionTrue {
						ok = true
						break
					} else {
						currentCond = string(cond.Status)
					}
				}
			}
		}
	}

	if !ok {
		return api.Host{}, fmt.Errorf("the node is not ready, current condition is [%s]", currentCond)
	}

	node.Hostname = nodeName
	id, task, err := b.m.Insert(node)
	if err != nil {
		return api.Host{}, err
	}

	node.ID = id
	node.Task.ID = task

	host, err := convertToHostV1alpha1(config)
	if err != nil {
		return api.Host{}, err
	}

	host.Name = nodeName
	nodeRole := config.Role
	setHostLabels(host, node, cluster, nodeRole, config.MaxUsage.HostStorage)

	wt := b.waits.NewWaitTask(node.ID, time.Minute, func(err error) error {

		tk := taskUpdate(task, err)
		if err == nil {
			node.Enabled = config.Enabled

			return b.m.UpdateHostTask(&node, tk)
		}

		return b.m.UpdateHostTask(nil, tk)
	})

	go wt.WithTimeout(time.Minute*10, func() (bool, error) {

		clone, err := siteInterface.Hosts().Get(host.Name)
		if errors.IsNotFound(err) {

			if config.RemoteStorage != nil && *config.RemoteStorage != "" {

				san, err := siteInterface.SanSystems().Get(*config.RemoteStorage)
				if err != nil {
					return false, err
				}

				host.Spec.San = &v1alpha1.SanSpec{
					Desc:     config.Desc,
					Location: config.Location.String(),
					//  Os string
					//	Network  string
					Model:     storage.Model,
					San:       san.GetName(),
					HostGroup: host.Spec.HostIP,
					Initiator: v1alpha1.Initiator{
						Type: storage.Type},
				}
			}

			host.Spec.NetworkMode = modelSite.NetworkMode
			clone, err = siteInterface.Hosts().Create(host)
		}

		if err != nil {
			return false, err
		}

		if clone.Status.Phase == v1alpha1.HostReady {
			klog.Info(host.Name, " host ready !!!")

			return true, nil
		}

		return false, nil
	})

	return convertToHostAPI(node, *host), err
}

type usageLimit struct {
	Pod       int `json:"pod"`
	CPU       int `json:"cpu"`
	Memory    int `json:"memory"`
	Storage   int `json:"storage"`
	Bandwidth int `json:"net_bandwidth"`
}

func setHostLabels(host *v1alpha1.Host, node model.Host, cluster model.Cluster, nodeRole string, usageStorage int) {
	if host.Labels == nil {
		host.Labels = make(map[string]string)
	}

	host.Labels[labelCluster] = cluster.ID
	host.Labels[labelSite] = cluster.SiteID
	host.Labels[labelRole] = nodeRole
	host.Labels[vars.LabelDBScaleKey] = vars.LabelDBScaleValue

	if node.RemoteStorageID != "" {
		host.Labels[labelRemoteStorage] = node.RemoteStorageID
	}

	if host.Annotations == nil {
		host.Annotations = make(map[string]string)
	}

	usage := usageLimit{
		Pod:       node.MaxUnit,
		CPU:       node.Max.MaxUsageCPU,
		Memory:    node.Max.MaxUsageMemory,
		Storage:   usageStorage,
		Bandwidth: node.Max.MaxUsageNetBandwidth,
	}

	data, _ := json.Marshal(usage)
	host.Annotations[annotationHostUsageLimit] = string(data)
}

func parseQuantity(s string) (int64, error) {
	q, err := resource.ParseQuantity(s)
	if err != nil {
		return 0, err
	}

	n, _ := q.AsInt64()

	return n, nil
}

func convertToHostV1alpha1(req api.HostConfig) (*v1alpha1.Host, error) {
	host := &v1alpha1.Host{
		Spec: v1alpha1.HostSpec{
			HostIP:        req.SSHConfig.IP.String(),
			HostPort:      int64(req.SSHConfig.Port),
			OsUser:        req.SSHConfig.User,
			OsPassword:    req.SSHConfig.Password,
			MaxPod:        int64(req.MaxUnit),
			NtpServer:     req.NTPServer,
			Unschedulable: !req.Enabled,
		},
	}

	host.Spec.LocalVGs = make([]v1alpha1.VGSpec, len(req.HostStorages))

	for i, ls := range req.HostStorages {

		host.Spec.LocalVGs[i] = v1alpha1.VGSpec{
			Name:    vgName(string(ls.Performance)),
			Level:   v1alpha1.Level(ls.Performance),
			Devices: ls.Paths,
		}
	}

	return host, nil
}

func vgName(level string) string {

	return strings.Join([]string{"local", level, "VG"}, "_")
}

func convertToModelHost(req api.HostConfig, cluster model.Cluster) model.Host {
	rs := ""
	if req.RemoteStorage != nil {
		rs = *req.RemoteStorage
	}

	hss := make([]model.HostStorage, len(req.HostStorages))
	for i, hs := range req.HostStorages {
		hss[i] = model.HostStorage{
			MaxUsage:    req.MaxUsage.HostStorage,
			Name:        vgName(string(hs.Performance)),
			Performance: string(hs.Performance),
			Paths:       model.NewSliceString(hs.Paths),
		}
	}

	return model.Host{
		// Enabled: req.Enabled,
		// Name:      req.Name,
		IP:        req.SSHConfig.IP.String(),
		ClusterID: cluster.ID,
		Cluster: model.ClusterBrief{
			Name:   cluster.Name,
			SiteID: cluster.SiteID,
		},
		Location: model.Location{
			Room: req.Location.Room,
			Seat: req.Location.Seat,
		},
		RemoteStorageID: rs,
		Desc:            req.Desc,
		Max: model.Max{
			MaxUnit:              req.MaxUnit,
			MaxUsageCPU:          req.MaxUsage.CPU,
			MaxUsageMemory:       req.MaxUsage.Memory,
			MaxUsageNetBandwidth: req.MaxUsage.NetBandwidth,
		},
		HostStorages: hss,

		Editor: newCreateEditor(req.User),
	}
}

func (b *bankendHost) GetDetail(ctx context.Context, hostID string) (api.HostDetail, error) {
	hostDetail := api.HostDetail{}

	host, err := b.m.Get(hostID)
	if model.IsNotExist(err) {
		return hostDetail, nil
	}
	if err != nil {
		return hostDetail, err
	}

	iface, err := b.zone.HostInterface(host.Cluster.SiteID)
	if err != nil {
		return hostDetail, fmt.Errorf("Do HostInterface err: %s", err)
	}

	h, err := iface.Get(host.Hostname)
	if err == nil {
		maxHostStorage := 0

		hss := make([]api.HostStorageBrief, len(host.HostStorages))

		for i, hs := range host.HostStorages {
			maxHostStorage = hs.MaxUsage

			hss[i] = api.HostStorageBrief{
				HostStorage: api.HostStorage{
					Performance: api.Performance(hs.Performance),
					Paths:       hs.Paths.Strings(),
				},
			}

			var capacity, free v1alpha1.VGStatus

			for _, vg := range h.Status.Capacity.LocalVGs {
				if vg.Name == hs.Name {
					capacity = vg
					break
				}
			}

			for _, vg := range h.Status.Allocatable.LocalVGs {
				if vg.Name == hs.Name {
					free = vg
					break
				}
			}

			total := convertQuantityToMi(capacity.Size)
			capacity.Size.Sub(free.Size)
			used := convertQuantityToMi(capacity.Size)

			hss[i].ResourceStatus = api.ResourceStatus{
				Capacity: total,
				Used:     used,
			}
		}

		hostIP := h.Spec.HostIP

		hostDetail = api.HostDetail{
			Enabled:   host.Enabled,
			ID:        host.ID,
			HostName:  host.Hostname,
			Desc:      host.Desc,
			Cluster:   api.NewIDName(host.ClusterID, host.Cluster.Name),
			Arch:      h.Status.NodeInfo.Architecture,
			NTPServer: h.Spec.NtpServer,
			Role:      h.Labels[labelRole],
			Location: api.Location{
				Room: host.Location.Room,
				Seat: host.Location.Seat,
			},

			HostStorages:  hss,
			RemoteStorage: api.NewIDName(host.RemoteStorageID, host.RemoteStorageName),
			Node: api.NodeInfo{
				Name: h.Name,
				IP:   hostIP,
				Os:   h.Status.NodeInfo.OperatingSystem,
			},

			ResourceLimit: api.ResourceLimit{
				MaxUnit: host.MaxUnit,
				MaxUsage: api.MaxUsage{
					CPU:          host.MaxUsageCPU,
					Memory:       host.MaxUsageMemory,
					HostStorage:  maxHostStorage,
					NetBandwidth: host.MaxUsageNetBandwidth,
				},
			},
			Task:     convertToTaskBrief(host.Task),
			Created:  api.NewEditor(host.CreatedUser, host.CreatedAt),
			Modified: api.NewEditor(host.ModifiedUser, host.ModifiedAt),
		}

		hostDetail.Node.CPU.Capacity = h.Status.Capacity.Cpu.Value()
		hostDetail.Node.CPU.Used = h.Status.Capacity.Cpu.Value() - h.Status.Allocatable.Cpu.Value()

		hostDetail.Node.Memory.Capacity = convertQuantityToMi(h.Status.Capacity.Memery)
		h.Status.Capacity.Memery.Sub(h.Status.Allocatable.Memery)
		hostDetail.Node.Memory.Used = convertQuantityToMi(h.Status.Capacity.Memery)

		hostDetail.Node.Pod.Capacity = h.Status.Capacity.Pods.Value()
		hostDetail.Node.Pod.Used = h.Status.Capacity.Pods.Value() - h.Status.Allocatable.Pods.Value()

		siteId := host.Cluster.SiteID
		siteiface, err := b.zone.SiteInterface(siteId)
		if err != nil {
			return api.HostDetail{}, fmt.Errorf("b.zone.SiteInterface err: %s", err)
		}

		var units []api.UnitBrief
		podList, err := siteiface.Pods().List(h.Namespace, metav1.ListOptions{})
		if err != nil {
			return api.HostDetail{}, fmt.Errorf("List all pods about this host err: %s", err)
		}

		for i := range podList {
			pod := podList[i]
			unit := api.UnitBrief{}
			if pod.Status.HostIP == hostIP {
				app, ok := pod.Labels["app"]
				if !ok {
					continue
				}

				unit.Name = pod.GetName()
				unit.Type = pod.Spec.Containers[0].Name
				unit.State = string(pod.Status.Phase)
				unit.ID = app
				//unit.NetBandwidth= 0

				for _, container := range pod.Spec.Containers {
					for rName, rQuant := range container.Resources.Requests {
						switch rName {
						case corev1.ResourceCPU:
							unit.CPU += rQuant.Value()
						case corev1.ResourceMemory:
							unit.Memory += rQuant.Value()
						}
					}
				}

				units = append(units, unit)
			}

		}
		hostDetail.Units = units
		//hostDetail.ExistingUnit = len(units)
	} else {
		return hostDetail, fmt.Errorf("ifaec get err: %s", err)
	}

	return hostDetail, nil
}

func (b *bankendHost) List(ctx context.Context, id, name, cluster, site, enabled string) ([]api.Host, error) {
	selector := make(map[string]string)

	if id != "" {
		selector["id"] = id
	}
	if name != "" {
		selector["name"] = name
	}
	if cluster != "" {
		selector["cluster_id"] = cluster
	}
	if site != "" {
		selector["site_id"] = site
	}
	if enabled != "" {
		selector[labelEnabled] = boolValue(enabled)
	}

	list, err := b.m.List(selector)
	if model.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	hosts := make([]api.Host, len(list))

	for i := range list {

		hostInterface, err := b.zone.HostInterface(list[i].Cluster.SiteID)
		if err != nil {
			hosts[i] = convertToHostAPI(list[i], v1alpha1.Host{})
			continue
		}

		h, err := hostInterface.Get(list[i].Hostname)
		if err == nil {
			hosts[i] = convertToHostAPI(list[i], *h)
		} else {
			klog.Info("Error " + err.Error())
			hosts[i] = convertToHostAPI(list[i], v1alpha1.Host{})
		}

	}

	return hosts, nil
}

func convertToHostAPI(h model.Host, hostv1 v1alpha1.Host) api.Host {
	maxHostStorage := 0

	hss := make([]api.HostStorageBrief, len(h.HostStorages))

	for i, hs := range h.HostStorages {
		maxHostStorage = hs.MaxUsage

		hss[i] = api.HostStorageBrief{
			HostStorage: api.HostStorage{
				Performance: api.Performance(hs.Performance),
				Paths:       hs.Paths.Strings(),
			},
		}

		var capacity, free v1alpha1.VGStatus

		for _, vg := range hostv1.Status.Capacity.LocalVGs {
			if vg.Name == hs.Name {
				capacity = vg
				break
			}
		}

		for _, vg := range hostv1.Status.Allocatable.LocalVGs {
			if vg.Name == hs.Name {
				free = vg
				break
			}
		}

		total := convertQuantityToMi(capacity.Size)
		capacity.Size.Sub(free.Size)
		used := convertQuantityToMi(capacity.Size)

		hss[i].ResourceStatus = api.ResourceStatus{
			Capacity: total,
			Used:     used,
		}
	}

	host := api.Host{
		Enabled:   h.Enabled,
		ID:        h.ID,
		Desc:      h.Desc,
		Cluster:   api.NewIDName(h.ClusterID, h.Cluster.Name),
		Arch:      hostv1.Status.NodeInfo.Architecture,
		NTPServer: hostv1.Spec.NtpServer,
		Role:      hostv1.Labels[labelRole],
		Location: api.Location{
			Room: h.Location.Room,
			Seat: h.Location.Seat,
		},

		// 本地存储
		HostStorages:  hss,
		RemoteStorage: api.NewIDName(h.RemoteStorageID, h.RemoteStorageName),
		Node: api.NodeInfo{
			Name: hostv1.Name,
			IP:   hostv1.Spec.HostIP,
			Os:   hostv1.Status.NodeInfo.OperatingSystem,
		},

		// TODO: units info
		//	Units []UnitBrief `json:"units"`

		ResourceLimit: api.ResourceLimit{
			MaxUnit: h.MaxUnit,
			MaxUsage: api.MaxUsage{
				CPU:          h.MaxUsageCPU,
				Memory:       h.MaxUsageMemory,
				HostStorage:  maxHostStorage,
				NetBandwidth: h.MaxUsageNetBandwidth,
			},
		},
		Task:     convertToTaskBrief(h.Task),
		Created:  api.NewEditor(h.CreatedUser, h.CreatedAt),
		Modified: api.NewEditor(h.ModifiedUser, h.ModifiedAt),
	}

	host.Node.CPU.Capacity = hostv1.Status.Capacity.Cpu.Value()
	host.Node.CPU.Used = hostv1.Status.Capacity.Cpu.Value() - hostv1.Status.Allocatable.Cpu.Value()

	host.Node.Memory.Capacity = convertQuantityToMi(hostv1.Status.Capacity.Memery)
	hostv1.Status.Capacity.Memery.Sub(hostv1.Status.Allocatable.Memery)
	host.Node.Memory.Used = convertQuantityToMi(hostv1.Status.Capacity.Memery)

	host.Node.Pod.Capacity = hostv1.Status.Capacity.Pods.Value()
	host.Node.Pod.Used = hostv1.Status.Capacity.Pods.Value() - hostv1.Status.Allocatable.Pods.Value()

	return host
}

func (b *bankendHost) Set(ctx context.Context, id string, opts api.HostOptions) (api.Host, error) {
	host, err := b.m.Get(id)
	if err != nil {
		return api.Host{}, err
	}
	hostback := host
	host = mergeHost(host, opts)

	task, err := b.m.Update(host, opts.MaxUsage.HostStorage)
	if err != nil {
		return api.Host{}, err
	}

	wt := b.waits.NewWaitTask(host.ID, retryInterval, func(err error) error {
		tk := taskUpdate(task, err)
		if err != nil {
			host.Enabled = hostback.Enabled
		}

		return b.m.UpdateHostTask(&host, tk)
	})

	go wt.WithTimeout(10*time.Minute, func() (bool, error) {

		iface, err := b.zone.HostInterface(host.Cluster.SiteID)
		if err != nil {
			return false, err
		}

		hostNode, err := iface.Get(host.Hostname)
		if err != nil {
			return false, err
		}

		if hostNode.Spec.Unschedulable == !host.Enabled {
			return true, nil
		}

		hostNode = hostNode.DeepCopy()
		hostNode.Spec.Unschedulable = !host.Enabled

		if opts.Role != "" {
			hostNode.Labels[labelRole] = opts.Role
		}

		hostNode, err = iface.Update(hostNode)
		if err != nil {
			return false, err
		}
		return true, nil
	})

	host.Task = model.Task{
		ID: task,
	}

	return convertToHostAPI(host, v1alpha1.Host{}), err
}

func mergeHost(host model.Host, opts api.HostOptions) model.Host {
	if opts.Room != nil {
		host.Location.Room = *opts.Room
	}

	if opts.Seat != nil {
		host.Location.Seat = *opts.Seat
	}

	if opts.Desc != nil {
		host.Desc = *opts.Desc
	}

	if opts.Enabled != nil {
		host.Enabled = *opts.Enabled
	}

	if opts.MaxUnit != nil {
		host.MaxUnit = *opts.MaxUnit
	}

	if opts.MaxUsage.CPU != nil {
		host.MaxUsageCPU = *opts.MaxUsage.CPU
	}

	if opts.MaxUsage.Memory != nil {
		host.MaxUsageMemory = *opts.MaxUsage.Memory
	}

	if opts.MaxUsage.HostStorage != nil {
		for i := range host.HostStorages {
			host.HostStorages[i].MaxUsage = *opts.MaxUsage.HostStorage
		}
	} else {
		host.HostStorages = nil
	}

	if opts.MaxUsage.NetBandwidth != nil {
		host.MaxUsageNetBandwidth = *opts.MaxUsage.NetBandwidth
	}

	if opts.User != "" {
		host.ModifiedUser = opts.User
	}

	host.ModifiedAt = time.Now()

	return host
}

func (b *bankendHost) Delete(ctx context.Context, id, user, password string, port int) (api.TaskObjectResponse, error) {
	host, err := b.m.Get(id)
	if model.IsNotExist(err) {
		return api.TaskObjectResponse{}, nil
	}
	if err != nil {
		return api.TaskObjectResponse{}, err
	}

	iface, err := b.zone.HostInterface(host.Cluster.SiteID)
	if err != nil {
		return api.TaskObjectResponse{}, err
	}

	node, err := iface.Get(host.Hostname)
	if err != nil {
		return api.TaskObjectResponse{}, err
	}

	units, err := b.listUnitsInHost(host.Cluster.SiteID, node.Name)
	if err != nil {
		klog.Errorf("list units in node: %s err: %v", node.Name, err)
		return api.TaskObjectResponse{}, err
	}

	if len(units) != 0 {
		return api.TaskObjectResponse{}, fmt.Errorf("since there are pods on this host, you cannot delete it")
	}

	task, err := b.m.InsertHostTask(host, model.ActionHostDelete)
	if err != nil {
		return api.TaskObjectResponse{}, err
	}

	b.waits.Delete(host.ID)

	wt := NewWaitTask(time.Minute, func(err error) error {
		if err == nil {
			err = b.m.Delete(host.ID)
		}

		tk := taskUpdate(task, err)

		return b.m.UpdateHostTask(nil, tk)
	})

	go wt.WithTimeout(time.Minute*10, func() (bool, error) {

		if node != nil && node.Spec.ActCode != v1alpha1.DeletCode {

			node = node.DeepCopy()
			node.Spec.ActCode = v1alpha1.DeletCode
			node.Spec.OsPassword = cryptoutil.AesEncrypto(password, b.key)
			node.Spec.OsUser = cryptoutil.AesEncrypto(user, b.key)
			node.Spec.HostPort = int64(port)

			_, err = iface.Update(node)
			if errors.IsNotFound(err) {
				return true, nil
			}
		} else {
			return true, nil
		}

		return false, err
	})

	return api.TaskObjectResponse{
		ObjectID:   host.ID,
		ObjectName: host.IP,
		TaskID:     task,
	}, err
}

func (b *bankendHost) ValidateHost(ctx context.Context, hlc api.HostConfig, checkType string) error {
	cluster, err := b.clusters.Get(hlc.Cluster)
	if err != nil {
		return err
	}

	opt := execAPI.LegalizeHostOption{
		SSHUser:    hlc.SSHConfig.User,
		SSHPasswod: hlc.SSHConfig.Password,
		SSHPort:    int64(hlc.SSHConfig.Port),
		SSHIP:      hlc.SSHConfig.IP.String(),
		HostName:   hlc.SSHConfig.IP.String(),
		MaxUnit:    hlc.MaxUnit,
		NtpServer:  hlc.NTPServer,
	}
	opt.LocalVGs = []execAPI.LocalVGCfg{}
	for _, ls := range hlc.HostStorages {
		opt.LocalVGs = append(opt.LocalVGs, execAPI.LocalVGCfg{
			Devices: strings.Join(ls.Paths, ","),
			Name:    vgName(string(ls.Performance)),
		})
	}

	opt.CheckType = checkType

	iface, err := b.zone.SiteInterface(cluster.SiteID)
	if err != nil {
		return err
	}

	err = iface.HostLegalizeExec().HostLegalize(opt)
	if err != nil {
		return err
	}

	return nil
}

// ListPodInNode get node namespace != kube-system
func (b *bankendHost) ListPodInNode(site, nodeName string) ([]corev1.Pod, error) {
	var workerPods []corev1.Pod
	iface, err := b.zone.SiteInterface(site)
	if err != nil {
		klog.Errorf("get SiteInterface: site: %s ERR: %s", site, err)
		return nil, err
	}

	pods, err := iface.Pods().List("", metav1.ListOptions{
		FieldSelector: "spec.nodeName=" + nodeName,
	})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return pods, err
	}

	for _, pod := range pods {
		if pod.Namespace != KubeSystemNameSpace {
			workerPods = append(workerPods, pod)
		}
	}

	return workerPods, nil
}

func (b *bankendHost) listUnitsInHost(site, nodeName string) ([]corev1.Pod, error) {
	var pu []corev1.Pod
	iface, err := b.zone.SiteInterface(site)
	if err != nil {
		klog.Errorf("get SiteInterface: site: %s ERR: %s", site, err)
		return pu, err
	}

	pods, err := iface.Pods().List("", metav1.ListOptions{
		FieldSelector: "spec.nodeName=" + nodeName,
	})
	if err != nil {
		if errors.IsNotFound(err) {
			return pu, nil
		}
		return pu, err
	}

	mus, err := b.m.ListUnits()
	if err != nil {
		return pu, fmt.Errorf("list unit in database err:%s", err)
	}

	if mus == nil {
		return pu, nil
	}

	for _, pod := range pods {
		for _, mu := range mus {
			if pod.Name == mu.ID {
				pu = append(pu, pod)
			}
		}
	}

	return pu, nil
}

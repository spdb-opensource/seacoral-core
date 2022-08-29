package bankend

import (
	"encoding/json"
	stderrors "errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"

	hostv1 "github.com/upmio/dbscale-kube/pkg/apis/host/v1alpha1"
	netv1 "github.com/upmio/dbscale-kube/pkg/apis/networking/v1alpha1"
	"github.com/upmio/dbscale-kube/pkg/structs"
	"github.com/upmio/dbscale-kube/pkg/vars"

	"k8s.io/klog/v2"

	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/api"
	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/model"
	unitv4 "github.com/upmio/dbscale-kube/pkg/apis/unit/v1alpha4"
	"github.com/upmio/dbscale-kube/pkg/utils"
	podutil "github.com/upmio/dbscale-kube/pkg/utils/pod"
	"github.com/upmio/dbscale-kube/pkg/zone/site"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func NewScheduler(
	clusters clusterGetter,
	networks networkGetter,
	hosts hostGetter,
	storages storageGetter,
	pools poolGetter) *scheduler {

	return &scheduler{
		//	sites    siteGetter
		clusters: clusters,
		networks: networks,
		hosts:    hosts,
		storages: storages,
		pools:    pools,
	}
}

type scheduler struct {
	//	sites    siteGetter
	clusters clusterGetter
	networks networkGetter
	hosts    hostGetter
	storages storageGetter
	pools    poolGetter
}

type matches struct {
	cluster model.Cluster
	network model.Network
	hosts   []model.Host
}

type plan struct {
	topology string
	group    []matches
}

const GroupNamePattern string = "%02d"

func (s *scheduler) Search(requests api.ResourceRequirements, conditions api.Conditions, image string, replicas int) ([]plan, error) {
	var (
		err      error
		rss      []model.RemoteStorage
		networks map[string][]model.Network
	)

	clusters, err := s.filterClusterCondition(conditions.Cluster, image)
	if err != nil {
		return nil, stderrors.New("filterClusterCondition is null")
	}

	if requests.Bandwidth != nil {

		networks, err = s.filterNetworkCondition(conditions.Network)
		if err != nil {
			return nil, stderrors.New("filterNetworkCondition is null")
		}
	}

	hosts, err := s.filterHostCondition(replicas, conditions.Host)
	if err != nil {
		return nil, stderrors.New("filterHostCondition is null / " + err.Error())
	}

	if requests.Storage != nil && requests.Storage.Type == api.StorageTypeRemote {

		rss, err = s.filterRemoteStorageCondition(*requests.Storage, conditions.RemoteStorage)
		if err != nil {
			return nil, stderrors.New("filterRemoteStorageCondition is null")
		}
	}

	out := s.matches(clusters, networks, hosts, rss, requests, conditions, replicas)
	if len(out) == 0 {
		return nil, stderrors.New("no plan matched")
	}

	return out, nil
}

func (s *scheduler) matches(
	clusters []model.Cluster,
	networks map[string][]model.Network,
	hosts map[string][]model.Host,
	rss []model.RemoteStorage,
	requests api.ResourceRequirements,
	conditions api.Conditions,
	replicas int) []plan {

	out := make([]plan, 0, len(clusters))

	if requests.Bandwidth == nil {

		plan := plan{group: make([]matches, 0, len(clusters))}

		for _, cluster := range clusters {

			single := matches{
				cluster: cluster,
				hosts:   hosts[cluster.ID],
			}

			list := s.filterHostByStorage(requests.Storage, single.hosts, rss)
			if len(list) > 0 {
				single.hosts = list

				plan.group = append(plan.group, single)
			}
		}

		if len(plan.group) > 0 {
			out = append(out, plan)
		}

	} else {

		for topo, nets := range networks {

			plan := plan{
				topology: topo,
				group:    make([]matches, 0, len(nets)),
			}

			for _, net := range nets {

				single := matches{
					network: net,
				}

			loop:
				for _, cluster := range clusters {

					if cluster.ID == net.ClusterID {

						single.cluster = cluster
						single.hosts = hosts[cluster.ID]

						break loop
					}
				}

				list := s.filterHostByStorage(requests.Storage, single.hosts, rss)
				if len(list) > 0 {
					single.hosts = list

					plan.group = append(plan.group, single)
				}
			}

			if len(plan.group) > 0 {
				out = append(out, plan)
			}
		}
	}

	plans := make([]plan, 0, len(out))

loop1:
	for _, gp := range out {

		if conditions.Network.HighAvailability &&
			requests.Bandwidth != nil {

			if len(gp.group) < 2 {
				continue loop1
			}
		}

		if conditions.Cluster.HighAvailability {

			ids := make(map[string]struct{}, len(gp.group))

			for _, v := range gp.group {
				ids[v.cluster.ID] = struct{}{}
			}

			if len(ids) < 2 {
				continue loop1
			}
		}

		if conditions.Host.HighAvailability {
			hosts := make(map[string]struct{})

			for _, obj := range gp.group {

				for _, host := range obj.hosts {
					hosts[host.ID] = struct{}{}
				}
			}

			if len(hosts) < replicas {
				continue loop1
			}
		}

		sortGroupByHostNumber(gp.group)

		plans = append(plans, gp)
	}

	sortPlanByGroupNumber(plans)

	return plans
}

func (s *scheduler) filterHostByStorage(requests *api.StorageRequirement, hosts []model.Host, rss []model.RemoteStorage) []model.Host {
	if len(hosts) == 0 {
		return nil
	}

	if requests == nil {
		return hosts

	} else if requests.Type == api.StorageTypeHost {

		hlist := make([]model.Host, 0, len(hosts))

		for _, host := range hosts {

			for _, hs := range host.HostStorages {
				if hs.Performance == string(requests.Performance) {
					hlist = append(hlist, host)

					break
				}
			}
		}

		return hlist

	}

	hlist := make([]model.Host, 0, len(hosts))

	for _, host := range hosts {

		if host.RemoteStorageID == "" {
			continue
		}

		for l := range rss {

			if host.RemoteStorageID == rss[l].ID {
				hlist = append(hlist, host)
				break
			}
		}
	}

	return hlist
}

type planController struct {
	zone zoneIface

	units []unitv4.Unit
}

func NewPlanController(z zoneIface) *planController {
	return &planController{
		zone: z,
	}
}

func (ctrl *planController) DeployService(appName, groupName, groupType string, replicas int, tmpl unitv4.Unit) error {

	var errs []error
	var uuids []string

	ctrl.units = make([]unitv4.Unit, 0, replicas)
	groups := map[string]int{}

	for count := 0; count < replicas; count += 1 {
		uuids = append(uuids, utils.NewUUID()[:3])
	}

	for count := 0; count < replicas; count += 1 {
		name := fmt.Sprintf("%s-%s-%d", groupName, uuids[count], count)
		key := fmt.Sprintf("%s/%s", tmpl.Namespace, name)
		groups[key] = count
	}

	groupStr, err := json.Marshal(groups)
	if err != nil {
		return err
	}

	for count := 0; count < replicas; count += 1 {
		unit := tmpl.DeepCopy()

		options := map[string]string{"count": strconv.Itoa(count + 1)}
		setSpecificEnv(unit, options)
		_ = setSpecificAnnotation(unit, options)

		if err != nil {
			return err
		}
		unit.Annotations[unitv4.PodGroupAnnotation] = string(groupStr)

		unit.Name = fmt.Sprintf("%s-%s-%d", groupName, uuids[count], count)
		//unit.Spec.Site = match.cluster.SiteID

		//if unit.Spec.Networking != nil || requests.Bandwidth != nil {
		//	unit.Spec.Networking.Network = match.network.ObjectName()
		//}

		unit, err = ctrl.zone.createUnit(unit)
		if err != nil {
			errs = append(errs, err)
			ctrl.units = append(ctrl.units, *unit)
			continue
		}

		err = wait.PollImmediate(3*time.Second, 60*time.Second, func() (bool, error) {
			_, err := ctrl.zone.getPod(ctrl.zone.GetSite(), unit.Namespace, unit.Name)
			if err != nil {
				return false, nil
			}

			return true, nil
		})

		ctrl.units = append(ctrl.units, *unit)
	}

	return utilerrors.NewAggregate(errs)
}

func (ctrl *planController) deleteUnits(units []unitv4.Unit) error {
	if len(units) == 0 {
		return nil
	}

	var errs []error

	for _, unit := range units {

		err := ctrl.zone.deleteUnit(unit)
		if err != nil && !errors.IsNotFound(err) {
			errs = append(errs, err)
		}
	}

	return utilerrors.NewAggregate(errs)
}

func (beApp bankendApp) checkUnitStatusOK(units []unitv4.Unit, interval time.Duration, ready bool) ([]unitv4.Unit, error) {
	var errs []error
	dead := make([]unitv4.Unit, 0, len(units))

	for i, unit := range units {

		iface, err := beApp.zone.siteInterface(beApp.GetSiteStr())
		if err != nil {
			dead = append(dead, unit)
			errs = append(errs, err)
			continue
		}

		obj, err := iface.Units().Get(unit.Namespace, unit.Name)
		if err != nil {
			dead = append(dead, unit)
			errs = append(errs, err)
			continue
		}
		units[i] = *obj

		isTimeout := time.Since(obj.GetCreationTimestamp().Time) > interval

		podName := unit.PodName()

		pod, err := iface.Pods().Get(unit.Namespace, podName)
		if err != nil {
			if isTimeout {
				dead = append(dead, unit)
			}

			errs = append(errs, err)
			continue
		}

		isTimeout = time.Since(pod.GetCreationTimestamp().Time) > interval

		if pod.Spec.NodeName == "" || !podutil.IsRunning(pod) {
			if isTimeout {
				dead = append(dead, unit)
			}

			errs = append(errs,
				fmt.Errorf("Pod %s is not running,current phase is %s,retry later",
					pod.Name, pod.Status.Phase))

			continue
		}

		if isTimeout && !podutil.IsRunningAndReady(pod) {
			dead = append(dead, unit)

			errs = append(errs,
				fmt.Errorf("Pod %s is running but not ready,current phase is %s,retry later", pod.Name, pod.Status.Phase))

			continue
		}

		if ready && !podutil.IsRunningAndReady(pod) {
			errs = append(errs,
				fmt.Errorf("Pod %s is not ready,current phase is %s,retry later",
					pod.Name, pod.Status.Phase))
		}
	}

	return dead, utilerrors.NewAggregate(errs)
}

func sortGroupByHostNumber(list []matches) {

	sort.Slice(list,
		func(i, j int) bool {
			return len(list[i].hosts) > len(list[j].hosts)
		})
}

func sortPlanByGroupNumber(plans []plan) {

	sort.Slice(plans,
		func(i, j int) bool {

			if len(plans[i].group) > len(plans[j].group) {
				return true
			}

			mi := make(map[string]struct{})
			mj := make(map[string]struct{})

			for _, list := range plans[i].group {

				for k := range list.hosts {
					mi[list.hosts[k].IP] = struct{}{}
				}
			}

			for _, list := range plans[j].group {

				for k := range list.hosts {
					mj[list.hosts[k].IP] = struct{}{}
				}
			}

			return len(mi) > len(mj)
		})
}

func sortUnitsByNameOrdinal(units []unitv4.Unit) {

	names := make(map[string]string, len(units))

	getOrdinal := func(name string) string {
		if v, ok := names[name]; ok {
			return v
		}

		ps := strings.Split(name, "-")

		last := ps[len(ps)-1]

		names[name] = last

		return last
	}

	sort.Slice(units,
		func(i, j int) bool {

			return getOrdinal(units[i].Name) < getOrdinal(units[j].Name)
		})
}

func groupName(name, type_ string, ord int) string {
	if type_ == structs.ProxysqlServiceType {
		//just shorten the name
		type_ = structs.ProxysqlServiceTypeSN
	}
	return fmt.Sprintf("%s-%s"+GroupNamePattern, name, type_, ord)
}

func setSpecificEnv(unit *unitv4.Unit, _ map[string]string) {
	switch unit.Spec.MainContainerName {
	case structs.MysqlServiceType:
	}
}

func filterClusterCondition(cluster clusterGetter, candidates []string, image string) ([]string, error) {
	var (
		err  error
		list []model.Cluster
	)

	if len(candidates) == 0 {
		list, err = cluster.List(map[string]string{})
		if err != nil {
			return nil, err
		}
	} else {
		list = make([]model.Cluster, 0, len(candidates))
		for i := range candidates {
			cluster, err := cluster.Get(candidates[i])
			if err != nil {
				return nil, err
			}
			list = append(list, cluster)
		}
	}

	out := make([]string, 0, len(list))
	for i := range list {
		if list[i].Enabled && list[i].Images.In(image) {
			out = append(out, list[i].ID)
		}
	}

	if len(out) == 0 {
		return out, fmt.Errorf("not find suitable cluster")
	}

	return out, nil
}

func filterRemoteStorageCondition(storages storageGetter, pools poolGetter, requests api.StorageRequirement, candidates []string) ([]string, error) {

	if requests.Type == api.StorageTypeHost {
		return nil, nil
	}

	var (
		err  error
		list []model.RemoteStorage
	)

	if len(candidates) == 0 {
		list, err = storages.List(map[string]string{})
		if err != nil {
			return nil, err
		}

	} else {
		list = make([]model.RemoteStorage, 0, len(candidates))
		for i := range candidates {
			rs, err := storages.Get(candidates[i])
			if err != nil {
				return nil, err
			}
			list = append(list, rs)
		}
	}

	if requests.Performance != "" {
		out := make([]model.RemoteStorage, 0, len(list))
	loop:
		for i := range list {
			pools, _ := pools.ListPools(map[string]string{
				"remote_storage_id": list[i].ID,
			})

			for k := range pools {
				if pools[k].Performance == string(requests.Performance) {
					out = append(out, list[i])
					continue loop
				}
			}
		}
		list = out
	}

	if len(list) == 0 {
		return nil, fmt.Errorf("not find suitable remote storage")
	}

	ret := []string{}
	for i := range list {
		ret = append(ret, list[i].ID)
	}
	return ret, nil
}

func filterClusterByNetwork(siteiface site.Interface, clusters []string) ([]string, error) {
	var ret []string
	networks, err := siteiface.Networks().List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for i := range networks {
		if networks[i].Status.AllIPCounts == networks[i].Status.UsedIPCount {
			continue
		}

		clusterLabel, ok := networks[i].Labels[labelCluster]
		if !ok {
			klog.Warningf("%s: not find  %s lable", networks[i].Name, labelCluster)
			continue
		}

		if networks[i].Annotations[netv1.NetworkPhaseAnnotation] == netv1.NetworkmaintenancePhase {
			continue
		}

		if utils.ContainsString(clusters, clusterLabel) &&
			!utils.ContainsString(ret, clusterLabel) {
			ret = append(ret, clusterLabel)
		}
	}

	if len(ret) == 0 {
		// lijj32: correct spell problem.
		return nil, fmt.Errorf("not find suitable clusters after filter by network(before:%v)", clusters)
	}

	return ret, nil
}

func (beApp *bankendApp) injectSchedulerInfo(unit *unitv4.Unit, arch string, replicas int, conditions api.Conditions, image model.Image, storagerequests *api.StorageRequirement) error {
	if unit.Spec.Template.Spec.NodeSelector == nil {
		unit.Spec.Template.Spec.NodeSelector = map[string]string{}
	}

	unit.Spec.Template.Spec.NodeSelector[labelRole] = vars.NodeRolenode
	unit.Spec.Template.Spec.NodeSelector[corev1.LabelArchStable] = arch

	//TODO: use setpodAffinity
	unit.Spec.Template.Spec.Affinity = &corev1.Affinity{}
	if conditions.Host.HighAvailability {
		term := corev1.PodAffinityTerm{
			TopologyKey: corev1.LabelHostname,
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      labelAppIDAndGroup,
						Operator: metav1.LabelSelectorOpIn,
						Values:   []string{unit.Labels[labelAppIDAndGroup]},
					},
				},
			},
		}

		unit.Spec.Template.Spec.Affinity.PodAntiAffinity = &corev1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
				term,
			},
		}
	}

	var expressions []corev1.NodeSelectorRequirement
	var constraints []corev1.TopologySpreadConstraint

	clusters, err := filterClusterCondition(beApp.clusters, conditions.Cluster.Candidates, image.Type)
	if err != nil {
		return err
	}

	if conditions.Cluster.HighAvailability && replicas >= 2 {
		if len(clusters) < 2 {
			return fmt.Errorf("at least 2 clusters are required, currently only %d cluster is available", len(clusters))
		}
	}

	siteiface, err := beApp.zone.siteInterface(image.SiteID)
	if err != nil {
		return err
	}

	clusters, err = filterClusterByNetwork(siteiface, clusters)
	if err != nil {
		return err
	}

	if conditions.Network.HighAvailability && replicas >= 2 {
		if len(clusters) < 2 {
			return fmt.Errorf("at least 2 networks (in 2 clusters) are required, currently only %d cluster with available network", len(clusters))
		}
	}

	expressions = append(expressions, corev1.NodeSelectorRequirement{
		Key:      labelCluster,
		Operator: corev1.NodeSelectorOpIn,
		Values:   clusters,
	})

	clusterUnsatisfiableConstraintAction := corev1.ScheduleAnyway
	if conditions.Cluster.HighAvailability {
		clusterUnsatisfiableConstraintAction = corev1.DoNotSchedule
	}

	constraints = append(constraints, corev1.TopologySpreadConstraint{
		MaxSkew:           1,
		TopologyKey:       labelCluster,
		WhenUnsatisfiable: clusterUnsatisfiableConstraintAction,
		LabelSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				labelAppID:       unit.Labels[labelAppID],
				labelServiceType: unit.Labels[labelServiceType],
			},
		},
	})

	if storagerequests.Type == api.StorageTypeRemote {
		storages, err := filterRemoteStorageCondition(beApp.storages, beApp.pools, *storagerequests, conditions.RemoteStorage.Candidates)
		if err != nil {
			return err
		}
		expressions = append(expressions, corev1.NodeSelectorRequirement{
			Key:      labelRemoteStorage,
			Operator: corev1.NodeSelectorOpIn,
			Values:   storages,
		})

		storageUnsatisfiableConstraintAction := corev1.ScheduleAnyway
		if conditions.RemoteStorage.HighAvailability {
			storageUnsatisfiableConstraintAction = corev1.DoNotSchedule
		}

		constraints = append(constraints, corev1.TopologySpreadConstraint{
			MaxSkew:           1,
			TopologyKey:       labelRemoteStorage,
			WhenUnsatisfiable: storageUnsatisfiableConstraintAction,
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					labelAppID:       unit.Labels[labelAppID],
					labelServiceType: unit.Labels[labelServiceType],
				},
			},
		})
	}

	setNodeAffinity(&unit.Spec.Template.Spec, expressions)

	// lijj32: add new node selector, the pod will prefer to select nodes with medium or high resource allocatable label.
	preferredExpressions := []corev1.NodeSelectorRequirement{
		{
			Key:      labelResourceAllocatable,
			Operator: corev1.NodeSelectorOpIn,
			Values: []string{
				hostv1.DefaultResourceMediumLabelValue,
				hostv1.DefaultResourceHighLabelValue,
			},
		},
	}
	setPreferredNodeAffinity(&unit.Spec.Template.Spec, preferredExpressions)

	setTopologySpreadConstraints(unit, constraints)

	return nil
}

func setTopologySpreadConstraints(unit *unitv4.Unit, constraints []corev1.TopologySpreadConstraint) {

	if len(unit.Spec.Template.Spec.TopologySpreadConstraints) == 0 {
		unit.Spec.Template.Spec.TopologySpreadConstraints = constraints
		return
	}

	for i := range constraints {
		unit.Spec.Template.Spec.TopologySpreadConstraints = append(unit.Spec.Template.Spec.TopologySpreadConstraints, constraints[i])
	}

}

func setSpecificAnnotation(unit *unitv4.Unit, options map[string]string) error {
	optionmap := map[string]string{}
	err := json.Unmarshal([]byte(unit.Annotations[unitv4.OptionsAnnotation]), &optionmap)
	if err != nil {
		return err
	}

	switch unit.Spec.MainContainerName {
	case structs.MysqlServiceType:
		optionmap["service_id"] = options["count"]
	default:
		return nil
	}

	optionstr, err := json.Marshal(optionmap)
	if err != nil {
		return err
	}

	unit.Annotations[unitv4.PodOptionsAnnotation] = string(optionstr)
	return nil
}

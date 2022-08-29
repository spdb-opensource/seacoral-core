package bankend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	stderror "github.com/pkg/errors"
	"github.com/upmio/dbscale-kube/pkg/structs"
	"github.com/upmio/dbscale-kube/pkg/vars"
	"io"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/api"
	"github.com/upmio/dbscale-kube/cluster_manager/apiserver/model"
	sanv1 "github.com/upmio/dbscale-kube/pkg/apis/san/v1alpha1"
	unitv4 "github.com/upmio/dbscale-kube/pkg/apis/unit/v1alpha4"
	podutil "github.com/upmio/dbscale-kube/pkg/utils/pod"
	"github.com/upmio/dbscale-kube/pkg/zone"
	"github.com/upmio/dbscale-kube/pkg/zone/site"
)

const DefaultPreferredSelectorWeight int32 = 50

type zoneIface struct {
	zone zone.ZoneInterface
}

func (z zoneIface) listSites() []zone.Site {
	return z.zone.ListSites()
}

//TODO, in 1.0 site is in unit.Spec but in 2.0 I cannot find it
func (z zoneIface) GetSite() string {
	return z.listSites()[0].Name()
}

func (z zoneIface) siteInterface(site string) (site.Interface, error) {
	return z.zone.SiteInterface(site)
}

func (z zoneIface) getUnit(site, namespace, name string) (*unitv4.Unit, error) {
	iface, err := z.zone.UnitInterface(site)
	if err != nil {
		return nil, err
	}

	return iface.Get(namespace, name)
}

func (z zoneIface) createUnit(unit *unitv4.Unit) (*unitv4.Unit, error) {
	iface, err := z.zone.UnitInterface(z.GetSite())
	if err != nil {
		return nil, err
	}

	return iface.Create(unit.Namespace, unit)
}

func (z zoneIface) updateUnit(unit *unitv4.Unit) error {

	iface, err := z.zone.UnitInterface(z.GetSite())
	if err != nil {
		return err
	}

	unit, err = iface.Update(unit.Namespace, unit)

	return err
}

func (z zoneIface) updateUnitAction(site, namespace, unitName string, state api.State) (bool, error) {

	iface, err := z.zone.SiteInterface(site)
	if err != nil {
		return false, err
	}

	unit, err := iface.Units().Get(namespace, unitName)
	if err != nil {
		return false, err
	}

	pod, err := iface.Pods().Get(unit.Namespace, unitv4.GetPodName(unit))
	if err != nil {
		return false, err
	}

	unit = unit.DeepCopy()

	if state == api.StatePassing {
		if podutil.IsRunningAndReady(pod) {
			return true, nil
		}
		unit.Spec.UnService = false
	} else {
		if !podutil.IsRunningAndReady(pod) {
			return true, nil
		}
		unit.Spec.UnService = true
	}

	unit, err = iface.Units().Update(unit.Namespace, unit)

	return false, err
}

func (z zoneIface) deleteUnit(unit unitv4.Unit) error {
	iface, err := z.zone.UnitInterface(z.GetSite())
	if err != nil {
		return err
	}

	return deleteUnit(iface, unit)
}

func deleteUnit(iface site.UnitInterface, unit unitv4.Unit) error {

	wt := NewWaitTask(time.Second*10, nil)

	return wt.WithTimeout(time.Minute, func() (bool, error) {

		unit, err := iface.Get(unit.Namespace, unit.Name)
		if errors.IsNotFound(err) {
			return true, nil
		}

		if unit.Spec.Action.Delete != nil {
			return false, nil
		}

		clone := unit.DeepCopy()

		clone.Spec.Action.Delete = &unitv4.DeleteAction{}

		clone, err = iface.Update(clone.Namespace, clone)
		if errors.IsNotFound(err) {
			return true, nil
		}

		return false, err
	})
}

func (z zoneIface) deletePod(site, namespace, name string) error {
	iface, err := z.zone.SiteInterface(site)
	if err != nil {
		return err
	}

	return iface.Pods().Delete(namespace, name, metav1.DeleteOptions{})
}

func (z zoneIface) getUnitIP(unit unitv4.Unit) (string, error) {
	if unit.Spec.Template.Spec.HostNetwork {
		return "", nil
	}

	iface, err := z.zone.SiteInterface(z.GetSite())
	klog.Infof("unit:%s getUnitIP: (site: %s) get siteInterface", unit.Name, z.GetSite())
	if err != nil {
		return "", err
	}

	/*clone, err := iface.Units().Get(unit.Namespace, unit.Name)
	if err != nil {
		return "", err
	}*/

	claim, err := iface.NetworkClaims().Get(unitv4.GetNetworkClaimName(&unit))
	if err != nil {
		return "", err
	}

	if claim.Status.BindIP == "" {
		return "", fmt.Errorf("Unit %s/%s networkclaim IP is not ready,current phase is %s",
			unit.Namespace, unit.Name, claim.Status.Status)
	}

	return claim.Status.BindIP, nil
}

func (z zoneIface) getPod(site, namespace, name string) (*corev1.Pod, error) {

	iface, err := z.zone.SiteInterface(site)
	if err != nil {
		return nil, err
	}

	return iface.Pods().Get(namespace, name)
}

func (z zoneIface) ensureUnitResourceEqual(requests api.ResourceRequirements, site, namespace, name string) (*unitv4.Unit, error) {
	iface, err := z.zone.SiteInterface(site)
	if err != nil {
		return nil, err
	}

	unit, err := iface.Units().Get(namespace, name)
	if err != nil {
		return nil, err
	}

	clone := unit.DeepCopy()

	equal, _, err := mergeUnitResources(clone, requests)
	if err != nil {
		return unit, err
	}

	if !equal {
		clone, err = iface.Units().Update(clone.Namespace, clone)
		if err != nil {
			return unit, err
		}
	}

	return clone, nil
}

func mergeUnitResources(clone *unitv4.Unit, requests api.ResourceRequirements) (equal, restart bool, err error) {
	equal, restart = true, false

	//if requests.Bandwidth != nil &&
	//	clone.Spec.Networking != nil &&
	//	*requests.Bandwidth != clone.Spec.Networking.Bandwidth {
	//
	//	equal = false
	//	clone.Spec.Networking.Bandwidth = *requests.Bandwidth
	//}

	if requests.Storage != nil {

		err := setHostVGRequestAnnotations(clone, requests.Storage)
		if err != nil {
			return equal, restart, err
		}

		for i, vc := range clone.Spec.VolumeClaims {
			for _, v := range requests.Storage.Volumes {

				if v.Type != vc.Name {
					continue
				}

				q, err := convertMiToQuantity(v.Capacity)
				if err != nil {
					return equal, restart, err
				}

				if q.Cmp(vc.Storage.Request) > 0 {
					equal = false
					clone.Spec.VolumeClaims[i].Storage.Request = q
				}
			}
		}
	}

	cpu, err := resource.ParseQuantity(fmt.Sprintf("%dm", int(requests.CPU)))
	if err != nil {
		return equal, restart, err
	}

	memory, err := convertMiToQuantity(requests.Memory)
	if err != nil {
		return equal, restart, err
	}

	spec, ok := isPodResourceEqual(clone.Spec.Template.Spec, clone.Spec.MainContainerName, cpu, memory)
	if !ok {
		equal = false
		clone.Spec.Template.Spec = spec
		restart = true
	}

	return equal, restart, nil
}

func (z *zoneIface) runInContainer(site, namespace, unitName string, cmd []string) (bool, io.Reader, error) {
	klog.Info("[runInContainer] ", cmd)
	iface, err := z.zone.SiteInterface(z.GetSite())
	if err != nil {
		return false, nil, err
	}

	unit, err := iface.Units().Get(namespace, unitName)
	if err != nil {
		return false, nil, err
	}

	return runInContainer(iface.PodExec(), *unit, cmd)
}

func runInContainer(execer site.PodExecInterface, unit unitv4.Unit, cmd []string) (bool, io.Reader, error) {
	podname := unitv4.GetPodName(&unit)

	stderr, stdout := bytes.NewBuffer(nil), bytes.NewBuffer(nil)

	ok, err := execer.RunInContainer(unit.Namespace, podname, unit.Spec.MainContainerName, cmd, stderr, stdout)

	klog.Infof("Pod %s/%s Container %s exec %s done:%t,error:%v,Output:%s,%s", unit.Namespace, podname, unit.Spec.MainContainerName, cmd, ok, err, stderr.String(), stdout.String())

	return ok, stdout, err
}

func getUnitReplication(execer site.PodExecInterface, unit unitv4.Unit) (api.Replication, error) {
	resp := api.Replication{}
	cmd := []string{"sh", shell, "replication", "show"}

	ok, r, err := runInContainer(execer, unit, cmd)
	if err != nil {
		return resp, err
	}
	if ok {
		err = decodeJson(r, &resp)
	} else {
		return resp, fmt.Errorf("%s/%s run in container %s failed", unit.Namespace, unit.Name, cmd)
	}

	return resp, err
}

func getUnitVolumesUsage(execer site.PodExecInterface, unit unitv4.Unit) ([]api.UnitVolumeUsage, error) {
	cmd := []string{"sh", shell, "volume", "show"}

	resp := []api.UnitVolumeUsage{}

	ok, r, err := runInContainer(execer, unit, cmd)
	if err != nil {
		return resp, err
	}
	if ok {
		err = decodeJson(r, &resp)
	} else {
		return resp, fmt.Errorf("%s/%s run in container %s failed", unit.Namespace, unit.Name, cmd)
	}

	return resp, err
}

func (z zoneIface) calculateUnitResourceDelta(requests api.ResourceRequirements,
	site, namespace, name string,
	hostsAndResourceRecord map[string]resourceRecord) (string, error) {

	iface, err := z.zone.SiteInterface(site)
	if err != nil {
		return "", err
	}

	unit, err := iface.Units().Get(namespace, name)
	if err != nil {
		return "", err
	}

	pod, err := iface.Pods().Get(unit.Namespace, unitv4.GetPodName(unit))
	if err != nil {
		return "", err
	}

	if !podutil.IsRunningAndReady(pod) {
		return "", stderror.Errorf("Unit %s is not running or ready", unit.Name)
	}

	hostNode, err := iface.Hosts().Get(pod.Spec.NodeName)
	if err != nil {
		return "", stderror.Errorf("Host %s is not found", pod.Status.HostIP)
	}
	hostName := hostNode.Name

	hostIface, err := z.zone.HostInterface(site)
	originalUnschedulable := hostNode.Spec.Unschedulable
	hostNode.Spec.Unschedulable = true
	hostNode, err = hostIface.Update(hostNode)

	if _, ok := hostsAndResourceRecord[hostName]; !ok {
		klog.Infof("for host %s, the unschedulable = %t", hostName, originalUnschedulable)
		cpuFree := hostNode.Status.Allocatable.Cpu.ScaledValue(resource.Milli)
		memFree := hostNode.Status.Allocatable.Memery.Value() >> 20
		hostsAndResourceRecord[hostName] = NewResourceRecord(site, hostName, cpuFree, memFree, originalUnschedulable)
		temp := hostsAndResourceRecord[hostName]
		storage := temp.storage

		for _, localVg := range hostNode.Status.Allocatable.LocalVGs {
			level := string(localVg.Level)
			if _, ok := storage[level]; !ok {
				storage[level] = struct {
					level    string
					diskReq  int64
					diskFree int64
				}{
					level:    level,
					diskReq:  0,
					diskFree: localVg.Size.Value() >> 20,
				}
			}
		}

		temp.storage = storage
		hostsAndResourceRecord[hostName] = temp
	}

	temp, _ := hostsAndResourceRecord[hostName]
	temp.pods = append(temp.pods, pod.Name)

	if requests.CPU > 0 {
		cpuRequest := requests.CPU
		cpuCurrent := pod.Spec.Containers[0].Resources.Requests.Cpu().ScaledValue(resource.Milli)
		temp.cpuReq += cpuRequest - cpuCurrent
	}

	if requests.Memory > 0 {
		memRequest := requests.Memory
		memCurrent := pod.Spec.Containers[0].Resources.Requests.Memory().Value() >> 20
		temp.memReq += memRequest - memCurrent
	}

	if requests.Storage != nil {
		var diskRequest int64
		var diskCurrent int64
		var level string
		storage := temp.storage

		for _, unitPvc := range unit.Spec.VolumeClaims {
			level = unitPvc.Storage.Level
			diskCurrent += unitPvc.Storage.Request.Value() >> 20
			foundInRequests := false

			for _, requestVol := range requests.Storage.Volumes {
				if unitPvc.Name == requestVol.Type {
					foundInRequests = true
					diskRequest += requestVol.Capacity
				}
			}

			if !foundInRequests {
				diskRequest += unitPvc.Storage.Request.Value() >> 20
			}
		}

		subStorage := storage[level]
		subStorage.diskReq += diskRequest - diskCurrent
		storage[level] = subStorage
		temp.storage = storage
	}

	hostsAndResourceRecord[hostName] = temp

	return hostName, nil
}

func (z zoneIface) setHostUnschedulable(site, hostName string, unschedulable bool) (string, error) {

	iface, err := z.zone.SiteInterface(site)
	if err != nil {
		return "", err
	}

	hostNode, err := iface.Hosts().Get(hostName)
	if err != nil {
		return "", stderror.Errorf("Host %s is not found", hostName)
	}

	hostIface, err := z.zone.HostInterface(site)
	if err != nil {
		return "", err
	}

	hostNode.Spec.Unschedulable = unschedulable
	hostNode, err = hostIface.Update(hostNode)
	if err != nil {
		return "", err
	}

	return "", nil
}

func isPodResourceEqual(spec corev1.PodSpec, container string, cpu, memory resource.Quantity) (corev1.PodSpec, bool) {
	equal := true
	clone := spec.DeepCopy()

	for i := range spec.Containers {
		if spec.Containers[i].Name != container {
			continue
		}

		q := spec.Containers[i].Resources.Requests.Cpu()
		if q == nil || cpu.Cmp(*q) != 0 {
			equal = false

			clone.Containers[i].Resources.Limits[corev1.ResourceCPU] = cpu
			clone.Containers[i].Resources.Requests[corev1.ResourceCPU] = cpu
		}

		q = spec.Containers[i].Resources.Requests.Memory()
		if q == nil || memory.Cmp(*q) != 0 {
			equal = false

			clone.Containers[i].Resources.Limits[corev1.ResourceMemory] = memory
			clone.Containers[i].Resources.Requests[corev1.ResourceMemory] = memory
		}
	}

	return *clone, equal
}

func setPodAntiAffinity(spec *corev1.PodSpec, group string) {
	term := corev1.PodAffinityTerm{
		TopologyKey: corev1.LabelHostname,
		LabelSelector: &metav1.LabelSelector{
			MatchExpressions: []metav1.LabelSelectorRequirement{
				{
					Key:      labelGroupName,
					Operator: metav1.LabelSelectorOpIn,
					Values:   []string{group},
				}, {
					Key:      labelGroupHostHA,
					Operator: metav1.LabelSelectorOpIn,
					Values:   []string{group},
				},
			},
		},
	}

	if spec.Affinity == nil {
		spec.Affinity = &corev1.Affinity{
			PodAntiAffinity: &corev1.PodAntiAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
					term,
				},
			},
		}

		return
	}
	if spec.Affinity.PodAntiAffinity == nil {
		spec.Affinity.PodAntiAffinity = &corev1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
				term,
			},
		}
		return
	}
	if spec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
		spec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution = []corev1.PodAffinityTerm{
			term,
		}

		return
	}

	terms := spec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution

	for i := range terms {
		if terms[i].TopologyKey == term.TopologyKey {
			if terms[i].LabelSelector == nil {
				terms[i].LabelSelector = term.LabelSelector
			} else {
				terms[i].LabelSelector.MatchExpressions = append(terms[i].LabelSelector.MatchExpressions, term.LabelSelector.MatchExpressions...)
			}
		}
	}

	terms = append(terms, term)

	spec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution = terms
}

func setNodeAffinity(spec *corev1.PodSpec, matches []corev1.NodeSelectorRequirement) {

	if spec.Affinity == nil {
		spec.Affinity = &corev1.Affinity{
			NodeAffinity: &corev1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: matches,
						},
					},
				},
			},
		}

		return
	}

	if spec.Affinity.NodeAffinity == nil {
		spec.Affinity.NodeAffinity = &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{
						MatchExpressions: matches,
					},
				},
			},
		}
		return
	}

	if spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
		spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = &corev1.NodeSelector{
			NodeSelectorTerms: []corev1.NodeSelectorTerm{
				{
					MatchExpressions: matches,
				},
			},
		}

		return
	}

	if len(spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms) == 0 {
		spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms = []corev1.NodeSelectorTerm{
			{
				MatchExpressions: matches,
			},
		}

		return
	}

	for i, term := range spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms {
		if term.MatchExpressions == nil {
			term.MatchExpressions = matches
		} else {
			term.MatchExpressions = append(term.MatchExpressions, matches...)
		}

		spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[i] = term
	}
}

// lijj32: setPreferredNodeAffinity sets preferred node affinity to the pod spec
func setPreferredNodeAffinity(spec *corev1.PodSpec, matches []corev1.NodeSelectorRequirement) {

	if spec.Affinity == nil {
		spec.Affinity = &corev1.Affinity{
			NodeAffinity: &corev1.NodeAffinity{
				PreferredDuringSchedulingIgnoredDuringExecution: []corev1.PreferredSchedulingTerm{
					{
						Weight: DefaultPreferredSelectorWeight,
						Preference: corev1.NodeSelectorTerm{
							MatchExpressions: matches,
						},
					},
				},
			},
		}

		return
	}

	if spec.Affinity.NodeAffinity == nil {
		spec.Affinity.NodeAffinity = &corev1.NodeAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.PreferredSchedulingTerm{
				{
					Weight: DefaultPreferredSelectorWeight,
					Preference: corev1.NodeSelectorTerm{
						MatchExpressions: matches,
					},
				},
			},
		}

		return
	}

	if spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution == nil {
		spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution = []corev1.PreferredSchedulingTerm{
			{
				Weight: DefaultPreferredSelectorWeight,
				Preference: corev1.NodeSelectorTerm{
					MatchExpressions: matches,
				},
			},
		}

		return
	}

	for i, term := range spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
		if term.Preference.MatchExpressions == nil {
			term.Preference.MatchExpressions = matches
		} else {
			term.Preference.MatchExpressions = append(term.Preference.MatchExpressions, matches...)
		}

		spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution[i] = term
	}
}

func convertMiToQuantity(val int64) (resource.Quantity, error) {
	str := fmt.Sprintf("%dMi", val)
	return resource.ParseQuantity(str)
}

func convertQuantityToMi(q resource.Quantity) int64 {
	v := q.Value()

	return v >> 20
}

func convertGroupSpecToUnit(appId, appName, groupname, grouptype, servicetype string,
	spec api.ServiceSpec,
	image model.Image,
	registry, projectName, networkMode string) (unitv4.Unit, error) {
	ports := make([]corev1.ContainerPort, len(spec.Ports))

	for i := range spec.Ports {
		ports[i] = corev1.ContainerPort{
			Name:          spec.Ports[i].Name,
			ContainerPort: spec.Ports[i].Port,
		}
	}

	cpu, err := resource.ParseQuantity(fmt.Sprintf("%dm", int(spec.Units.Resources.Requests.CPU)))
	if err != nil {
		return unitv4.Unit{}, err
	}

	memory, err := convertMiToQuantity(spec.Units.Resources.Requests.Memory)
	if err != nil {
		return unitv4.Unit{}, err
	}

	podTemplate, err := image.ConvertToPodTemplate()
	if err != nil {
		return unitv4.Unit{}, err
	}

	unitImage := fmt.Sprintf("%s/%s/%s", registry, projectName, image.ImageWithArch())
	defaultCmd := []string{"/bin/bash", "-c", "trap : TERM INT; sleep infinity & wait"}
	unit := unitv4.Unit{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   metav1.NamespaceDefault,
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
		},
		Spec: unitv4.UnitSpec{
			MainContainerName: image.Type,
			MainImageVerison:  image.ImageVersion.VersionWithArch(),
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ImagePullSecrets: []corev1.LocalObjectReference{
						corev1.LocalObjectReference{
							Name: structs.DefaultImagePullSecret,
						}},
					Containers: []corev1.Container{
						{
							Name:    image.Type,
							Image:   unitImage,
							Ports:   ports,
							Command: defaultCmd,
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    cpu,
									corev1.ResourceMemory: memory,
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    cpu,
									corev1.ResourceMemory: memory,
								},
							},
							ReadinessProbe: &podTemplate.ReadinessProbe,
							LivenessProbe:  &podTemplate.LivenessProbe,
						},
					},
				},
			},
		},
	}

	Bandwidth := int32(0)
	if spec.Units.Resources.Requests.Bandwidth != nil {
		Bandwidth = *spec.Units.Resources.Requests.Bandwidth
	}

	unit.Spec.Networking = unitv4.NetworkingRequest{
		Mode:      unitv4.NetworkModeType(networkMode),
		Type:      unitv4.NetworkInternal,
		Bandwidth: Bandwidth,
	}

	if spec.Units.Resources.Requests.Storage != nil {

		claims := make([]unitv4.PVCRequest, len(spec.Units.Resources.Requests.Storage.Volumes))

		for i, req := range spec.Units.Resources.Requests.Storage.Volumes {

			storageType := sanv1.LocalType
			allocType := ""
			if spec.Units.Resources.Requests.Storage.Type == api.StorageTypeRemote {
				storageType = sanv1.RemoteType
				allocType = sanv1.AllocThick
			}

			q, err := convertMiToQuantity(req.Capacity)
			if err != nil {
				return unit, err
			}

			claims[i] = unitv4.PVCRequest{
				Name:             req.Type,
				StorageClassName: sanv1.VolumePath,
				Storage: unitv4.Storage{
					Type:      storageType,
					Level:     string(spec.Units.Resources.Requests.Storage.Performance),
					AllocType: allocType,
					Request:   q,
				},
				FsType:      sanv1.DefaultFsType,
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany},
			}
		}

		unit.Spec.VolumeClaims = claims
	}

	unit.Labels[labelAppID] = appId
	unit.Labels[unitv4.LabelGroup] = appName
	unit.Labels[labelServiceName] = appId
	unit.Labels[labelServiceImageType] = servicetype
	unit.Labels[labelGroupName] = groupname
	unit.Labels[labelServiceType] = servicetype
	unit.Labels[labelGroupType] = grouptype
	unit.Labels[labelAppIDAndGroup] = fmt.Sprintf("%s-%s", appId, servicetype)
	unit.Labels[vars.LabelDBScaleKey] = vars.LabelDBScaleValue

	err = setHostVGRequestAnnotations(&unit, spec.Units.Resources.Requests.Storage)
	if err != nil {
		return unit, err
	}

	if err := setCommonEnvForContainer(&unit, spec, image); err != nil {
		return unit, err
	}

	if err := setCommonOptionsAnnotation(&unit, spec, image); err != nil {
		return unit, err
	}

	// TODO
	//if spec.Units.ReadinessState == api.StatePassing {
	//	unit.Spec.Action.ServiceRunning = unitv4.NewActionStrategy(unitv4.ServiceInitStart)
	//} else {
	//	unit.Spec.Action.ServiceRunning = unitv4.NewActionStrategy(unitv4.ServiceStop)
	//}

	return unit, nil
}

func setHostVGRequestAnnotations(unit *unitv4.Unit, requests *api.StorageRequirement) error {
	if requests == nil || requests.Type != api.StorageTypeHost {
		return nil
	}

	capacity := int64(0)

	for _, v := range requests.Volumes {
		capacity += v.Capacity
	}

	q, err := convertMiToQuantity(capacity)
	if err != nil {
		return err
	}

	unit.Annotations[annotationVGRequest] = fmt.Sprintf(`{"%s":"%s"}`, requests.Performance, q.String())

	return nil
}

func setCommonOptionsAnnotation(unit *unitv4.Unit, spec api.ServiceSpec, image model.Image) error {
	archstr, err := json.Marshal(spec.Arch)
	if err != nil {
		return err
	}

	if spec.Options == nil {
		spec.Options = make(map[string]string)
	}

	spec.Options[structs.OptionArch] = string(archstr)

	// switch image.BackupType {
	// case structs.MysqlServiceType:
	// default:
	// }

	optionstr, err := json.Marshal(spec.Options)
	if err != nil {
		return err
	}

	unit.Annotations[unitv4.PodOptionsAnnotation] = string(optionstr)

	return nil
}

func setCommonEnvForContainer(unit *unitv4.Unit, spec api.ServiceSpec, image model.Image) error {
	commonenvs := []corev1.EnvVar{
		corev1.EnvVar{
			Name: "INTERNAL_IP",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "status.podIP",
				},
			},
		},
		{
			Name:  "SERVICE_GROUP_TYPE",
			Value: unit.Labels[labelGroupType],
		},
	}

	for i := range commonenvs {
		setContainerEnv(unit, commonenvs[i])
	}

	return nil
}

func setContainerEnv(unit *unitv4.Unit, env corev1.EnvVar) {
	for i := range unit.Spec.Template.Spec.Containers {
		unit.Spec.Template.Spec.Containers[i].Env = append(unit.Spec.Template.Spec.Containers[i].Env, env)
	}
}

func getJobLogs(kubeClient kubernetes.Interface, job *batchv1.Job) (string, error) {
	pods, err := getPodsForJob(kubeClient, job)
	if err != nil {
		return "", err
	}
	if len(pods) == 0 {
		return "", fmt.Errorf("job %s:not find pods", job.Name)
	}

	return getPodLogs(kubeClient, pods[0])
}

func getPodLogs(kubeClient kubernetes.Interface, pod corev1.Pod) (string, error) {
	podLogOpts := corev1.PodLogOptions{}

	req := kubeClient.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &podLogOpts)
	podLogs, err := req.Stream(context.TODO())
	if err != nil {
		return "", fmt.Errorf(":%s: error in opening stream", pod.Name)
	}
	defer podLogs.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return "", fmt.Errorf(":%s: error in copy information from podLogs to buf", pod.Name)
	}
	str := buf.String()
	return str, nil
}

func getPodsForJob(kubeClient kubernetes.Interface, job *batchv1.Job) ([]corev1.Pod, error) {
	if job == nil {
		return nil, nil
	}

	var err error

	if job.GetUID() == "" {
		job, err = kubeClient.BatchV1().Jobs(job.Namespace).Get(context.TODO(), job.Name, metav1.GetOptions{})
	}
	if errors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	r, err := labels.NewRequirement("controller-uid", selection.Equals, []string{string(job.GetUID())})
	if err != nil {
		return nil, err
	}

	selector := labels.NewSelector().Add(*r)

	pods, err := kubeClient.CoreV1().Pods(job.Namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: selector.String(),
	})

	return pods.Items, err
}

func getPodsForJobV2(client kubernetes.Interface, job *batchv1.Job) ([]corev1.Pod, error) {
	if job == nil {
		return nil, nil
	}

	var err error

	if job.GetUID() == "" {
		job, err = client.BatchV1().Jobs(job.Namespace).Get(context.TODO(), job.Name, metav1.GetOptions{})
	}
	if errors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	r, err := labels.NewRequirement("controller-uid", selection.Equals, []string{string(job.GetUID())})
	if err != nil {
		return nil, err
	}

	selector := labels.NewSelector().Add(*r)

	list, err := client.CoreV1().Pods(job.Namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	if err != nil {
		return nil, err
	}

	return list.Items, err
}

func getPodsForJobFromSiteInterface(iface site.Interface, job *batchv1.Job) ([]corev1.Pod, error) {
	if job == nil {
		return nil, nil
	}

	var err error

	if job.GetUID() == "" {
		job, err = iface.Jobs().Get(job.Namespace, job.Name)
	}
	if errors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	r, err := labels.NewRequirement("controller-uid", selection.Equals, []string{string(job.GetUID())})
	if err != nil {
		return nil, err
	}

	selector := labels.NewSelector().Add(*r)

	pods, err := iface.Pods().List(job.Namespace, metav1.ListOptions{
		LabelSelector: selector.String(),
	})

	return pods, err
}

func deleteJob(iface site.Interface, job *batchv1.Job) error {

	pods, err := getPodsForJobFromSiteInterface(iface, job)
	if err != nil {
		return err
	}

	for _, pod := range pods {

		err := iface.Pods().Delete(pod.Namespace, pod.Name, metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return err
		}

		klog.Info("Delete pod ", pod.Name, err)
	}

	var errs []error

	for _, pod := range pods {

		for _, v := range pod.Spec.Volumes {
			if v.PersistentVolumeClaim == nil || v.PersistentVolumeClaim.ClaimName == "" {
				continue
			}

			pvc, err := iface.PersistentVolumeClaims().Get(pod.Namespace, v.PersistentVolumeClaim.ClaimName)
			if errors.IsNotFound(err) {
				continue
			}
			if err != nil {
				errs = append(errs, err)
				continue
			}

			err = iface.PersistentVolumeClaims().Delete(pvc.Namespace, pvc.Name, metav1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				errs = append(errs, err)
			}

			if pvc.Spec.VolumeName == "" {
				continue
			}

			err = iface.PersistentVolumes().Delete(pvc.Spec.VolumeName, metav1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				errs = append(errs, err)
			}
		}
	}

	for _, v := range job.Spec.Template.Spec.Volumes {
		if v.PersistentVolumeClaim == nil || v.PersistentVolumeClaim.ClaimName == "" {
			continue
		}

		pvc, err := iface.PersistentVolumeClaims().Get(job.Namespace, v.PersistentVolumeClaim.ClaimName)
		if errors.IsNotFound(err) {
			continue
		}
		if err != nil {
			errs = append(errs, err)
			continue
		}

		err = iface.PersistentVolumeClaims().Delete(pvc.Namespace, pvc.Name, metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			errs = append(errs, err)
		}

		if pvc.Spec.VolumeName == "" {
			continue
		}

		err = iface.PersistentVolumes().Delete(pvc.Spec.VolumeName, metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			errs = append(errs, err)
		}
	}

	err = iface.Jobs().Delete(job.Namespace, job.Name, metav1.DeleteOptions{
		GracePeriodSeconds: &gracePeriodSeconds,
		PropagationPolicy:  &propagationPolicy, // grace delete include pods
	})
	if err != nil && !errors.IsNotFound(err) {
		errs = append(errs, err)
	}

	klog.Info("Delete job ", job.Name, err)

	return utilerrors.NewAggregate(errs)
}

/*
func deleteLungroup(iface site.LungroupInterface, ns, name string) (bool, error) {
	lun, err := iface.Get(name)
	if errors.IsNotFound(err) {
		return true, nil
	}
	if err != nil {
		return false, err
	}

	if !lun.Spec.DeleteProcess {

		lun = lun.DeepCopy()
		lun.Spec.DeleteProcess = true

		lun, err = iface.Update(lun)
		if errors.IsNotFound(err) {
			return true, nil
		}
	}

	return false, err
}
*/

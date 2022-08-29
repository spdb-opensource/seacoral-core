package v1alpha4

import (
	"context"
	"encoding/json"
	"fmt"
	hostv1 "github.com/upmio/dbscale-kube/pkg/apis/host/v1alpha1"
	"github.com/upmio/dbscale-kube/pkg/utils"
	"github.com/upmio/dbscale-kube/pkg/vars"
	"k8s.io/apimachinery/pkg/labels"
	"path/filepath"

	"time"

	"github.com/upmio/dbscale-kube/pkg/structs"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes/scheme"
	ref "k8s.io/client-go/tools/reference"

	"k8s.io/klog/v2"

	netv1 "github.com/upmio/dbscale-kube/pkg/apis/networking/v1alpha1"
	sanv1 "github.com/upmio/dbscale-kube/pkg/apis/san/v1alpha1"
	unitv4 "github.com/upmio/dbscale-kube/pkg/apis/unit/v1alpha4"
	vpv1 "github.com/upmio/dbscale-kube/pkg/apis/volumepath/v1alpha1"

	parser "github.com/upmio/dbscale-kube/pkg/parser"

	podutil "github.com/upmio/dbscale-kube/pkg/utils/pod"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

// unitKind contains the schema.GroupVersionKind for this controller type.
var unitKind = unitv4.SchemeGroupVersion.WithKind("Unit")

const (
	calicoAnnotation  = "cni.projectcalico.org/ipAddrs"
	appLabel          = "app"
	labelCluster      = "dbscale.cluster"
	hostPathType      = corev1.HostPathDirectory
	hostPathDirPrefix = "/volumepath/k8s/"
)

func (ctrl *Controller) ensureCustomConfig(unit *unitv4.Unit) error {
	// ref := metav1.NewControllerRef(unit, unitKind)
	_, err := ctrl.configMapLister.ConfigMaps(unit.GetNamespace()).Get(unitv4.GetUnitConfigName(unit))
	if errors.IsNotFound(err) {
		template, err := ctrl.configMapLister.ConfigMaps(corev1.NamespaceDefault).Get(unitv4.GetTemplateConfigName(unit))
		if err != nil {
			return err
		}

		// path, ok := template.Data[unitv4.ConfigFilePathTab]
		// if !ok {
		// 	return unit, fmt.Errorf(":%s(key:%s)not find config filepath", template.GetName(), unitv4.ConfigFilePathTab)
		// }
		prclient := parser.ParserClient{
			KubeClient: ctrl.kubeClient,
			NetClient:  ctrl.netClient,
		}
		pr, err := parser.NewParser(context.TODO(), prclient, unit, template)
		if err != nil {
			return err
		}

		data, err := pr.GenerateConfig()
		if err != nil {
			return err
		}
		config := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      unitv4.GetUnitConfigName(unit),
				Namespace: unit.GetNamespace(),
				// OwnerReferences: []metav1.OwnerReference{*ref},
			},

			Data: map[string]string{
				unitv4.ConfigDataTab: data,
			},
		}

		_, err = ctrl.kubeClient.CoreV1().ConfigMaps(unit.GetNamespace()).Create(context.TODO(), config, metav1.CreateOptions{})
		if err == nil {
			ctrl.recorder.Eventf(unit, corev1.EventTypeNormal, SuccessCreated, "create  config %s for pod %s", unitv4.GetUnitConfigName(unit), unit.PodName())
		}
		return err
	}

	return nil
}

func (ctrl *Controller) ensureConfigSecret(unit *unitv4.Unit) error {
	// ref := metav1.NewControllerRef(unit, unitKind)

	script, err := ctrl.configMapLister.ConfigMaps(unit.GetNamespace()).Get(unitv4.GetUnitScriptConfigName(unit))
	if errors.IsNotFound(err) {

		templatescript, err := ctrl.configMapLister.ConfigMaps(corev1.NamespaceDefault).Get(unitv4.GetScriptTemplateName(unit))
		if err != nil {
			return err
		}

		config := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					vars.LabelDBScaleKey: vars.LabelDBScaleValue,
				},

				Annotations: map[string]string{
					unitv4.ScriptSyncResourceVersionTab: templatescript.GetResourceVersion(),
				},

				Name:      unitv4.GetUnitScriptConfigName(unit),
				Namespace: unit.GetNamespace(),
				// OwnerReferences: []metav1.OwnerReference{*ref},
			},

			Data: map[string]string{
				unitv4.ScriptDataTab: templatescript.Data[unitv4.ScriptDataTab],
				unitv4.ScriptSyncTab: templatescript.GetName(),
			},
		}

		script, err = ctrl.kubeClient.CoreV1().ConfigMaps(unit.GetNamespace()).Create(context.TODO(), config, metav1.CreateOptions{})
		if err == nil {
			ctrl.recorder.Eventf(unit, corev1.EventTypeNormal, SuccessCreated, "create script %s for pod %s", unitv4.GetUnitScriptConfigName(unit), unit.PodName())
		}
		return err
	}

	if err != nil {
		return err
	}

	//sync configs
	syncFrom, ok := script.Data[unitv4.ScriptSyncTab]
	if !ok {
		ctrl.recorder.Eventf(unit, corev1.EventTypeWarning, "sync-scripts", "%s not find synctab(%s)", script.GetName(), unitv4.ScriptSyncTab)
		return nil
		// return fmt.Errorf("%s not find synctab(%s)", sctript.GetName(), unitv4.ScriptSyncTab)
	}
	templatescript, err := ctrl.configMapLister.ConfigMaps(corev1.NamespaceDefault).Get(unitv4.GetScriptTemplateName(unit))
	if err != nil {
		ctrl.recorder.Eventf(unit, corev1.EventTypeWarning, "sync-scripts", "%s not find template configmap(%s)", templatescript)
		return err
	}

	generation := script.Annotations[unitv4.ScriptSyncResourceVersionTab]
	change := false

	if generation != templatescript.GetResourceVersion() || syncFrom != unitv4.GetScriptTemplateName(unit) {
		script.Annotations[unitv4.ScriptSyncResourceVersionTab] = templatescript.GetResourceVersion()
		script.Data[unitv4.ScriptDataTab] = templatescript.Data[unitv4.ScriptDataTab]
		change = true
	}

	if change {
		_, err = ctrl.kubeClient.CoreV1().ConfigMaps(script.Namespace).Update(context.TODO(), script, metav1.UpdateOptions{})
		if err != nil {
			ctrl.recorder.Eventf(unit, corev1.EventTypeWarning, "sync-scripts", "sync-scripts fail from  %s(resource_verison:%s):%s", templatescript.GetName(), templatescript.GetResourceVersion(), err.Error())
			return err
		}
		ctrl.recorder.Eventf(unit, corev1.EventTypeNormal, "sync-scripts", "sync-scripts ok from %s(resource_verison:%s)", templatescript.GetName(), templatescript.GetResourceVersion())

	}

	return nil
}

func (ctrl *Controller) getLastestUnit(namespace, name string) (*unitv4.Unit, error) {
	return ctrl.clientset.UnitV1alpha4().Units(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}

func (ctrl *Controller) ensureNetworks(unit *unitv4.Unit) error {

	claim, err := ctrl.networkClaimLister.Get(unitv4.GetNetworkClaimName(unit))
	if errors.IsNotFound(err) {

		network, err := ctrl.networkLister.Get(unit.Spec.Networking.Network)
		if err != nil {
			return err
		}

		typ := netv1.NetworkClaimLabelInternal
		networkClaim := &netv1.NetworkClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name: unitv4.GetNetworkClaimName(unit),
			},
			Spec: netv1.NetworkClaimSpec{
				Mode:      network.Spec.Mode,
				Type:      typ,
				Bandwidth: unit.Spec.Networking.Bandwidth,
				Network:   network.GetName(),
			},
		}
		// ref := metav1.NewControllerRef(unit, unitKind)
		// setOwnerReference(networkClaim, ref)

		claim, err := ctrl.netClient.NetworkingV1alpha1().NetworkClaims().Create(context.TODO(), networkClaim, metav1.CreateOptions{})
		if err == nil {
			ctrl.recorder.Eventf(unit, corev1.EventTypeNormal, SuccessCreated, "create networkclaim %s for pod %s", claim.Name, unit.PodName())
		}

		return err

	}

	if err != nil {
		return err
	}

	switch claim.Spec.Mode {
	case netv1.CalicoNetworkMode:
		return nil
	case netv1.MacVlanNetworkMode, netv1.SriovNetworkMode:
		if claim.Spec.Bandwidth != unit.Spec.Networking.Bandwidth {
			old := claim.Spec.Bandwidth
			claim.Spec.Bandwidth = unit.Spec.Networking.Bandwidth
			claim, err = ctrl.netClient.NetworkingV1alpha1().NetworkClaims().Update(context.TODO(), claim, metav1.UpdateOptions{})
			if err != nil {
				return err
			}

			ctrl.recorder.Eventf(unit, corev1.EventTypeNormal, SuccessUpdated, "update networkclaim %s bandwidth,%d->%d", claim.Name, old, claim.Spec.Bandwidth)
		}
		return err
	default:
		return fmt.Errorf("unit not support the network mode(%s)", claim.Spec.Mode)
	}
	return nil
}

//get pathfile from template
func (ctrl *Controller) getConfigPath(unit *unitv4.Unit) (string, error) {
	template, err := ctrl.configMapLister.ConfigMaps(corev1.NamespaceDefault).Get(unitv4.GetTemplateConfigName(unit))
	if err != nil {
		return "", err
	}
	path, ok := template.Data[unitv4.ConfigFilePathTab]
	if !ok {
		return "", fmt.Errorf(":%s(key:%s)not find config filepath", template.GetName(), unitv4.ConfigFilePathTab)
	}

	return path, nil

}

func convertUnitToPod(unit *unitv4.Unit, configpath string) *corev1.Pod {
	// ref := metav1.NewControllerRef(unit, unitKind)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        unitv4.GetPodName(unit),
			Namespace:   unit.GetNamespace(),
			Labels:      unit.GetLabels(),
			Annotations: unit.GetAnnotations(),
			// OwnerReferences: []metav1.OwnerReference{*ref},
		},
		Spec: unit.Spec.Template.Spec,
	}

	//add networkClaim(now just support internal)
	pod.Annotations[netv1.NetworkClaimLabelInternal] = unitv4.GetNetworkClaimName(unit)

	//add local vg request
	localvgs, need := getLocalVGRequest(unit)
	if need {
		pod.Annotations[unitv4.VGRequestAnnotation] = localvgs
	}

	//add volumes
	volumes := []corev1.Volume{}
	envs := []corev1.EnvVar{}
	vloumemounts := []corev1.VolumeMount{}
	for _, req := range unit.Spec.VolumeClaims {
		volume := corev1.Volume{Name: req.Name}
		volume.PersistentVolumeClaim = &corev1.PersistentVolumeClaimVolumeSource{
			ClaimName: unitv4.GetPersistentVolumeClaimName(unit, req.Name),
		}

		if req.Mounter != "" {
			vloumemounts = append(vloumemounts, corev1.VolumeMount{Name: req.Name, MountPath: req.Mounter})
		} else {
			switch req.Name {
			case "log":
				vloumemounts = append(vloumemounts, corev1.VolumeMount{Name: req.Name, MountPath: "/DBAASLOG"})
				envs = append(envs, corev1.EnvVar{
					Name:  "LOG_MOUNT",
					Value: "/DBAASLOG",
				})

			case "data":
				vloumemounts = append(vloumemounts, corev1.VolumeMount{Name: req.Name, MountPath: "/DBAASDAT"})
				envs = append(envs, corev1.EnvVar{
					Name:  "DATA_MOUNT",
					Value: "/DBAASDAT",
				})
			default:
				vloumemounts = append(vloumemounts, corev1.VolumeMount{Name: req.Name, MountPath: filepath.Join("/", req.Name)})
			}
		}

		volumes = append(volumes, volume)
	}

	//script configmap
	mode := int32(0755)
	scriptvolume := corev1.Volume{Name: "script"}
	scriptvolume.ConfigMap = &corev1.ConfigMapVolumeSource{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: unit.ScriptName(),
		},
		Items: []corev1.KeyToPath{
			corev1.KeyToPath{
				Key:  "scripts",
				Path: "unitMGR",
				Mode: &mode,
			},
		},
	}
	volumes = append(volumes, scriptvolume)

	scriptmounter := corev1.VolumeMount{Name: "script", MountPath: "/opt/app-root/scripts"}
	vloumemounts = append(vloumemounts, scriptmounter)

	//custom configmaps
	custommode := int32(0644)
	customConfigVolume := corev1.Volume{Name: "custom-config"}
	customConfigVolume.ConfigMap = &corev1.ConfigMapVolumeSource{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: unitv4.GetUnitConfigName(unit),
		},
		Items: []corev1.KeyToPath{
			corev1.KeyToPath{
				Key:  unitv4.ConfigDataTab,
				Path: filepath.Base(configpath),
				Mode: &custommode,
			},
		},
	}
	volumes = append(volumes, customConfigVolume)
	configmounter := corev1.VolumeMount{Name: "custom-config", MountPath: filepath.Dir(configpath)}
	vloumemounts = append(vloumemounts, configmounter)

	pod.Spec.Volumes = volumes
	for i := range pod.Spec.Containers {
		pod.Spec.Containers[i].VolumeMounts = vloumemounts

	}

	//ENV
	cfgpath := corev1.EnvVar{
		Name:  "CONFIG_PATH",
		Value: configpath,
	}
	envs = append(envs, cfgpath)

	for i := range pod.Spec.Containers {
		for _, env := range envs {
			pod.Spec.Containers[i].Env = append(pod.Spec.Containers[i].Env, env)
		}

	}

	return pod
}

//目前 不同步image,尽管k8s支持，还是重建pod来完成.
func generatePatchPod(unit *unitv4.Unit, curPod *corev1.Pod, netclaim *netv1.NetworkClaim) *corev1.Pod {
	clone := curPod.DeepCopy()

	for key, value := range unit.Labels {
		if clone.Labels == nil {
			clone.Labels = make(map[string]string)
		}

		if clone.Labels[key] != value {
			clone.Labels[key] = value
		}
	}

	for key, value := range unit.Annotations {
		if clone.Annotations == nil {
			clone.Annotations = make(map[string]string)
		}
		if clone.Annotations[key] != value {
			clone.Annotations[key] = value
		}
	}

	if unit.Spec.Template.Spec.NodeName == "" && curPod.Spec.NodeName != "" {
		clone.Spec.NodeName = curPod.Spec.NodeName
	} else if unit.Spec.Template.Spec.NodeName != "" && unit.Spec.Template.Spec.NodeName != curPod.Spec.NodeName {
		clone.Spec.NodeName = unit.Spec.Template.Spec.NodeName
	}

	localvgs, need := getLocalVGRequest(unit)
	if need {
		clone.Annotations[unitv4.VGRequestAnnotation] = localvgs
	}

	if unit.Spec.Networking.Mode == unitv4.CalicoNetworkMode {
		calicoIPs := []string{"0.0.0.0"}
		if netclaim != nil && netclaim.Spec.Mode == netv1.CalicoNetworkMode && netclaim.Status.BindIP != "" {
			calicoIPs = []string{netclaim.Status.BindIP}
		}

		ipstr, err := json.Marshal(calicoIPs)
		if err != nil {
			klog.Errorf("Marshal  %v  fail:%s", calicoIPs, err.Error())
		}

		clone.Annotations[calicoAnnotation] = string(ipstr)
	}

	return clone
}

func (ctrl *Controller) upgratePod(unit *unitv4.Unit) error {

	ctrl.recorder.Event(unit, corev1.EventTypeNormal, "upgrate", "trigger : stop service -> delete pod -> create pod")

	configpath, err := ctrl.getConfigFilePath(unit)
	if err != nil {
		return err
	}

	//stop service
	tmpuint := unit.DeepCopy()
	tmpuint.Spec.UnService = true
	err = ctrl.ensureUnitServer(tmpuint)
	if err != nil && !errors.IsNotFound(err) {
		ctrl.recorder.Eventf(unit, corev1.EventTypeWarning, "upgrate", "ignore: stop server fail%s", err.Error())
		// return err
	}

	//delete pod
	err = ctrl.kubeClient.CoreV1().Pods(unit.GetNamespace()).Delete(context.TODO(), unitv4.GetPodName(unit), metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	//wait delete
	err = wait.PollImmediate(retryInterval, retryTimeout, func() (bool, error) {
		_, err = ctrl.kubeClient.CoreV1().Pods(unit.Namespace).Get(context.TODO(), unitv4.GetPodName(unit), metav1.GetOptions{})
		if err != nil && errors.IsNotFound(err) {
			return true, nil
		}
		return false, nil
	})

	if err != nil {
		return fmt.Errorf("upgratePod: wait pod delete fail:%s", err.Error())
	}

	//create
	pod := convertUnitToPod(unit, configpath)
	_, err = ctrl.kubeClient.CoreV1().Pods(unit.Namespace).Create(context.TODO(), pod, metav1.CreateOptions{})
	if err == nil {
		ctrl.recorder.Eventf(unit, corev1.EventTypeNormal, SuccessCreated, "recreate pod %s ok", pod.Name)
	}

	return err

}

func (ctrl *Controller) ensurePod(unit *unitv4.Unit) error {
	pod, err := ctrl.podLister.Pods(unit.Namespace).Get(unitv4.GetPodName(unit))
	if errors.IsNotFound(err) {
		cfgpath, err := ctrl.getConfigPath(unit)
		if err != nil {
			return err
		}
		pod := convertUnitToPod(unit, cfgpath)
		_, err = ctrl.kubeClient.CoreV1().Pods(unit.Namespace).Create(context.TODO(), pod, metav1.CreateOptions{})
		if err == nil {
			ctrl.recorder.Eventf(unit, corev1.EventTypeNormal, SuccessCreated, "create pod %s ok", pod.Name)
		}
		return err
	}
	if err != nil {
		return err
	}

	networkclaim, err := ctrl.networkClaimLister.Get(unitv4.GetNetworkClaimName(unit))
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	//update mem,cpu,image trigger rereate pod
	if isNeedUpgratePod(unit, pod) {
		klog.V(2).Infof("%s:upgrate pod", unit.GetName())
		return ctrl.upgratePod(unit)
	}

	//sync label ,not image there
	patch, need, err := isNeedPatchPod(unit, pod, networkclaim)
	if need {
		_, err = ctrl.kubeClient.CoreV1().Pods(pod.Namespace).Patch(context.TODO(), pod.Name, types.StrategicMergePatchType, patch, metav1.PatchOptions{})
		if err == nil {
			ctrl.recorder.Eventf(unit, corev1.EventTypeNormal, SuccessCreated, "patch pod %s ok   (patch data: %s)", pod.Name, string(patch))
		} else {
			ctrl.recorder.Eventf(unit, corev1.EventTypeWarning, SuccessCreated, "Patch pod %s fail:%s (patch data: %s)", pod.Name, err.Error(), string(patch))
		}
	}

	if err != nil {
		ctrl.recorder.Eventf(unit, corev1.EventTypeWarning, "patch", "check patch pod fail:%s", err.Error())
	}

	return nil

}

//主container
func isNeedUpgratePod(unit *unitv4.Unit, pod *corev1.Pod) bool {
	for _, templatecontainer := range unit.Spec.Template.Spec.Containers {
		for _, podcontainer := range pod.Spec.Containers {

			if templatecontainer.Name == podcontainer.Name &&
				podcontainer.Name == unit.Spec.MainContainerName {
				//maincontaier image
				if templatecontainer.Name == unit.Spec.MainContainerName &&
					templatecontainer.Image != podcontainer.Image {
					return true
				}

				//cpu,mem
				if templatecontainer.Resources.Requests.Cpu().MilliValue() != podcontainer.Resources.Requests.Cpu().MilliValue() ||
					templatecontainer.Resources.Limits.Cpu().MilliValue() != podcontainer.Resources.Limits.Cpu().MilliValue() ||
					templatecontainer.Resources.Requests.Memory().Value() != podcontainer.Resources.Requests.Memory().Value() ||
					templatecontainer.Resources.Limits.Memory().Value() != podcontainer.Resources.Limits.Memory().Value() {
					return true
				}

			}
		}
	}

	return false
}

func isNeedPatchPod(unit *unitv4.Unit, pod *corev1.Pod, netclaim *netv1.NetworkClaim) ([]byte, bool, error) {
	updatepod := generatePatchPod(unit, pod, netclaim)
	modJson, err := json.Marshal(updatepod)
	if err != nil {
		return []byte{}, false, err
	}

	curJson, err := json.Marshal(pod)
	if err != nil {
		return []byte{}, false, err
	}

	patch, err := strategicpatch.CreateTwoWayMergePatch(curJson, modJson, corev1.Pod{})
	if err != nil {
		return []byte{}, false, err
	}

	if len(patch) == 0 || string(patch) == "{}" {
		return patch, false, nil
	}
	return patch, true, nil
}

func (ctrl *Controller) getNetworkByNodeName(nodename string) (string, error) {
	if nodename == "" {
		return "", fmt.Errorf("getNetworkByNodeName failed: nodename is empty")
	}
	node, err := ctrl.kubeClient.CoreV1().Nodes().Get(context.TODO(), nodename, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	cluster, ok := node.Labels[labelCluster]
	if !ok {
		return "", fmt.Errorf("not find %s lable from node", labelCluster)
	}
	sets := labels.Set{labelCluster: cluster}
	networks, err := ctrl.networkLister.List(labels.SelectorFromSet(sets))
	if err != nil {
		return "", err
	}

	if len(networks) == 0 {
		return "", fmt.Errorf("not find network by %s=%s ", labelCluster, cluster)
	}
	maxcunt := 0
	selectnetwork := ""
	for i := range networks {
		if networks[i].Annotations[netv1.NetworkPhaseAnnotation] == netv1.NetworkmaintenancePhase {
			continue
		}

		freecnt := int(networks[i].Status.AllIPCounts - networks[i].Status.UsedIPCount)
		if freecnt > maxcunt {
			selectnetwork = networks[i].Name
			maxcunt = freecnt
		}
	}

	if selectnetwork == "" {
		return "", fmt.Errorf("not find suitable network in cluster:%s (netwroks:%v)(maybe not enougth ip)", cluster, networks)
	}
	return selectnetwork, nil
}

func (ctrl *Controller) syncUnitHandler(unit *unitv4.Unit) (err error) {

	defer func() {
		klog.V(4).Infof("%s:finish syncunit", unit.Name)
	}()

	klog.V(4).Infof("%s:start syncunit", unit.Name)

	if unit.Spec.Action.Delete != nil || unit.GetDeletionTimestamp() != nil {
		return ctrl.removeUnit(unit)
	}

	if unit.Spec.Action.Migrate != nil {
		return ctrl.migrateUnit(unit)
	}

	if unit.Spec.Action.Rebuild != nil {
		return ctrl.rebuildUnit(unit)
	}

	err = unit.Valid()
	if err != nil {
		return err
	}

	err = ctrl.ensureConfigSecret(unit)
	if err != nil {
		return err
	}

	//_, _, err = ctrl.ensureStorageHandler(unit, "")
	//if err != nil {
	//	return err
	//}

	err = ctrl.ensurePersistentVolumeClaims(unit)
	if err != nil {
		return err
	}

	err = ctrl.ensurePod(unit)
	if err != nil {
		return err
	}

	pod, err := waitUntilPodScheduled(ctrl.kubeClient, unit.Name, unit.Namespace)
	if err != nil {
		return err
	}

	if unit.Spec.Template.Spec.NodeName == "" {
		updateunit := unit.DeepCopy()
		updateunit.Spec.Template.Spec.NodeName = pod.Spec.NodeName
		unit, err = ctrl.patchUnit(unit, updateunit)
		if err != nil {
			return err
		}
	}

	if unit.Spec.Networking.Network == "" {
		network, err := ctrl.getNetworkByNodeName(unit.Spec.Template.Spec.NodeName)
		if err != nil {
			return err
		}

		updateunit := unit.DeepCopy()
		updateunit.Spec.Networking.Network = network
		unit, err = ctrl.patchUnit(unit, updateunit)
		if err != nil {
			return err
		}
	}

	err = ctrl.ensureNetworks(unit)
	if err != nil {
		return err
	}

	err = ctrl.waitUntilNetowrkClaimsReady(unit)
	if err != nil {
		return err
	}

	//re ensurePod
	err = ctrl.ensurePod(unit)
	if err != nil {
		return err
	}

	_, _, err = ctrl.ensureStorageHandler(unit, pod.Spec.NodeName)
	if err != nil {
		return err
	}

	err = ctrl.ensureCustomConfig(unit)
	if err != nil {
		return err
	}

	unit, err = ctrl.ensureInitStart(unit)
	if err != nil {
		return err
	}

	err = ctrl.ensureUnitServer(unit)
	if err != nil {
		return err
	}

	err = ctrl.ensureRebuildStatus(unit)
	if err != nil {
		return err
	}
	// unit, err = ctrl.syncPodPhase(unit)
	// if err != nil {
	// 	ctrl.recorder.Eventf(unit, corev1.EventTypeWarning, "sync", "sync pod phase err:%s", err.Error())
	// }

	return nil
}

func (ctrl *Controller) ensureRebuildStatus(unit *unitv4.Unit) error {
	if unit.Status.RebuildStatus == nil ||
		unit.Status.RebuildStatus.CurVolumeSuffix == unit.Status.RebuildStatus.RetainVolumeSuffix {
		return nil
	}

	if unit.Status.RebuildStatus.CurVolumeSuffix != unit.Status.RebuildStatus.RetainVolumeSuffix+1 {
		return fmt.Errorf("bad RebuildStatus:%v", unit.Status.RebuildStatus)
	}

	if err := ctrl.triggerRetainVolumeDeleted(unit); err != nil {
		return err
	}
	RebuildStatus := unitv4.RebuildVolumeStatus{
		CurVolumeSuffix:    unit.Status.RebuildStatus.CurVolumeSuffix,
		RetainVolumeSuffix: unit.Status.RebuildStatus.CurVolumeSuffix,
	}
	unit.Status.RebuildStatus = &RebuildStatus
	_, err := ctrl.clientset.UnitV1alpha4().Units(unit.Namespace).UpdateStatus(context.TODO(), unit, metav1.UpdateOptions{})
	return err
}

func convertToPVC(unit *unitv4.Unit, pvcRequset unitv4.PVCRequest) *corev1.PersistentVolumeClaim {
	// ref := metav1.NewControllerRef(unit, unitKind)
	claim := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: unit.Namespace,
			Name:      unitv4.GetPersistentVolumeClaimName(unit, pvcRequset.Name),
			// OwnerReferences: []metav1.OwnerReference{*ref},
			Annotations: map[string]string{
				unitv4.VolumePathAnnotation: unitv4.GetPersistentVolumeClaimName(unit, pvcRequset.Name),
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			VolumeName:  unitv4.GetPersistentVolumeName(unit, pvcRequset.Name),
			AccessModes: pvcRequset.AccessModes,
			Resources: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceStorage: pvcRequset.Storage.Request,
				},
			},
		},
	}

	return claim
}

func (ctrl *Controller) getLocalVgName(node string, level string) (string, error) {
	knode, err := ctrl.kubeClient.CoreV1().Nodes().Get(context.TODO(), node, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	vgs, ok := knode.Annotations[hostv1.VGsAnnotation]
	if !ok {
		return "", fmt.Errorf("not find %s Annotations", hostv1.VGsAnnotation)
	}

	vgmap := map[string]string{}
	if err := json.Unmarshal([]byte(vgs), &vgmap); err != nil {
		return "", err
	}

	for vgname, vglevel := range vgmap {
		if vglevel == level {
			return vgname, nil
		}
	}

	return "", fmt.Errorf("not find vg name by %s level from %s hostv1.VGsAnnotation %s : %s", level, node, hostv1.VGsAnnotation, vgs)
}

//TODO: 合理应该修改pvc就好了，迁移扩容在CSI,目前放这里管理简单方便，且迁移控制放在unit管理比较简单方便，后期变化概率也低
// 向CSI看齐的方案的话，应该修改PVC就好了，storage controller做下面的动作,但是迁移不好兼容。
func (ctrl *Controller) ensureStorageHandler(unit *unitv4.Unit, node string) (luns []sanv1.Lungroup, vps []vpv1.VolumePath, err error) {
	luns = make([]sanv1.Lungroup, 0, len(unit.Spec.VolumeClaims))
	vps = make([]vpv1.VolumePath, 0, len(unit.Spec.VolumeClaims))

	for _, claim := range unit.Spec.VolumeClaims {
		if claim.StorageClassName != sanv1.VolumePath {
			continue
		}
		//sync to pvc lable

		name := unitv4.GetLunGroupName(unit, claim.Name)

		switch claim.Storage.Type {
		case sanv1.RemoteType:
			lg, err := ctrl.lunClient.SanV1alpha1().Lungroups().Get(context.TODO(), name, metav1.GetOptions{})
			if err != nil && !errors.IsNotFound(err) {
				return luns, vps, err
			}

			if errors.IsNotFound(err) {
				lg = &sanv1.Lungroup{
					ObjectMeta: metav1.ObjectMeta{
						Name:   name,
						Labels: map[string]string{appLabel: unit.Name},
					},
					Spec: sanv1.LungroupSpec{
						Type:     claim.Storage.AllocType,
						Hostname: node,
						PVName:   name,
						VGName:   name,
						FsType:   claim.FsType,
						Capacity: claim.Storage.Request,
						Level:    sanv1.Level(claim.Storage.Level),
					},
				}
				if lg.Labels == nil {
					lg.Labels = make(map[string]string)
				}
				lg.Labels[structs.LabelGroup] = unit.GetName()

				lg, err = ctrl.lunClient.SanV1alpha1().Lungroups().Create(context.TODO(), lg, metav1.CreateOptions{})
				if err != nil {
					return luns, vps, err
				}

				ctrl.recorder.Eventf(unit, corev1.EventTypeNormal, SuccessCreated, "create lungroup %s", lg.Name)
			}

			//扩容
			if claim.Storage.Request.Cmp(lg.Spec.Capacity) > 0 && (node != "" && node == lg.Spec.Hostname) {

				lg = lg.DeepCopy()

				old := lg.Spec.Capacity
				lg.Spec.Capacity = claim.Storage.Request

				lg, err = ctrl.lunClient.SanV1alpha1().Lungroups().Update(context.TODO(), lg, metav1.UpdateOptions{})
				if err != nil {
					return luns, vps, err
				}

				ctrl.recorder.Eventf(unit, corev1.EventTypeNormal, SuccessUpdated, "expand lungroup %s for pod %s,%s->%s", lg.Name, unit.PodName(), old.String(), lg.Spec.Capacity.String())
			}

			luns = append(luns, *lg.DeepCopy())

		case sanv1.LocalType:

			vp, err := ctrl.lvmClient.LvmV1alpha1().VolumePaths().Get(context.TODO(), name, metav1.GetOptions{})
			if err != nil && !errors.IsNotFound(err) {
				return luns, vps, err
			}

			//size := claim.Storage.Request
			//// MB
			//size = size >> 20

			if errors.IsNotFound(err) {
				vgname, err := ctrl.getLocalVgName(node, claim.Storage.Level)
				if err != nil {
					return luns, vps, err
				}

				vp = &vpv1.VolumePath{
					ObjectMeta: metav1.ObjectMeta{
						Name: name,
						Labels: map[string]string{
							sanv1.PVCSourcePerformanceLabel: claim.Storage.Level,
						},
						// OwnerReferences: []metav1.OwnerReference{ref},
					},
					Spec: vpv1.VolumePathSpec{
						VgName: vgname,
						Type:   sanv1.LocalSource,
						FsType: claim.FsType,
						Size:   claim.Storage.Request,
						Vendor: sanv1.LocalSource,
						Node:   node,
					},
				}

				if vp.Labels == nil {
					vp.Labels = make(map[string]string)
				}
				vp.Labels[structs.LabelGroup] = unit.GetName()

				vp, err = ctrl.lvmClient.LvmV1alpha1().VolumePaths().Create(context.TODO(), vp, metav1.CreateOptions{})
				if err != nil {
					return luns, vps, err
				}

				ctrl.recorder.Eventf(unit, corev1.EventTypeNormal, SuccessCreated, "create volumepath %s for pod %s", vp.Name, unit.PodName())
			}

			if claim.Storage.Request.Cmp(vp.Spec.Size) > 0 && (node != "" && node == vp.Spec.Node) {

				vp = vp.DeepCopy()
				old := vp.Spec.Size
				vp.Spec.Size = claim.Storage.Request

				vp, err = ctrl.lvmClient.LvmV1alpha1().VolumePaths().Update(context.TODO(), vp, metav1.UpdateOptions{})
				if err != nil {
					return luns, vps, err
				}

				ctrl.recorder.Eventf(unit, corev1.EventTypeNormal, SuccessUpdated, "expand volumepath %s for pod %s,%s->%s", vp.Name, unit.PodName(), old.String(), vp.Spec.Size.String())
			}

			vps = append(vps, *vp.DeepCopy())
		}
	}

	return
}

func (ctrl *Controller) ensurePersistentVolumeClaims(unit *unitv4.Unit) error {

	for _, claimReq := range unit.Spec.VolumeClaims {
		pvcName := unitv4.GetPersistentVolumeClaimName(unit, claimReq.Name)
		claim, err := ctrl.pvcLister.PersistentVolumeClaims(unit.GetNamespace()).Get(pvcName)
		if errors.IsNotFound(err) {
			claim = convertToPVC(unit, claimReq)
			_, err = ctrl.kubeClient.CoreV1().PersistentVolumeClaims(claim.Namespace).Create(context.TODO(), claim, metav1.CreateOptions{})
			if err == nil {
				ctrl.recorder.Eventf(unit, corev1.EventTypeNormal, SuccessCreated, "create pvc %s", pvcName)
			}
		}

		if err != nil && !errors.IsNotFound(err) {
			return err
		}

		claimRef, err := ref.GetReference(scheme.Scheme, claim)
		if err != nil {
			return err
		}

		pvname := unitv4.GetPersistentVolumeName(unit, claimReq.Name)
		_, err = ctrl.pvLister.Get(pvname)
		if errors.IsNotFound(err) {

			/*		typ := hostPathType
					accessModes := claim.Spec.AccessModes
					if len(accessModes) == 0 {
						accessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}
					}
					_, err = ctrl.kubeClient.CoreV1().PersistentVolumes().Create(context.TODO(), &corev1.PersistentVolume{
						ObjectMeta: metav1.ObjectMeta{
							Name: pvname,
						},
						Spec: corev1.PersistentVolumeSpec{
							ClaimRef:    claimRef,
							Capacity:    claim.Spec.Resources.Requests,
							VolumeMode:  claim.Spec.VolumeMode,
							AccessModes: accessModes,
							PersistentVolumeSource: corev1.PersistentVolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: filepath.Join(hostPathDirPrefix, claim.Spec.VolumeName),
									Type: &typ,
								},
							},
						},
							}, metav1.CreateOptions{})*/

			accessModes := claim.Spec.AccessModes
			if len(accessModes) == 0 {
				accessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}
			}
			_, err = ctrl.kubeClient.CoreV1().PersistentVolumes().Create(context.TODO(), &corev1.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name: pvname,
				},
				Spec: corev1.PersistentVolumeSpec{
					ClaimRef:    claimRef,
					Capacity:    claim.Spec.Resources.Requests,
					VolumeMode:  claim.Spec.VolumeMode,
					AccessModes: accessModes,
					PersistentVolumeSource: corev1.PersistentVolumeSource{
						CSI: &corev1.CSIPersistentVolumeSource{
							Driver:       vpv1.VPCSIDriverName,
							VolumeHandle: pvname,
						},
					},
				},
			}, metav1.CreateOptions{})
			if err == nil {
				ctrl.recorder.Eventf(unit, corev1.EventTypeNormal, SuccessCreated, "create pv %s", pvcName)
			}
		}

	}

	return nil
}

func (ctrl *Controller) ensureService(unit *unitv4.Unit) (*corev1.Service, error) {
	name := unitv4.GetServiceName(unit)
	// ref := metav1.NewControllerRef(unit, unitKind)
	service, err := ctrl.serviceLister.Services(unit.Namespace).Get(name)
	if errors.IsNotFound(err) {
		service = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: unit.Namespace,
				Name:      name,
				Labels: map[string]string{
					appLabel: unit.Name,
				},
				// OwnerReferences: []metav1.OwnerReference{*ref},
			},
			Spec: corev1.ServiceSpec{
				ClusterIP: corev1.ClusterIPNone,
				Selector: map[string]string{
					appLabel: unit.Name,
				},
			},
		}

		service, err = ctrl.kubeClient.CoreV1().Services(unit.Namespace).Create(context.TODO(), service, metav1.CreateOptions{})
		if err == nil {
			ctrl.recorder.Eventf(unit, corev1.EventTypeNormal, SuccessCreated, "create service %s", service.Name)
		}
	}

	return service.DeepCopy(), err
}

const (
	retryInterval = 1 * time.Second
	retryTimeout  = 3 * time.Second
)

func waitForPodRunning(kubeClient kubernetes.Interface, podName, namespace string) (*corev1.Pod, error) {
	var pod *corev1.Pod
	var err error
	err = wait.PollImmediate(retryInterval, retryTimeout, func() (bool, error) {
		pod, err = kubeClient.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}

		if !podutil.IsRunning(pod) {
			return false, nil
		}

		return true, nil
	})

	if err != nil {
		err = fmt.Errorf("waitForPodRunning  %s fail:%s", podName, err.Error())
	}

	return pod, err
}

func (ctrl *Controller) getConfigFilePath(unit *unitv4.Unit) (string, error) {
	tempalte, err := ctrl.configMapLister.ConfigMaps(unit.GetNamespace()).Get(unitv4.GetTemplateConfigName(unit))
	if err != nil {
		return "", err
	}
	path, ok := tempalte.Data[unitv4.ConfigFilePathTab]
	if !ok {
		return "", fmt.Errorf("%s :not find %s tab", unitv4.GetTemplateConfigName(unit), unitv4.ConfigFilePathTab)
	}

	return path, nil

}

func waitUntilPodScheduled(kubeClient kubernetes.Interface, podName, namespace string) (*corev1.Pod, error) {

	var pod *corev1.Pod
	var err error
	err = wait.PollImmediate(retryInterval, retryTimeout, func() (bool, error) {
		pod, err = kubeClient.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}

		if pod.Spec.NodeName == "" || !podutil.IsCreated(pod) {
			return false, nil
		}

		return true, nil
	})

	if err != nil {
		err = fmt.Errorf("waitUntilPodScheduled %s fail: %s", podName, err.Error())
	}

	return pod, err
}

func (ctrl *Controller) waitUntilNetowrkClaimsReady(unit *unitv4.Unit) error {
	pod, err := ctrl.podLister.Pods(unit.Namespace).Get(unitv4.GetPodName(unit))
	if err != nil {
		return err
	}

	interncKey, ok := pod.Annotations[netv1.NetworkClaimLabelInternal]
	if !ok {
		return fmt.Errorf("pod not find Annotations %s", netv1.NetworkClaimLabelInternal)
	}

	err = wait.PollImmediate(retryInterval, retryTimeout, func() (bool, error) {
		claim, err := ctrl.netClient.NetworkingV1alpha1().NetworkClaims().Get(context.TODO(), interncKey, metav1.GetOptions{})
		if err != nil && errors.IsNotFound(err) {
			return false, nil
		}

		if err != nil {
			return false, err
		}

		if claim.Status.BindIP == "" || claim.Status.Used == "" {
			return false, nil
		}

		podkey := fmt.Sprintf("%s/%s", unit.GetNamespace(), unit.GetName())
		if claim.Status.Used != podkey {
			return false, fmt.Errorf("not used by the %s pod(cur:%s)", podkey, claim.Status.Used)
		}

		return true, nil
	})

	if err != nil {
		err = fmt.Errorf("waitUntilNetowrkClaimsReady %s fail:%s", unit.GetName(), err.Error())
	}

	return err
}

/*
func setOwnerReference(obj metav1.Object, ref *metav1.OwnerReference) {
	if ref == nil {
		return
	}

	owners := obj.GetOwnerReferences()

	for _, oref := range owners {
		if oref.Kind == ref.Kind && oref.UID == ref.UID && oref.Name == ref.Name {
			return
		}
	}

	if owners == nil {
		owners = make([]metav1.OwnerReference, 0, 1)
	}

	owners = append(owners, *ref)

	obj.SetOwnerReferences(owners)
}
*/
func (ctrl *Controller) ensureInitStart(unit *unitv4.Unit) (*unitv4.Unit, error) {

	initconditon := unitv4.GetContionStatus(unit, unitv4.InitStartCondition)
	if initconditon == unitv4.ConditionTrue {
		return unit, nil
	}

	klog.V(4).Infof("%s unit:start InitStart ", unit.GetName())

	_, err := waitForPodRunning(ctrl.kubeClient, unitv4.GetPodName(unit), unit.Namespace)
	if err != nil {
		return unit, err
	}

	//initcmd := getCommandByType(ServiceInitStart)
	initcmd, err := structs.GetExecCmd(structs.CommonType, structs.ServiceInitStartCmd)
	_, err = ctrl.UnitExec(unit, unit.Spec.MainContainerName, initcmd)
	if err != nil {
		return unit, err
	}

	updateunit := unit.DeepCopy()
	unitv4.SetContionStatus(updateunit, unitv4.Condition{
		Type:   unitv4.InitStartCondition,
		Status: unitv4.ConditionTrue,
	})

	updateunit, err = ctrl.clientset.UnitV1alpha4().Units(unit.Namespace).UpdateStatus(context.TODO(), updateunit, metav1.UpdateOptions{})
	if err != nil {
		return unit, err
	}

	ctrl.recorder.Eventf(unit, corev1.EventTypeNormal, "init start", "init start ok")

	klog.V(4).Infof("%s unit: InitStart ok", unit.GetName())
	return updateunit, nil
}

func (ctrl *Controller) patchUnit(old, _new *unitv4.Unit) (*unitv4.Unit, error) {
	patch, update, err := utils.GenerateMergePatch(old, _new, unitv4.Unit{})
	if err != nil || !update {
		return old, err
	}
	ctrl.recorder.Eventf(old, corev1.EventTypeNormal, "patch", "data:%s", patch)
	return ctrl.clientset.UnitV1alpha4().Units(old.Namespace).Patch(context.TODO(), old.Name, types.MergePatchType, patch, metav1.PatchOptions{})
}

func getLocalVGRequest(unit *unitv4.Unit) (string, bool) {
	ret := map[string]resource.Quantity{}
	for _, clam := range unit.Spec.VolumeClaims {
		if clam.Storage.Type != unitv4.StorageLocalType {
			return "", false
		}

		cur, ok := ret[clam.Storage.Level]
		if !ok {
			ret[clam.Storage.Level] = clam.Storage.Request
			continue
		}

		cur.Add(clam.Storage.Request)
		ret[clam.Storage.Level] = cur
	}

	tmp := map[string]string{}
	for k, v := range ret {
		tmp[k] = v.String()
	}

	retjson, err := json.Marshal(tmp)
	if err != nil {
		klog.Errorf("%s getVGRequest fail:%s", unit.GetName(), err.Error())
		return "", false
	}
	return string(retjson), true
}

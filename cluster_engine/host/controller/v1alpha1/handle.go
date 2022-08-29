package v1alpha1

import (
	"context"
	"encoding/json"
	"fmt"

	"k8s.io/klog/v2"
	"strconv"
	"strings"

	"time"

	hostv1 "github.com/upmio/dbscale-kube/pkg/apis/host/v1alpha1"
	"github.com/upmio/dbscale-kube/pkg/utils"
	"github.com/upmio/dbscale-kube/pkg/vars"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"

	// "k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	log "k8s.io/klog/v2"
)

func (c *Controller) hostManagerHandler(key string) error {

	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	host, err := c.hostLister.Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	host = host.DeepCopy()
	defer func() {
		klog.V(4).Infof("%s:finish synchost", key)
	}()

	klog.V(4).Infof("host %s:start synchost spec:%v,status:%v", key, host.Spec, host.Status)
	if host.Spec.ActCode == hostv1.DeletCode {
		return c.unInstallHandle(host)
	}

	//failed do nothing
	if host.Status.Phase == hostv1.HostDeployFailed {
		return nil
	}

	if host.Status.Phase != hostv1.HostReady {
		return c.installHandle(host)
	}

	node, err := c.nodeLister.Get(name)
	if err != nil {
		return err
	}

	err = c.syncHostToNode(host, node)
	if err != nil {
		c.recorder.Event(host, corev1.EventTypeWarning, "syncHostToNode fail", err.Error())
		log.V(2).Infof("syncHostToNode fail:%s", err.Error())
	}

	uphost, err := c.syncHostStatus(host, node)
	if err != nil {
		c.recorder.Event(host, corev1.EventTypeWarning, "syncHostStatus fail", err.Error())
		return err
	}

	err = c.syncSchedulerLabel(uphost, node)
	if err != nil {
		c.recorder.Event(host, corev1.EventTypeWarning, "syncSchedulerLabel fail", err.Error())
		return err
	}

	return nil
}

func (c *Controller) syncSchedulerLabel(host *hostv1.Host, node *corev1.Node) error {
	limitstr, ok := node.Annotations[hostv1.NodeUsageLimtAnnotation]
	if !ok {
		return fmt.Errorf("not find %s label", hostv1.NodeUsageLimtAnnotation)
	}

	limit := hostv1.UsageLimit{}
	err := json.Unmarshal([]byte(limitstr), &limit)
	if err != nil {
		return err
	}

	triggerLimit := false
	triggerlist := []string{}
	if int(host.Status.Capacity.Units.Value()-host.Status.Allocatable.Units.Value()) >= limit.Units {
		triggerLimit = true
		triggerlist = append(triggerlist, "units")
	}

	if host.Status.Capacity.Memery.Value()-host.Status.Allocatable.Memery.Value() >=
		host.Status.Capacity.Memery.Value()*int64(limit.Memory)/100 {
		triggerLimit = true
		triggerlist = append(triggerlist, "memery")
	}

	if host.Status.Capacity.Cpu.Value()-host.Status.Allocatable.Cpu.Value() >=
		host.Status.Capacity.Cpu.Value()*int64(limit.CPU)/100 {
		triggerLimit = true
		triggerlist = append(triggerlist, "cpu")
	}

	for i, _ := range host.Status.Capacity.LocalVGs {
		if host.Status.Capacity.LocalVGs[i].Size.Value()-host.Status.Allocatable.LocalVGs[i].Size.Value() >=
			host.Status.Capacity.LocalVGs[i].Size.Value()*int64(limit.Storage)/100 {
			triggerLimit = true
			triggerlist = append(triggerlist, "localstorage")
		}
	}

	updateNode := node.DeepCopy()
	triggervalue := strconv.FormatBool(triggerLimit)
	if triggerLimit {
		triggervalue = strings.Join(triggerlist, "-")
	}

	updateNode.Labels[vars.LabelResourceLimit] = triggervalue
	_, err = c.patchNode(node, updateNode)
	return err
}

func (c *Controller) syncHostToNode(host *hostv1.Host, node *corev1.Node) error {
	updatenode := node.DeepCopy()
	for key, value := range host.Labels {
		updatenode.Labels[key] = value
	}

	for key, value := range host.Annotations {
		updatenode.Annotations[key] = value
	}

	//add vg info to node
	vgs := map[string]string{}
	for _, vg := range host.Spec.LocalVGs {
		vgs[vg.Name] = string(vg.Level)
	}
	vgstr, err := json.Marshal(vgs)
	if err != nil {
		return err
	}
	updatenode.Annotations[hostv1.VGsAnnotation] = string(vgstr)
	updatenode.Annotations[hostv1.HostNetworkMode] = host.Spec.NetworkMode
	updatenode.Spec.Unschedulable = host.Spec.Unschedulable

	_, err = c.patchNode(node, updatenode)
	return err
}

func (c *Controller) patchHost(old, _new *hostv1.Host) (*hostv1.Host, error) {
	patch, update, err := utils.GenerateMergePatch(old, _new, hostv1.Host{})
	if err != nil || !update {
		return old, err
	}
	c.recorder.Eventf(old, corev1.EventTypeNormal, "patch", "data:%s", patch)
	return c.hostClientset.HostV1alpha1().Hosts().Patch(context.TODO(), old.Name, types.MergePatchType, patch, metav1.PatchOptions{})
}

func (c *Controller) patchNode(old, _new *corev1.Node) (*corev1.Node, error) {
	patch, update, err := utils.GenerateMergePatch(old, _new, corev1.Node{})
	if err != nil || !update {
		return old, err
	}
	c.recorder.Eventf(old, corev1.EventTypeNormal, "patch", "data:%s", patch)
	return c.kubeclientset.CoreV1().Nodes().Patch(context.TODO(), old.Name, types.MergePatchType, patch, metav1.PatchOptions{})
}

func (c *Controller) syncSanInitiator(host *hostv1.Host, node *corev1.Node) error {
	if host.Spec.San != nil {
		values, ok := node.Annotations[hostv1.SanInitiatorAnnotation]
		if !ok {
			return fmt.Errorf("not find the  %s key from node Annotation", hostv1.SanInitiatorAnnotation)
		}

		initiator := hostv1.Initiator{}
		err := json.Unmarshal([]byte(values), &initiator)
		if err != nil {
			return fmt.Errorf("%s node Unmarshal fail:%s(datas:%s)", node.GetName(), hostv1.SanInitiatorAnnotation, values)
		}

		if initiator.Type != host.Spec.San.Initiator.Type {
			return fmt.Errorf("initiator.Type not match node:%s ;host:%s", initiator.Type, host.Spec.San.Initiator.Type)
		}

		chage := false
		if len(host.Spec.San.Initiator.ID) != len(initiator.ID) {
			chage = true
		}
		for i := range host.Spec.San.Initiator.ID {
			if initiator.ID[i] != host.Spec.San.Initiator.ID[i] {
				chage = true
				break
			}
		}

		if chage {
			if err := c.updateSanInitiator(host, initiator); err != nil {
				c.recorder.Event(host, corev1.EventTypeWarning, "updateSanInitiator fail", err.Error())
				log.V(2).Infof("%s Host updateSanInitiator fail:%s", node.GetName(), err.Error())
				return err
			}
		}

	}

	return nil
}
func (c *Controller) updateVgsAnnotation(node *corev1.Node, vgs string) error {
	updatenode := node.DeepCopy()
	updatenode.Annotations[hostv1.VGsAnnotation] = vgs

	// _, err := c.kubeclientset.CoreV1().Nodes().Update(updatenode)
	_, err := c.patchNode(node, updatenode)
	return err
}

func (c *Controller) installHandle(host *hostv1.Host) error {
	if needHostInstall(host) {
		log.V(2).Infof("%s:start install node", host.GetName())
		if host.Status.Phase == "" {
			err := c.UpdatePhaseStatus(host, hostv1.HostInstalling)
			if err != nil {
				c.recorder.Event(host, corev1.EventTypeWarning, "UpdatePhaseStatus fail:%s", err.Error())
			} else {
				c.recorder.Event(host, corev1.EventTypeNormal, "start installing", "start installing")
				log.V(2).Infof("%s: start installing node", host.GetName())

			}

			return err
		}

		cfg, err := generateHostCfg(host)
		if err != nil {
			c.recorder.Event(host, corev1.EventTypeWarning, "generateHostCfg fail", err.Error())

			return fmt.Errorf("generateHostCfg  fail %s", err.Error())
		}

		err = deployNode(*cfg)
		if err != nil {
			_err := c.UpdatePhaseStatus(host, hostv1.HostDeployFailed)
			if _err != nil {
				c.recorder.Eventf(host, corev1.EventTypeWarning, "UpdatePhaseStatus %s  fail:%s", string(hostv1.HostDeployFailed), err.Error())
			}
			c.recorder.Event(host, corev1.EventTypeWarning, "deployNode fail", err.Error())
			return err
		}

		if err := c.syncInitiatorTypeToNode(host); err != nil {
			c.recorder.Event(host, corev1.EventTypeWarning, "syncInitiatorTypeToNode fail", err.Error())
			log.V(2).Infof("%s: syncInitiatorTypeToNode err:%s", host.GetName(), err.Error())
			return err
		}

		if host.Spec.San != nil {
			if err := c.waitForSyncSanInitiator(host); err != nil {
				c.recorder.Event(host, corev1.EventTypeWarning, "waitForSyncSanInitiator fail", err.Error())
				log.V(2).Infof("%s: waitForSyncSanInitiator err:%s", host.GetName(), err.Error())
				return err
			}
		}

		err = c.UpdatePhaseStatus(host, hostv1.HostInstalled)
		if err != nil {
			c.recorder.Event(host, corev1.EventTypeWarning, "UpdatePhaseStatus fail", err.Error())
			return err
		}

		c.recorder.Event(host, corev1.EventTypeNormal, "node install sucess", "node install sucess")
		log.V(2).Infof("%s: install node sucess", host.GetName())
		return nil
	}

	if NeedSanRegister(host) {
		log.V(4).Infof("%s:waiting for sancontroller to do register.", host.GetName())
		c.recorder.Event(host, corev1.EventTypeNormal, "register host to san", "waiting  3s for sancontroller to do register")
		time.Sleep(3 * time.Second)
		return nil
	}

	if host.Status.Phase != hostv1.HostReady {
		err := c.UpdatePhaseStatus(host, hostv1.HostReady)
		if err != nil {
			c.recorder.Event(host, corev1.EventTypeWarning, "UpdatePhaseStatus", err.Error())
			return err
		}
	}

	c.recorder.Event(host, corev1.EventTypeNormal, "node ready", "node ready")
	return nil
}

func (c *Controller) syncInitiatorTypeToNode(host *hostv1.Host) error {
	time.Sleep(2 * time.Second)
	node, err := c.nodeLister.Get(host.GetName())
	if err != nil {
		return err
	}

	_, ok := node.Annotations[hostv1.SanInitiatorTypeAnnotation]
	if ok {
		return nil
	}

	_type := "local"
	if host.Spec.San != nil {
		_type = host.Spec.San.Initiator.Type
	}

	updatenode := node.DeepCopy()
	updatenode.Annotations[hostv1.SanInitiatorTypeAnnotation] = _type
	// _, err = c.kubeclientset.CoreV1().Nodes().Update(updatenode)
	_, err = c.patchNode(node, updatenode)
	return err
}

func (c *Controller) waitForSyncSanInitiator(host *hostv1.Host) error {

	time.Sleep(2 * time.Second)
	node, err := c.nodeLister.Get(host.GetName())
	if err != nil {
		return err
	}

	err = c.syncSanInitiator(host, node)
	if err != nil {
		return err
	}
	return nil
}

func (c *Controller) unInstallHandle(host *hostv1.Host) error {
	if NeedSanUnregister(host) {
		log.V(4).Infof("%s:waiting for sancontroller to do unregister", host.GetName())
		c.recorder.Event(host, corev1.EventTypeNormal, "unregister host to san", "waiting  3s for sancontroller to do register")
		time.Sleep(3 * time.Second)
		return nil
	}

	if host.Status.Phase != hostv1.HostUnInstalling {
		err := c.UpdatePhaseStatus(host, hostv1.HostUnInstalling)
		return err
	}

	log.V(2).Infof("%s:start uninstall node", host.GetName())
	c.recorder.Event(host, corev1.EventTypeNormal, "start uninstall", "start uninstall")

	log.V(2).Infof("%s: uninstall node: unSyncNodeLabel ok", host.GetName())

	cfg, err := generateHostCfg(host)
	if err != nil {
		c.recorder.Event(host, corev1.EventTypeWarning, "generateHostCfg fail", err.Error())
		return fmt.Errorf("generateHostCfg  fail %s", err.Error())
	}

	err = unDeployNode(*cfg)
	if err != nil {
		c.recorder.Event(host, corev1.EventTypeWarning, "unDeployNode fail", err.Error())
		return err
	}

	err = c.clearNodeLabel(host)
	if err != nil {
		c.recorder.Event(host, corev1.EventTypeWarning, "clearNodeLabel fail", err.Error())
		return err
	}

	log.V(2).Infof("%s: uninstall node sucess and delete the resource", host.GetName())
	return c.hostClientset.HostV1alpha1().Hosts().Delete(context.TODO(), host.GetName(), metav1.DeleteOptions{})

}

func (c *Controller) syncHostResoure(updatehost *hostv1.Host, node *corev1.Node) (bool, error) {
	var err error
	change := false
	cpucapacity, ok := node.Status.Capacity[corev1.ResourceCPU]
	if ok {
		if updatehost.Status.Capacity.Cpu.String() != cpucapacity.String() {
			change = true
			updatehost.Status.Capacity.Cpu = cpucapacity
		}
	}

	memcapacity, ok := node.Status.Capacity[corev1.ResourceMemory]
	if ok {
		if updatehost.Status.Capacity.Memery.String() != memcapacity.String() {
			change = true
			updatehost.Status.Capacity.Memery = memcapacity
		}

	}

	podcapacity, ok := node.Status.Capacity[corev1.ResourcePods]
	if ok {
		if updatehost.Status.Capacity.Pods.String() != podcapacity.String() {
			change = true
			updatehost.Status.Capacity.Pods = podcapacity
		}
	}

	cpuAllocatable, _ := node.Status.Allocatable[corev1.ResourceCPU]
	memAllocatable, _ := node.Status.Allocatable[corev1.ResourceMemory]
	podAllocatable, _ := node.Status.Allocatable[corev1.ResourcePods]

	var unitsAllocatable resource.Quantity

	maxunits, maxunitAnnExist := node.Annotations[hostv1.HostMaxUnitAnnotation]
	if maxunitAnnExist {
		unitsAllocatable, err = resource.ParseQuantity(maxunits)
		if err != nil {
			return change, err
		}

	}

	freePod := podAllocatable
	freeMemory := memAllocatable
	freeCPU := cpuAllocatable
	freeUnits := unitsAllocatable
	onePod, err := resource.ParseQuantity("1")
	if err != nil {
		return change, err
	}

	pods, err := c.podLister.List(labels.SelectorFromSet(labels.Set{vars.LabelDBScaleKey: vars.LabelDBScaleValue}))
	for _, pod := range pods {
		if pod.Spec.NodeName != node.GetName() {
			continue
		}

		if maxunitAnnExist {
			_unit, uerr := c.unitLister.Units(pod.Namespace).Get(pod.Name)
			if uerr == nil && _unit != nil {
				freeUnits.Sub(onePod)
			}
		}

		freePod.Sub(onePod)

		for _, container := range pod.Spec.Containers {
			for rName, rQuant := range container.Resources.Requests {
				switch rName {
				case corev1.ResourceCPU:
					freeCPU.Sub(rQuant)
				case corev1.ResourceMemory:
					freeMemory.Sub(rQuant)
				default:
				}
			}
		}
	}

	if maxunitAnnExist {
		if freeUnits.String() != updatehost.Status.Allocatable.Units.String() {
			change = true
			updatehost.Status.Allocatable.Units = freeUnits
		}

		if unitsAllocatable.String() != updatehost.Status.Capacity.Units.String() {
			change = true
			updatehost.Status.Capacity.Units = unitsAllocatable
		}
	}

	if freeCPU.String() != updatehost.Status.Allocatable.Cpu.String() {
		change = true
		updatehost.Status.Allocatable.Cpu = freeCPU
	}

	if freeMemory.String() != updatehost.Status.Allocatable.Memery.String() {
		change = true
		updatehost.Status.Allocatable.Memery = freeMemory
	}

	if freePod.String() != updatehost.Status.Allocatable.Pods.String() {
		change = true
		updatehost.Status.Allocatable.Pods = freePod
	}

	return change, nil
}

func (c *Controller) syncHostLocalVGs(updatehost *hostv1.Host, node *corev1.Node) (bool, error) {
	change := false
	for _, vg := range updatehost.Spec.LocalVGs {
		valueStr, ok := node.Annotations[hostv1.GetVgCapacityLabel(vg.Name)]
		if !ok {
			continue
		}
		value, err := resource.ParseQuantity(valueStr)
		if err != nil {
			return false, err
		}

		find := false
		for i := range updatehost.Status.Capacity.LocalVGs {
			if updatehost.Status.Capacity.LocalVGs[i].Name != vg.Name {
				continue
			}

			find = true
			if updatehost.Status.Capacity.LocalVGs[i].Size.String() != value.String() {
				change = true
				updatehost.Status.Capacity.LocalVGs[i].Size = value
			}
		}

		if !find {
			change = true
			updatehost.Status.Capacity.LocalVGs = append(updatehost.Status.Capacity.LocalVGs, hostv1.VGStatus{
				Name:  vg.Name,
				Size:  value,
				Level: vg.Level,
			})
		}
	}

	for _, vg := range updatehost.Spec.LocalVGs {
		valueStr, ok := node.Annotations[hostv1.GetVgAllocatableLabel(vg.Name)]
		if !ok {
			continue
		}

		value, err := resource.ParseQuantity(valueStr)
		if err != nil {
			return false, err
		}

		find := false
		for i := range updatehost.Status.Allocatable.LocalVGs {
			if updatehost.Status.Allocatable.LocalVGs[i].Name != vg.Name {
				continue
			}

			find = true
			if updatehost.Status.Allocatable.LocalVGs[i].Size.String() != value.String() {
				change = true
				updatehost.Status.Allocatable.LocalVGs[i].Size = value
			}
		}

		if !find {
			change = true
			updatehost.Status.Allocatable.LocalVGs = append(updatehost.Status.Allocatable.LocalVGs, hostv1.VGStatus{
				Name:  vg.Name,
				Size:  value,
				Level: vg.Level,
			})
		}
	}

	//有脏数据
	if len(updatehost.Status.Allocatable.LocalVGs) > len(updatehost.Spec.LocalVGs) {
		updatehost.Status.Capacity.LocalVGs = []hostv1.VGStatus{}
		updatehost.Status.Allocatable.LocalVGs = []hostv1.VGStatus{}
		return c.syncHostLocalVGs(updatehost, node)
	}

	return change, nil
}

func (c *Controller) clearNodeLabel(host *hostv1.Host) error {
	node, err := c.kubeclientset.CoreV1().Nodes().Get(context.TODO(), host.Name, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	if errors.IsNotFound(err) {
		return nil
	}

	find := false
	for hlabel, _ := range host.Labels {
		for nlabel, _ := range node.Labels {
			if hlabel == nlabel {
				find = true
				delete(node.Labels, hlabel)
			}
		}
	}
	for hannotain, _ := range host.Annotations {
		for nannotain, _ := range node.Annotations {
			if hannotain == nannotain {
				find = true
				delete(node.Annotations, nannotain)
			}
		}
	}

	for nannotain, _ := range node.Annotations {
		switch nannotain {
		case hostv1.HostMaxUnitAnnotation, hostv1.HostcomponentAnnotation, hostv1.SanInitiatorAnnotation,
			hostv1.NodeUsageLimtAnnotation, hostv1.VGsAnnotation, hostv1.SanInitiatorTypeAnnotation, hostv1.HostNetworkMode:
			find = true
			delete(node.Annotations, nannotain)
		default:
			slices := strings.Split(nannotain, ".")
			if len(slices) == 3 && slices[0] == "upm" && (slices[2] == "allocatable" || slices[2] == "capacity") {
				find = true
				delete(node.Annotations, nannotain)
			}
		}
	}

	if find {
		klog.Infof("%s update node to clear host related label/Annotations", node.Name)
		_, err = c.kubeclientset.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *Controller) syncHostStatus(host *hostv1.Host, node *corev1.Node) (*hostv1.Host, error) {
	var err error
	change := false
	updateHost := host.DeepCopy()

	for _, condition := range node.Status.Conditions {
		if condition.Type != corev1.NodeReady {
			continue
		}

		if condition.Status == corev1.ConditionTrue && host.Status.NodeReady != true {
			change = true
			updateHost.Status.NodeReady = true

		} else if condition.Status != corev1.ConditionTrue && host.Status.NodeReady == true {
			change = true
			updateHost.Status.NodeReady = false
		}
	}

	_change, err := c.syncHostResoure(updateHost, node)
	if err != nil {
		return nil, err
	}

	change = change || _change

	_change, err = c.syncHostLocalVGs(updateHost, node)
	if err != nil {
		return nil, err
	}

	hostnodeinfo, err := host.Status.NodeInfo.Marshal()
	if err != nil {
		return nil, fmt.Errorf("host.Status.NodeInfo.Marshal() err:%s", err.Error())
	}

	nodeinfo, err := node.Status.NodeInfo.Marshal()
	if err != nil {
		return nil, fmt.Errorf("node.Status.NodeInfo.Marshal() err:%s", err.Error())
	}

	if string(hostnodeinfo) != string(nodeinfo) {
		updateHost.Status.NodeInfo = node.Status.NodeInfo
		change = true
	}

	if change || _change {
		log.V(4).Infof("upate %s: host.status:%v", updateHost.GetName(), updateHost.Status)
		updateHost, err = c.hostClientset.HostV1alpha1().Hosts().UpdateStatus(context.TODO(), updateHost, metav1.UpdateOptions{})
		if err != nil {
			log.V(1).Infof("syncHostStatus  key:%s fail: %s)",
				host.GetNamespace()+"/"+host.GetName(), err.Error())
			return nil, err
		}
	}

	return updateHost, nil
}

func (c *Controller) updateSanInitiator(host *hostv1.Host, initiator hostv1.Initiator) error {
	updateHost := host.DeepCopy()
	updateHost.Spec.San.Initiator.ID = initiator.ID
	// _, err := c.hostClientset.HostV1alpha1().Hosts().Update(updateHost)
	_, err := c.patchHost(host, updateHost)
	if err != nil {
		log.V(1).Infof("updateSanInitiator  key:%s fail: %s)",
			host.GetName(), err.Error())
		return err
	}

	return nil
}

func (c *Controller) UpdatePhaseStatus(host *hostv1.Host, phase hostv1.HostPhase) error {

	updateHost := host.DeepCopy()

	updateHost.Status.Phase = phase
	_, err := c.hostClientset.HostV1alpha1().Hosts().UpdateStatus(context.TODO(), updateHost, metav1.UpdateOptions{})
	if err != nil {
		log.V(1).Infof("UpdateHostPhaseStatus  key:%s fail: %s,phase :%s->%s)",
			host.GetName(), err.Error(), host.Status.Phase, phase)
		return fmt.Errorf("phase: %s-> %s;err:%s", host.Status.Phase, phase, err.Error())
	}

	time.Sleep(150 * time.Millisecond)
	return nil

}

func needHostInstall(host *hostv1.Host) bool {
	if host.Spec.ActCode == hostv1.DeletCode {
		return false
	}

	if host.Status.Phase == "" || host.Status.Phase == hostv1.HostInstalling {
		return true
	}

	return false
}

func NeedSanRegister(host *hostv1.Host) bool {
	if host.Spec.San == nil || host.Spec.ActCode == hostv1.DeletCode {
		return false
	}

	if host.Status.Phase == hostv1.HostInstalled {
		return true
	}

	return false
}

func NeedSanUnregister(host *hostv1.Host) bool {
	if host.Spec.San == nil {
		return false
	}

	if host.Spec.ActCode == hostv1.DeletCode && host.Status.Phase == hostv1.HostReady {
		return true
	}

	return false
}

/*
func (c *Controller) mergeHostLabelToNode(host *hostv1.Host, node *corev1.Node) (err error) {

	updatenode := node.DeepCopy()
	hlabel := host.Labels
	change := false

	for k, v := range hlabel {
		_v, ok := node.Labels[k]
		if !ok {
			updatenode.Labels[k] = v
			change = true
			continue
		}

		if v != _v {
			updatenode.Labels[k] = v
			change = true
		}
	}

	if change {
		_, err = c.kubeclientset.CoreV1().Nodes().Update(updatenode)
	}

	return err

}

func (c *Controller) syncNodeVgsAnnotation(host *hostv1.Host, node *corev1.Node) error {
	vgs := map[string]string{}
	for _, vg := range host.Spec.LocalVGs {
		vgs[vg.Name] = string(vg.Level)
	}
	vgstr, err := json.Marshal(vgs)
	if err != nil {
		return err
	}

	value, ok := node.Annotations[hostv1.VGsAnnotation]
	if !ok || value != string(vgstr) {
		if err := c.updateVgsAnnotation(node, string(vgstr)); err != nil {
			return err
		}
	}

	return nil
}
*/

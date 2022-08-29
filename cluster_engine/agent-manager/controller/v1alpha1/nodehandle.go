package v1alpha1

import (
	"context"
	"encoding/json"
	"fmt"
	stdexec "os/exec"
	"path/filepath"
	"strconv"
	"time"

	hostv1 "github.com/upmio/dbscale-kube/pkg/apis/host/v1alpha1"
	"github.com/upmio/dbscale-kube/pkg/utils"
	"github.com/upmio/dbscale-kube/pkg/utils/exec"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	log "k8s.io/klog/v2"
)

type VGStatus struct {
	Free int64  `json:"vg_free"`
	Size int64  `json:"vg_size"`
	Name string `json:"vg_name"`
}

const (
	defaultTimeout = time.Minute
	hostMGR        = "hostMGR/hostMGR"
)

func (c *Controller) patchNode(old, _new *corev1.Node) (*corev1.Node, error) {
	patch, update, err := utils.GenerateMergePatch(old, _new, corev1.Node{})
	if err != nil || !update {
		return old, err
	}
	log.V(4).Infof("%s patchNode: %s", old.Name, patch)
	return c.kubeclientset.CoreV1().Nodes().Patch(context.TODO(), old.Name, types.MergePatchType, patch, metav1.PatchOptions{})
}

func (c *Controller) SyncNodeHandle(key string) error {
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	if name != c.HostName {
		return nil
	}

	node, err := c.nodeLister.Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	maps := map[string]string{}
	execfile := filepath.Join(c.ShellDir, hostMGR)

	vgcfg, ok := node.Annotations[hostv1.VGsAnnotation]
	if ok {
		vgmap := map[string]string{}
		err = json.Unmarshal([]byte(vgcfg), &vgmap)
		if err != nil {
			return fmt.Errorf("%s:upm.host.vgs annotation Unmarshal fail:%s (data:%s)", node.GetName(), err.Error(), vgcfg)
		}

		vgs := []string{}
		for vgname := range vgmap {
			vgs = append(vgs, vgname)
		}
		if len(vgs) > 0 {

			vglist, err := getVgInfo(execfile, vgs)
			if err != nil {
				return err
			}

			//schedulerinfos := map[string]string{}
			for _, vg := range vglist {
				maps[hostv1.GetVgAllocatableLabel(vg.Name)] = resource.NewQuantity(vg.Free, resource.BinarySI).String()
				maps[hostv1.GetVgCapacityLabel(vg.Name)] = resource.NewQuantity(vg.Size, resource.BinarySI).String()
				//level, ok := vgmap[vg.Name]
				//if !ok {
				//	return fmt.Errorf("shouldn't   happen:%s not find in %v", vg.Name, vglist)
				//}

				//schedulerinfos[level] = resource.NewQuantity(vg.Size, resource.BinarySI).String()
			}

			//schedulerinfoStr, err := json.Marshal(schedulerinfos)
			//if err != nil {
			//	return fmt.Errorf("Marshal schedulerinfos fail:%s", err.Error())
			//}
			//
			//maps[hostv1.NodeLocalVolumeAnnocation] = string(schedulerinfoStr)
		}
	}

	initiatorType, ok := node.Annotations[hostv1.SanInitiatorTypeAnnotation]
	if ok && initiatorType != "local" {
		setInitiatorInfo(execfile, initiatorType, maps)
	}

	networkmode := node.Annotations[hostv1.HostNetworkMode]

	setComponentInfo(execfile, maps)
	setMaxUnitInfo(execfile, networkmode, maps)
	err = c.updateNodeAnnotationInfo(node, maps)
	return err
}

func (c *Controller) updateNodeAnnotationInfo(node *corev1.Node, maps map[string]string) error {
	change := false
	updatenode := node.DeepCopy()
	for key, value := range maps {
		_value, ok := node.Annotations[key]
		if !ok {
			change = true
			updatenode.Annotations[key] = value
		}

		if value != _value {
			change = true
			updatenode.Annotations[key] = value
		}
	}

	if change {
		// _, err := c.kubeclientset.CoreV1().Nodes().Update(updatenode)
		_, err := c.patchNode(node, updatenode)
		if err != nil {
			return err
		}
	}

	return nil
}

func setMaxUnitInfo(shellfile, networkmode string, maps map[string]string) {
	max, err := getHardMaxUnit(shellfile, networkmode)
	if err != nil {
		log.Errorf("getHardMaxUnit fail:%s", err.Error())
		return
	}
	maps[hostv1.HostMaxUnitAnnotation] = strconv.Itoa(max)
}

func getHardMaxUnit(shellfile, networkmode string) (int, error) {
	ret := struct {
		MaxUnit int `json:"max_unit"`
	}{}
	arg := struct {
		NetworkMode string `json:"network_mode"`
	}{
		NetworkMode: networkmode,
	}
	cmd := []string{shellfile, "max_unit", "count"}
	err := exec.CommonShellExec(cmd, defaultTimeout, arg, &ret)
	return ret.MaxUnit, err
}

func getVgInfo(shellfile string, vgs []string) ([]VGStatus, error) {
	arg := struct {
		Vgs []string `json:"vg_name"`
	}{
		Vgs: vgs,
	}
	ret := []VGStatus{}
	cmd := []string{shellfile, "vg", "get"}
	err := exec.CommonShellExec(cmd, defaultTimeout, arg, &ret)
	return ret, err
}

func setComponentInfo(shellfile string, maps map[string]string) map[string]string {
	args := []string{"component", "status"}
	out, err := stdexec.Command(shellfile, args...).CombinedOutput()
	if err != nil {
		log.Errorf("setComponentInfo fail:%s,%s(cmd: %s %v)", err.Error(), string(out), shellfile, args)
		return maps
	}

	maps[hostv1.HostcomponentAnnotation] = string(out)
	return maps
}

func setInitiatorInfo(execfile string, initiatortype string, maps map[string]string) map[string]string {
	ret, err := getInitiatorInfo(execfile, initiatortype)
	if err != nil {
		log.Errorf("getInitiatorInfo fail :%s", err)
		return maps
	}

	initiatorjson, err := json.Marshal(ret)
	if err != nil {
		log.Errorf("err:%s", err)
		return maps
	}
	maps[hostv1.SanInitiatorAnnotation] = string(initiatorjson)

	return maps
}

func getInitiatorInfo(shellfile string, initiatortype string) (hostv1.Initiator, error) {

	ret := hostv1.Initiator{}

	arg := struct {
		Type string `json:"type"`
	}{
		Type: initiatortype,
	}
	cmd := []string{shellfile, "initiator", "list"}
	err := exec.CommonShellExec(cmd, defaultTimeout, arg, &ret)
	return ret, err
}

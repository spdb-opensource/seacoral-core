package v1alpha4

/*
import (
	// "bytes"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	unitv4 "github.com/upmio/dbscale-kube/pkg/apis/unit/v1alpha4"
	"github.com/upmio/dbscale-kube/pkg/parser"
	podutil "github.com/upmio/dbscale-kube/pkg/utils/pod"

	// executil "github.com/upmio/dbscale-kube/pkg/utils/exec"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"
)


func (ctrl *Controller) reloadConfigFileLoop() {
	klog.V(2).Infof("ConfigFileLoop:sync all units config to configmaps")

	units, err := ctrl.unitLister.List(labels.Everything())
	if err != nil {
		return
	}

	for i := range units {
		err := ctrl.reloadUnitConfig(units[i])
		if err != nil {
			klog.V(2).Infof("reloadUnitConfig,Unit %s error:%v", units[i].Name, err)
			continue
		}
	}
}

func (ctrl *Controller) reloadUnitConfig(unit *unitv4.Unit) error {
	if unit.Spec.Action.Delete != nil ||
		unit.Spec.Action.Rebuild != nil ||
		unit.GetDeletionTimestamp() != nil {
		return nil
	}

	pod, err := ctrl.podLister.Pods(unit.Namespace).Get(unitv4.GetPodName(unit))
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	if !podutil.IsRunning(pod) {
		// klog.V(3).Infof("reloadUnitConfig,pod %s  continaer not ready", pod.Name)
		return fmt.Errorf("reloadUnitConfig,pod %s  continaer not ready", pod.Name)
	}

	path, err := ctrl.getConfigFilePath(unit)
	if err != nil {
		return err
	}

	containerName := unit.Spec.MainContainerName
	cmd := []string{"cat", path}

	text, err := ctrl.podExec(pod, containerName, cmd)
	if err != nil {
		// klog.V(3).Infof("catPodConfig,Pod %s error:%s", pod.Name, err)
		return err
	}

	pr, err := parser.NewParser(ctrl.kubeClient, unit, nil)
	if err != nil {
		return err
	}

	err = pr.ParseData(text)
	if err != nil {
		// klog.V(2).Infof("Unit %s od %s parse config file error:%s\n%s", unit.Name, pod.Name, err, text)
		return err
	}

	cfg, err := ctrl.configMapLister.ConfigMaps(unit.Namespace).Get(unitv4.GetUnitConfgName(unit))
	if err != nil && errors.IsNotFound(err) {
		ref := metav1.NewControllerRef(unit, unitKind)
		configmap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:            unitv4.GetUnitConfgName(unit),
				Namespace:       unit.GetNamespace(),
				OwnerReferences: []metav1.OwnerReference{*ref},
			},
			Data: map[string]string{
				unitv4.ConfgDataTab: text,
			},
		}
		_, err := ctrl.kubeClient.CoreV1().ConfigMaps(unit.Namespace).Create(configmap)
		return err
	}

	if err != nil {
		return err
	}

	if cfg.Data[unitv4.ConfgDataTab] != text {
		updatecfg := cfg.DeepCopy()
		updatecfg.Data[unitv4.ConfgDataTab] = text

		_, err = ctrl.kubeClient.CoreV1().ConfigMaps(updatecfg.Namespace).Update(updatecfg)
		return err
	}

	return nil

}

func (ctrl *Controller) updateConfig(unit *unitv4.Unit, container, path, content string, backup bool) error {
	if backup {
		err := ctrl.bakcupFile(unit, container, path)
		if err != nil {
			// return err
			klog.V(1).Infof(" %s/%s-%s:backupFile fail:%s", unit, container, path, err.Error())
		}
	}
	//TODO:使用脚本
	cmd := []string{"/bin/sh", "-c", fmt.Sprintf(`echo "%s" > %s`, content, path)}
	_, err := ctrl.UnitExec(unit, unit.Spec.MainContainerName, cmd)
	if err != nil {
		return err
	}

	return nil
}

func (ctrl *Controller) bakcupFile(unit *unitv4.Unit, container, path string) error {
	ext := filepath.Ext(path)
	dst := strings.Replace(path, ext, "-"+time.Now().Format("2006-01-02T15:04:05")+ext, 1)

	cmd := []string{"cp", path, dst}
	_, err := ctrl.UnitExec(unit, unit.Spec.MainContainerName, cmd)
	if err != nil {
		return err
	}

	return nil
}
*/

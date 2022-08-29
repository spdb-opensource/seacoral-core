package v1alpha4

import (
	"context"
	"fmt"
	sanv1 "github.com/upmio/dbscale-kube/pkg/apis/san/v1alpha1"
	unitv4 "github.com/upmio/dbscale-kube/pkg/apis/unit/v1alpha4"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"time"
)

func (ctrl *Controller) rebuildUnit(unit *unitv4.Unit) error {

	var err error

	if unit.Spec.Action.Rebuild.RetainVolume != nil && *unit.Spec.Action.Rebuild.RetainVolume == true {
		rebuildStatus := unitv4.RebuildVolumeStatus{CurVolumeSuffix: 1, RetainVolumeSuffix: 0}
		update := true
		if unit.Status.RebuildStatus != nil {
			rebuildStatus = *unit.Status.RebuildStatus
			if rebuildStatus.RetainVolumeSuffix == rebuildStatus.CurVolumeSuffix {
				rebuildStatus.CurVolumeSuffix += 1
			} else {
				update = false
			}
		}

		if update {
			unit.Status.RebuildStatus = &rebuildStatus
			unit, err = ctrl.clientset.UnitV1alpha4().Units(unit.Namespace).UpdateStatus(context.TODO(), unit, metav1.UpdateOptions{})
			if err != nil {
				return err
			}
			ctrl.recorder.Eventf(unit, corev1.EventTypeNormal, "rebuild", "update rebuildStatus:%v", unit.Status.RebuildStatus)
		}
	}

	err = ctrl.waitForPodDeleted(unit)
	if err != nil {
		return err
	}

	ctrl.recorder.Eventf(unit, corev1.EventTypeNormal, "rebuild", "rebuild: delete pod ok")

	if unit.Status.RebuildStatus != nil {
		ctrl.recorder.Eventf(unit, corev1.EventTypeNormal, "rebuild", "the CurVolumeSuffix:%s", unit.Status.RebuildStatus.CurVolumeSuffix)
	}

	err = ctrl.waitForLungroupsDeleted(unit)
	if err != nil {
		return err
	}
	ctrl.recorder.Eventf(unit, corev1.EventTypeNormal, "rebuild", "rebuild: delete lungroup ok")

	err = ctrl.waitForVPDeleted(unit)
	if err != nil {
		return err
	}
	ctrl.recorder.Eventf(unit, corev1.EventTypeNormal, "rebuild", "rebuild: delete voluempath ok")

	unitv4.SetContionStatus(unit, unitv4.Condition{
		Type:   unitv4.InitStartCondition,
		Status: unitv4.ConditionFalse,
	})

	unit, err = ctrl.clientset.UnitV1alpha4().Units(unit.Namespace).UpdateStatus(context.TODO(), unit, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	ctrl.recorder.Eventf(unit, corev1.EventTypeNormal, "rebuild", "set InitStartCondition  false")

	if unit.Spec.Action.Rebuild.NodeName != nil {
		unit.Spec.Template.Spec.NodeName = *unit.Spec.Action.Rebuild.NodeName
	} else {
		unit.Spec.Template.Spec.NodeName = ""
	}

	unit.Spec.Action.Rebuild = nil
	unit.Spec.UnService = false
	unit, err = ctrl.clientset.UnitV1alpha4().Units(unit.Namespace).Update(context.TODO(), unit, metav1.UpdateOptions{})

	if err == nil {
		ctrl.recorder.Eventf(unit, corev1.EventTypeNormal, "rebuild", "update unit to trigger recreate pod")
	}

	return err
}

func isNodeReady(node *corev1.Node) bool {
	if node.Status.Phase != corev1.NodeRunning {
		return false
	}
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady {
			if condition.Status == corev1.ConditionTrue {
				return true
			}
			return false
		}
	}

	return false
}

func (ctrl *Controller) migrateUnit(unit *unitv4.Unit) error {
	node, err := ctrl.kubeClient.CoreV1().Nodes().Get(context.TODO(), unit.Spec.Action.Migrate.NodeName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			ctrl.recorder.Eventf(unit, corev1.EventTypeWarning, "migrate", "not find % node", node.Name)
			return nil
		}
		return err
	}

	if !isNodeReady(node) {
		ctrl.recorder.Eventf(unit, corev1.EventTypeWarning, "migrate", "the %s node status not ready", node.Name)
		return nil
	}

	err = ctrl.waitForPodDeleted(unit)
	if err != nil {
		return err
	}

	for _, vc := range unit.Spec.VolumeClaims {
		lg, err := ctrl.lunClient.SanV1alpha1().Lungroups().Get(context.TODO(), unitv4.GetLunGroupName(unit, vc.Name), metav1.GetOptions{})
		if err != nil {
			return err
		}
		if lg.Spec.Action.Migrate == nil {
			lg.Spec.Action.Migrate = &sanv1.MigrateAction{
				Force: unit.Spec.Action.Migrate.Force,
				Node:  unit.Spec.Action.Migrate.NodeName,
			}
			_, err = ctrl.lunClient.SanV1alpha1().Lungroups().Update(context.TODO(), lg, metav1.UpdateOptions{})
			if err != nil {
				return err
			}
			ctrl.recorder.Eventf(unit, corev1.EventTypeNormal, "migrate", "update lg %s to trigger migrate", lg.Name)
		}
	}

	for _, vc := range unit.Spec.VolumeClaims {
		err = wait.PollImmediate(time.Second*10, time.Minute, func() (bool, error) {
			lg, err := ctrl.lunClient.SanV1alpha1().Lungroups().Get(context.TODO(), unitv4.GetLunGroupName(unit, vc.Name), metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			if lg.Spec.Action.Migrate == nil && lg.Status.Phase == sanv1.LunGroupReady {
				return true, nil
			}
			return false, nil
		})
		if err != nil {
			ctrl.recorder.Eventf(unit, corev1.EventTypeWarning, "migrate", "wait for %s fail:%s", unitv4.GetLunGroupName(unit, vc.Name), err.Error())
			return err
		}
	}

	unit.Spec.Template.Spec.NodeName = unit.Spec.Action.Migrate.NodeName
	unit.Spec.Action.Migrate = nil
	unit, err = ctrl.clientset.UnitV1alpha4().Units(unit.Namespace).Update(context.TODO(), unit, metav1.UpdateOptions{})
	if err == nil {
		ctrl.recorder.Eventf(unit, corev1.EventTypeNormal, "migrate", "update unit to trigger recreate pod")
	}
	return err

}

func (ctrl *Controller) waitForPodDeleted(unit *unitv4.Unit) error {
	delopt := metav1.DeleteOptions{}
	if unit.Spec.Action.Rebuild.Force {
		second := int64(0)
		delopt = metav1.DeleteOptions{GracePeriodSeconds: &second}
	}

	err := ctrl.kubeClient.CoreV1().Pods(unit.GetNamespace()).Delete(context.TODO(), unitv4.GetPodName(unit), delopt)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	err = wait.PollImmediate(time.Second*2, 10*time.Second, func() (bool, error) {
		_, err := ctrl.kubeClient.CoreV1().Pods(unit.GetNamespace()).Get(context.TODO(), unit.GetName(), metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return true, nil
			}
			return false, err
		}
		return false, nil
	})
	if err != nil {
		return fmt.Errorf("wait for pod deleted:%s", err.Error())
	}

	return nil
}

func (ctrl *Controller) rebackVolumeAction(unit *unitv4.Unit) error {
	var err error
	if unit.Status.RebuildStatus == nil || unit.Status.RebuildStatus.CurVolumeSuffix == unit.Status.RebuildStatus.RetainVolumeSuffix {
		ctrl.recorder.Eventf(unit, corev1.EventTypeNormal, "ReuseRetainVolume", "do nothing")
		unit.Spec.Action.ReuseRetainVolume = nil
		_, err = ctrl.clientset.UnitV1alpha4().Units(unit.Namespace).Update(context.TODO(), unit, metav1.UpdateOptions{})
		return err
	}

	oldnode := ""
	oldvolumes := []unitv4.PVCRequest{}
	for i, _ := range unit.Spec.VolumeClaims {
		retainvp := unit.Spec.VolumeClaims[i]
		vpname := unitv4.GetRetainVolumeName(unit, unit.Spec.VolumeClaims[i].Name)
		vp, err := ctrl.lvmClient.LvmV1alpha1().VolumePaths().Get(context.TODO(), vpname, metav1.GetOptions{})
		if err != nil {
			return err
		}
		retainvp.Storage.Request = vp.Spec.Size
		oldvolumes = append(oldvolumes, retainvp)
		if oldnode == "" && vp.Status.BindingNode != "" {
			oldnode = vp.Status.BindingNode
		}
	}

	if oldnode == "" {
		ctrl.recorder.Eventf(unit, corev1.EventTypeWarning, "rebacKVolumeAction", "Retain volume Status.BindingNode is emtpy")
		return fmt.Errorf("Retain volume Status.BindingNode is emtpy")
	}

	err = ctrl.waitForPodDeleted(unit)
	if err != nil {
		return err
	}

	ctrl.recorder.Eventf(unit, corev1.EventTypeNormal, "rebacKVolumeAction", "delete pod ok")
	//
	//err = ctrl.waitForLungroupsDeleted(unit)
	//if err != nil {
	//	return err
	//}
	//ctrl.recorder.Eventf(unit, corev1.EventTypeNormal, "rebacKVolumeAction", "delete lungroup(CurVolumeSuffix:%s) ok", unit.Status.RebuildStatus.CurVolumeSuffix)
	//
	//err = ctrl.waitForVPDeleted(unit)
	//if err != nil {
	//	return err
	//}
	//ctrl.recorder.Eventf(unit, corev1.EventTypeNormal, "rebacKVolumeAction", "delete voluempath(CurVolumeSuffix:%s)  ok", unit.Status.RebuildStatus.CurVolumeSuffix)

	if unitv4.GetContionStatus(unit, unitv4.InitStartCondition) == unitv4.ConditionFalse {
		unitv4.SetContionStatus(unit, unitv4.Condition{
			Type:   unitv4.InitStartCondition,
			Status: unitv4.ConditionTrue,
		})
	}
	unit.Status.RebuildStatus.CurVolumeSuffix = unit.Status.RebuildStatus.RetainVolumeSuffix
	unit, err = ctrl.clientset.UnitV1alpha4().Units(unit.Namespace).UpdateStatus(context.TODO(), unit, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	ctrl.recorder.Eventf(unit, corev1.EventTypeNormal, "rebacKVolumeAction", "set status to use retain volume")

	unit.Spec.VolumeClaims = oldvolumes
	unit.Spec.Template.Spec.NodeName = oldnode
	unit.Spec.Action.ReuseRetainVolume = nil
	_, err = ctrl.clientset.UnitV1alpha4().Units(unit.Namespace).Update(context.TODO(), unit, metav1.UpdateOptions{})
	if err == nil {
		ctrl.recorder.Eventf(unit, corev1.EventTypeNormal, "rebacKVolumeAction", "update unit to trigger recreate pod")
	}
	return err
}

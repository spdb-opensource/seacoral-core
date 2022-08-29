package v1alpha4

import (
	"context"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"time"

	lgv1 "github.com/upmio/dbscale-kube/pkg/apis/san/v1alpha1"
	unitv4 "github.com/upmio/dbscale-kube/pkg/apis/unit/v1alpha4"
	vpv1 "github.com/upmio/dbscale-kube/pkg/apis/volumepath/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

func (ctrl *Controller) removeUnit(unit *unitv4.Unit) error {
	var (
		gracePeriodSeconds = int64(300)
		propagationPolicy  = metav1.DeletePropagationForeground
	)

	delopt := metav1.DeleteOptions{}
	if unit.Spec.Action.Delete.Force {
		second := int64(0)
		delopt = metav1.DeleteOptions{GracePeriodSeconds: &second}
	}

	err := ctrl.kubeClient.CoreV1().Pods(unit.GetNamespace()).Delete(context.TODO(), unitv4.GetPodName(unit), delopt)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	err = ctrl.kubeClient.CoreV1().Services(unit.Namespace).Delete(context.TODO(), unitv4.GetServiceName(unit), metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	for _, claim := range unit.Spec.VolumeClaims {
		pvcname := unitv4.GetPersistentVolumeClaimName(unit, claim.Name)
		err := ctrl.kubeClient.CoreV1().PersistentVolumeClaims(unit.Namespace).Delete(context.TODO(), pvcname, metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
	}

	for _, claim := range unit.Spec.VolumeClaims {
		pvname := unitv4.GetPersistentVolumeName(unit, claim.Name)

		err := ctrl.kubeClient.CoreV1().PersistentVolumes().Delete(context.TODO(), pvname, metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
	}

	err = ctrl.kubeClient.CoreV1().ConfigMaps(unit.Namespace).Delete(context.TODO(), unitv4.GetUnitScriptConfigName(unit), metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	err = ctrl.kubeClient.CoreV1().ConfigMaps(unit.Namespace).Delete(context.TODO(), unitv4.GetUnitConfigName(unit), metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	err = ctrl.netClient.NetworkingV1alpha1().NetworkClaims().Delete(context.TODO(), unitv4.GetNetworkClaimName(unit), metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	err = ctrl.waitForLungroupsDeleted(unit)
	if err != nil {
		return err
	}

	err = ctrl.waitForVPDeleted(unit)
	if err != nil {
		return err
	}

	err = ctrl.clientset.UnitV1alpha4().Units(unit.Namespace).Delete(context.TODO(), unit.Name, metav1.DeleteOptions{
		GracePeriodSeconds: &gracePeriodSeconds,
		PropagationPolicy:  &propagationPolicy, // grace delete include pods
	})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	ctrl.unitStore.Delete(unit)

	return nil
}

func (ctrl *Controller) waitForVPDeleted(unit *unitv4.Unit) error {
	vps := []string{}

	for _, req := range unit.Spec.VolumeClaims {
		vps = append(vps, unitv4.GetVolumePathName(unit, req.Name))
	}

	err := wait.PollImmediate(time.Second*10, time.Minute, func() (bool, error) {
		exist := false

		for i := range vps {

			vp, err := ctrl.lvmClient.LvmV1alpha1().VolumePaths().Get(context.TODO(), vps[i], metav1.GetOptions{})
			if errors.IsNotFound(err) {
				continue
			}

			if err != nil {
				return false, err
			}

			exist = true
			if (unit.Spec.Action.Rebuild != nil && unit.Spec.Action.Rebuild.Force) ||
				(unit.Spec.Action.Delete != nil && unit.Spec.Action.Delete.Force) {
				err = ctrl.lvmClient.LvmV1alpha1().VolumePaths().Delete(context.TODO(), vps[i], metav1.DeleteOptions{})
				if err == nil {
					continue
				}
				return false, err
			}

			if vp.Spec.ActCode != vpv1.DeleteCode {
				vp = vp.DeepCopy()
				vp.Spec.ActCode = vpv1.DeleteCode

				vp, err = ctrl.lvmClient.LvmV1alpha1().VolumePaths().Update(context.TODO(), vp, metav1.UpdateOptions{})
				if err != nil {
					return false, err
				}
			}
		}

		return !exist, nil

	})

	return err
}

func (ctrl *Controller) triggerRetainVolumeDeleted(unit *unitv4.Unit) error {
	volumes := []string{}

	if unit.Status.RebuildStatus == nil {
		return fmt.Errorf("RebuildStatus==nil:should not happen")
	}
	if unit.Status.RebuildStatus.RetainVolumeSuffix == unit.Status.RebuildStatus.CurVolumeSuffix {
		return fmt.Errorf("RetainVolumeSuffix==CurVolumeSuffix: should not happen")
	}

	for _, req := range unit.Spec.VolumeClaims {
		volumes = append(volumes, unitv4.GetRetainVolumeName(unit, req.Name))
	}

	for i := range volumes {
		ctrl.recorder.Eventf(unit, corev1.EventTypeNormal, "ensureRebuildStatus", "trigger remove retain pv/pvc/vp:%s", volumes[i])
		err := ctrl.kubeClient.CoreV1().PersistentVolumeClaims(unit.Namespace).Delete(context.TODO(), volumes[i], metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return err
		}

		err = ctrl.kubeClient.CoreV1().PersistentVolumes().Delete(context.TODO(), volumes[i], metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return err
		}

		vp, err := ctrl.lvmClient.LvmV1alpha1().VolumePaths().Get(context.TODO(), volumes[i], metav1.GetOptions{})
		if errors.IsNotFound(err) {
			continue
		}

		vp = vp.DeepCopy()
		vp.Annotations["retain"] = "deleted"
		vp.Spec.ActCode = vpv1.DeleteCode
		vp, err = ctrl.lvmClient.LvmV1alpha1().VolumePaths().Update(context.TODO(), vp, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	//for i := range volumes {
	//	lg, err := ctrl.lunClient.SanV1alpha1().Lungroups().Get(context.TODO(), lgs[i], metav1.GetOptions{})
	//	if errors.IsNotFound(err) {
	//		continue
	//	}
	//	lg = lg.DeepCopy()
	//	lg.Spec.Action.Delete = &lgv1.DeleteAction{}
	//	lg, err = ctrl.lunClient.SanV1alpha1().Lungroups().Update(context.TODO(), lg, metav1.UpdateOptions{})
	//	if err != nil {
	//		return err
	//	}
	//}
	return nil
}

func (ctrl *Controller) waitForLungroupsDeleted(unit *unitv4.Unit) error {
	lgs := []string{}

	for _, req := range unit.Spec.VolumeClaims {
		lgs = append(lgs, unitv4.GetLunGroupName(unit, req.Name))
	}

	err := wait.PollImmediate(time.Second*10, time.Minute, func() (bool, error) {
		exist := false

		for i := range lgs {

			lg, err := ctrl.lunClient.SanV1alpha1().Lungroups().Get(context.TODO(), lgs[i], metav1.GetOptions{})
			if errors.IsNotFound(err) {
				continue
			}

			if err != nil {
				return false, err
			}

			exist = true
			if (unit.Spec.Action.Rebuild != nil && unit.Spec.Action.Rebuild.Force) ||
				(unit.Spec.Action.Delete != nil && unit.Spec.Action.Delete.Force) {
				err = ctrl.lunClient.SanV1alpha1().Lungroups().Delete(context.TODO(), lgs[i], metav1.DeleteOptions{})
				if err == nil {
					continue
				}
				return false, err
			}

			if lg.Spec.Action.Delete != nil {
				lg = lg.DeepCopy()
				lg.Spec.Action.Delete = &lgv1.DeleteAction{}

				lg, err = ctrl.lunClient.SanV1alpha1().Lungroups().Update(context.TODO(), lg, metav1.UpdateOptions{})
				if err != nil {
					return false, err
				}
			}
		}

		return !exist, nil
	})

	return err
}

package v1alpha1

import (
	"context"
	"fmt"
	"github.com/upmio/dbscale-kube/pkg/utils"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"time"

	hostv1 "github.com/upmio/dbscale-kube/pkg/apis/host/v1alpha1"
	"github.com/upmio/dbscale-kube/pkg/apis/san/v1alpha1"
	lvmv1alpha1 "github.com/upmio/dbscale-kube/pkg/apis/volumepath/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

// lungroupWorker processes items from  sanQueue. It must run only once,
// syncLungroup is not assured to be reentrant.
func (ctrl *Controller) lungroupWorker() {
	workFunc := func() bool {
		keyObj, quit := ctrl.lunQueue.Get()
		if quit {
			return true
		}
		defer ctrl.lunQueue.Done(keyObj)

		err := func() error {

			key, ok := keyObj.(string)
			if !ok {
				// As the item in the lunQueue is actually invalid, we call
				// Forget here else we'd go into a loop of attempting to
				// process a work item that is invalid.
				ctrl.lunQueue.Forget(keyObj)

				return fmt.Errorf("expected string in lunQueue but got %#v", keyObj)
			}

			start := time.Now()
			defer func() {
				klog.Infof("lungroupWorker[%s] %s", key, time.Since(start))
			}()

			_, name, err := cache.SplitMetaNamespaceKey(key)
			if err != nil {
				return fmt.Errorf("error getting name of lungroup %q to get lungroup from informer: %v", key, err)
			}

			lg, err := ctrl.lunLister.Get(name)
			if err == nil {

				// The san still exists in informer cache, the event must have
				// been add/update/sync
				err = ctrl.syncLungroupVolumepath(lg)
				if err != nil {
					ctrl.recorder.Eventf(lg, corev1.EventTypeWarning, failedSync, messageResourceSyncFailed, key, err)

					return fmt.Errorf("error getting name of lungroup %q to sync lungroup: %v", key, err)
				}

				return err
			}

			if errors.IsNotFound(err) {
				return nil
			}

			return fmt.Errorf("error getting lungroup %q from informer: %v", key, err)
		}()

		if err == nil {
			klog.Infof("lungroupWorker successfully synced '%s'", keyObj)
		} else {
			klog.Error("lungroupWorker  %s fail:%s", keyObj, err.Error())
		}

		ctrl.lunQueue.Forget(keyObj)

		//if err == nil {
		//	ctrl.lunQueue.Forget(keyObj)
		//} else {
		//	runtime.HandleError(err)
		//
		//	if ctrl.lunQueue.NumRequeues(keyObj) < maxRetries {
		//		ctrl.lunQueue.AddRateLimited(keyObj)
		//	} else {
		//		ctrl.lunQueue.Forget(keyObj)
		//	}
		//}

		return false
	}
	for {
		if quit := workFunc(); quit {
			klog.Infof("lungroup worker queue shutting down")
			return
		}
	}
}

func (ctrl *Controller) updateLungroupStatusPhase(lg *v1alpha1.Lungroup, phase v1alpha1.LunPhase) (*v1alpha1.Lungroup, error) {
	if lg.Status.Phase == phase {
		return lg, nil
	}
	update := lg.DeepCopy()
	update.Status.Phase = phase
	ctrl.recorder.Eventf(lg, corev1.EventTypeNormal, "updatephase", "update phase:%s->%s", lg.Status.Phase, phase)
	return ctrl.clientset.SanV1alpha1().Lungroups().UpdateStatus(context.TODO(), update, metav1.UpdateOptions{})
}

func (c *Controller) patchLungroup(old, _new *v1alpha1.Lungroup) (*v1alpha1.Lungroup, error) {
	patch, update, err := utils.GenerateMergePatch(old, _new, v1alpha1.Lungroup{})
	if err != nil || !update {
		return old, err
	}
	c.recorder.Eventf(old, corev1.EventTypeNormal, "patch", "data:%s", patch)
	return c.clientset.SanV1alpha1().Lungroups().Patch(context.TODO(), old.Name, types.MergePatchType, patch, metav1.PatchOptions{})
}

func (ctrl *Controller) patchLungroupSanInfo(lg *v1alpha1.Lungroup) (*v1alpha1.Lungroup, error) {
	if lg.Spec.Hostname == "" {
		return lg, fmt.Errorf("Spec.Hostname should not be emtpy")
	}

	host, err := ctrl.hostByName(lg.Spec.Hostname)
	if err != nil {
		return lg, err
	}

	runner, err := ctrl.runnerFromHost(*host)
	if err != nil {
		return lg, err
	}
	san := runner.SAN()

	updatelg := lg.DeepCopy()
	updatelg.Spec.San = san.GetName()
	updatelg.Spec.Vendor = san.Spec.Auth.Vendor
	return ctrl.patchLungroup(lg, updatelg)

}

func (ctrl *Controller) extend(lg *v1alpha1.Lungroup) error {
	var err error

	if lg.Status.Phase != v1alpha1.LunGroupReady && lg.Status.Phase != v1alpha1.LunGroupExtending {
		return fmt.Errorf("extend fail:lg staus must be %s or %s", v1alpha1.LunGroupReady, v1alpha1.LunGroupExtending)
	}

	if lg.Status.Phase != v1alpha1.LunGroupExtending {
		lg, err = ctrl.updateLungroupStatusPhase(lg, v1alpha1.LunGroupExtending)
		if err != nil {
			return err
		}
	}

	runner, err := ctrl.runnerBySan(lg.Spec.San)
	if err != nil {
		return err
	}

	err = runner.syncLungroupCapacityCmd(lg)
	if err != nil {
		//ctrl.updateLungroupStatusPhase(lg, v1alpha1.LunGroupExtendFailed)
		return err
	}

	vp, err := ctrl.lvmclient.LvmV1alpha1().VolumePaths().Get(context.TODO(), lg.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if vp.Spec.Size.Equal(lg.Spec.Capacity) {
		vp.Spec.LunIDs = lg.LunIDs()
		vp.Spec.Size = lg.Spec.Capacity
		if vp.Status.Status == lvmv1alpha1.VpExtendFail {
			vp.Spec.ActCode = lvmv1alpha1.ResetStatusCode
		}

		vp, err = ctrl.lvmclient.LvmV1alpha1().VolumePaths().Update(context.TODO(), vp, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	err = wait.PollImmediate(5*time.Second, 60*time.Second, func() (bool, error) {
		vp, err = ctrl.lvmclient.LvmV1alpha1().VolumePaths().Get(context.TODO(), lg.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if vp.Status.CurSize.Equal(vp.Spec.Size) && vp.Status.Status == lvmv1alpha1.VpBinding {
			return true, nil
		}
		if vp.Status.Status == lvmv1alpha1.VpExtendFail {
			return false, fmt.Errorf("vp extend failed")
		}
		return false, nil
	})

	if err != nil {
		if vp.Status.Status == lvmv1alpha1.VpExtendFail {
			//ctrl.updateLungroupStatusPhase(lg, v1alpha1.LunGroupExtendFailed)
		}
		return err
	}

	_, err = ctrl.updateLungroupStatusPhase(lg, v1alpha1.LunGroupReady)
	ctrl.recorder.Eventf(lg, corev1.EventTypeNormal, "extend", "extend lg ok")
	return err
}

func (ctrl *Controller) create(lg *v1alpha1.Lungroup) error {
	var err error

	if lg.Status.Phase != "" && lg.Status.Phase != v1alpha1.LunGroupCreating {
		return fmt.Errorf("create fail:lg staus must be empty or %s", v1alpha1.LunGroupCreating)
	}

	if lg.Status.Phase != "" {
		lg, err = ctrl.updateLungroupStatusPhase(lg, v1alpha1.LunGroupCreating)
		if err != nil {
			return err
		}
	}

	runner, err := ctrl.runnerBySan(lg.Spec.San)
	if err != nil {
		return err
	}

	host, err := ctrl.hostByName(lg.Spec.Hostname)
	if err != nil {
		return err
	}

	err = runner.syncLungroupCapacityCmd(lg)
	if err != nil {
		//ctrl.updateLungroupStatusPhase(lg, v1alpha1.LunGroupExtendFailed)
		return err
	}

	if lg.Status.IsMappingAllowed() {
		err = runner.createMappingView(lg, host.Spec.San.HostGroup, host.Name)
		if err != nil {
			return err
		}
	}

	vp, err := ctrl.lvmlister.Get(lg.Name)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}

		vp, err = ctrl.createVolumePath(*lg, *host)
		if err != nil {
			return err
		}
	}

	err = wait.PollImmediate(5*time.Second, 60*3*time.Second, func() (bool, error) {
		vp, err = ctrl.lvmclient.LvmV1alpha1().VolumePaths().Get(context.TODO(), lg.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if vp.Status.CurSize.Equal(vp.Spec.Size) && vp.Status.Status == lvmv1alpha1.VpBinding {
			return true, nil
		}
		if vp.Status.Status == lvmv1alpha1.VpCreateFail {
			return false, fmt.Errorf("vp create failed")
		}
		return false, nil
	})

	if err != nil {
		if vp.Status.Status == lvmv1alpha1.VpCreateFail {
			ctrl.updateLungroupStatusPhase(lg, v1alpha1.LunGroupCreatFailed)
		}
		return err
	}

	_, err = ctrl.updateLungroupStatusPhase(lg, v1alpha1.LunGroupReady)
	ctrl.recorder.Eventf(lg, corev1.EventTypeNormal, "create", "create lg ok")
	return err
}

func (ctrl *Controller) syncLungroupVolumepath(lg *v1alpha1.Lungroup) error {
	var err error
	if lg.Spec.Action.Delete != nil {
		return ctrl.deteleLungroup(lg)
	}

	if lg.Spec.Action.Migrate != nil {
		return ctrl.migrateLungroup(lg)
	}

	if lg.Spec.San == "" {
		lg, err = ctrl.patchLungroupSanInfo(lg)
		if err != nil {
			return err
		}
	}

	//create
	if lg.Status.Phase == "" || lg.Status.Phase == v1alpha1.LunGroupCreating {
		return ctrl.create(lg)
	}

	//extend
	if lg.Capacity() > lg.CurSize() &&
		(lg.Status.Phase == v1alpha1.LunGroupReady || lg.Status.Phase == v1alpha1.LunGroupExtending) {
		return ctrl.extend(lg)
	}

	return nil
}

func (ctrl *Controller) createVolumePath(lg v1alpha1.Lungroup, host hostv1.Host) (*lvmv1alpha1.VolumePath, error) {
	ids := lg.LunIDs()

	if len(lg.Status.Luns) == 0 || len(ids) == 0 ||
		lg.Status.MappingView == nil ||
		lg.Status.MappingView.HostGroup == "" {
		return nil, fmt.Errorf("lungroup %s not ready from create volumepath", lg.Name)
	}

	return ctrl.lvmclient.LvmV1alpha1().VolumePaths().Create(context.TODO(), &lvmv1alpha1.VolumePath{
		ObjectMeta: metav1.ObjectMeta{
			Name:            lg.Name,
			Labels:          lg.GetLabels(),
			Annotations:     lg.GetAnnotations(),
			OwnerReferences: lg.GetOwnerReferences(),
		},
		Spec: lvmv1alpha1.VolumePathSpec{
			Type:          v1alpha1.RemoteSource,
			Node:          lg.Spec.Hostname,
			Vendor:        lg.Spec.Vendor,
			VgName:        lg.Spec.PVName,
			FsType:        lg.Spec.FsType,
			Size:          lg.Spec.Capacity,
			LunIDs:        ids,
			InitiatorType: host.Spec.San.Initiator.Type,
		},
	}, metav1.CreateOptions{})
}

func (ctrl *Controller) waitForVolumePathClean(lg *v1alpha1.Lungroup) error {
	vp, err := ctrl.lvmclient.LvmV1alpha1().VolumePaths().Get(context.TODO(), lg.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	soucreNode, err := ctrl.kubeClient.CoreV1().Nodes().Get(context.TODO(), vp.Status.BindingNode, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if soucreNode.Status.Phase != corev1.NodeRunning {
		return fmt.Errorf("%s node not running.phase:%s", soucreNode.Status.Phase)
	}

	err = wait.PollImmediate(time.Second*5, 3*60*time.Second, func() (bool, error) {

		return true, nil
	})
	return nil
}

func (ctrl *Controller) migrateLungroup(lg *v1alpha1.Lungroup) error {
	if !lg.Spec.Action.Migrate.Force {
		return fmt.Errorf("now just support force migrate")
	}

	vp, err := ctrl.lvmlister.Get(lg.Name)
	if err != nil {
		return err
	}

	runner, err := ctrl.runnerBySan(lg.Spec.San)
	if err != nil {
		return err
	}

	deshost, err := ctrl.hostByName(lg.Spec.Action.Migrate.Node)
	if err != nil {
		return err
	}

	if lg.Status.MappingView != nil && lg.Status.MappingView.Name != deshost.Name {
		err = runner.delMappingView(lg)
		if err != nil {
			return err
		}
	}

	err = runner.createMappingView(lg, deshost.Spec.San.HostGroup, deshost.Name)
	if err != nil {
		return err
	}

	//update vp

	updatevp := vp.DeepCopy()
	if updatevp.Spec.ForceMigarete != true || updatevp.Spec.Node != lg.Spec.Action.Migrate.Node {
		updatevp.Spec.ForceMigarete = true
		updatevp.Spec.Node = lg.Spec.Action.Migrate.Node
		vp, err = ctrl.lvmclient.LvmV1alpha1().VolumePaths().Update(context.TODO(), vp, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		ctrl.recorder.Eventf(lg, corev1.EventTypeNormal, "migrate", "update vp node %s->%s,ForceMigarete=true", vp.Spec.Node, updatevp.Spec.Node)
		return nil
	}

	if vp.Status.Status != lvmv1alpha1.VpBinding {
		ctrl.recorder.Eventf(lg, corev1.EventTypeNormal, "migrate", "wait for volumepath be binding(current status:%s)", vp.Status.Status)
		return nil
	}

	lg, err = ctrl.updateLungroupStatusPhase(lg, v1alpha1.LunGroupReady)
	if err != nil {
		return err
	}

	lg.Spec.Action.Migrate = nil
	_, err = ctrl.clientset.SanV1alpha1().Lungroups().Update(context.TODO(), lg, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}

// 删除vp:
// 1. 设置node字段为空
// 2.等待vp: deactivated
// 3.删除lungroup
// 4.设置DeleteCode
func (ctrl *Controller) deteleLungroup(lg *v1alpha1.Lungroup) error {
	var err error

	if lg.Status.Phase != v1alpha1.LungroupDeleteing {
		lg, err = ctrl.updateLungroupStatusPhase(lg, v1alpha1.LungroupDeleteing)
		if err != nil {
			return err
		}
	}

	vp, err := ctrl.lvmlister.Get(lg.Name)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	if err == nil && vp != nil {
		if vp.Spec.Node != "" {

			vp := vp.DeepCopy()
			vp.Spec.Node = ""

			vp, err = ctrl.lvmclient.LvmV1alpha1().VolumePaths().Update(context.TODO(), vp, metav1.UpdateOptions{})
			if err != nil {
				ctrl.recorder.Eventf(lg, corev1.EventTypeWarning, "update vp fail", "set %s vp.spec.node emtpy fail:%s", vp.Name, err.Error())
				return err
			}

			ctrl.recorder.Eventf(lg, corev1.EventTypeNormal, "trigger vp deactive", "set %s vp.spec.node emtpy ok", vp.Name)
			return nil
		}

		if vp.Status.Status != lvmv1alpha1.VpDeactivated {
			ctrl.recorder.Eventf(lg, corev1.EventTypeNormal, "wait for VpDeactivated", "wait %s for VpDeactivated", vp.Name)
			return nil
		}
	}

	runner, err := ctrl.runnerBySan(lg.Spec.San)
	if err != nil {
		return err
	}

	err = runner.deleteLungroupCmd(lg)
	if err != nil {
		return err
	}

	if vp != nil {
		err = ctrl.deleteVolumePath(vp)
		return err
	}

	err = ctrl.clientset.SanV1alpha1().Lungroups().Delete(context.TODO(), lg.Name, metav1.DeleteOptions{})
	if errors.IsNotFound(err) {
		return nil
	}

	return err
}

func (ctrl *Controller) deleteVolumePath(vp *lvmv1alpha1.VolumePath) error {
	if vp.Spec.ActCode == lvmv1alpha1.DeleteCode {
		return fmt.Errorf("wait for vp deleted")
	}

	clone := vp.DeepCopy()
	clone.Spec.Node = ""
	clone.Spec.ActCode = lvmv1alpha1.DeleteCode

	_, err := ctrl.lvmclient.LvmV1alpha1().VolumePaths().Update(context.TODO(), clone, metav1.UpdateOptions{})

	return err
}

package v1alpha1

import (
	"context"
	"fmt"

	hostv1 "github.com/upmio/dbscale-kube/pkg/apis/host/v1alpha1"
	"github.com/upmio/dbscale-kube/pkg/apis/san/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	hostctrl "github.com/upmio/dbscale-kube/cluster_engine/host/controller/v1alpha1"
)

// hostWorker processes items from hostQueue. It must run only once,
// syncHost is not assured to be reentrant.
func (ctrl *Controller) hostWorker() {
	workFunc := func() bool {
		keyObj, quit := ctrl.hostQueue.Get()
		if quit {
			return true
		}
		defer ctrl.hostQueue.Done(keyObj)

		err := func() error {

			// We expect strings to come off the workqueue. These are of the
			// form namespace/name. We do this as the delayed nature of the
			// workqueue means the items in the informer cache may actually be
			// more up to date that when the item was initially put onto the
			// workqueue.
			key, ok := keyObj.(string)
			if !ok {
				// As the item in the workqueue is actually invalid, we call
				// Forget here else we'd go into a loop of attempting to
				// process a work item that is invalid.
				ctrl.hostQueue.Forget(keyObj)

				return fmt.Errorf("expected string in workqueue but got %#v", keyObj)
			}

			klog.V(5).Infof("hostWorker[%s]", key)

			_, name, err := cache.SplitMetaNamespaceKey(key)
			if err != nil {
				return fmt.Errorf("error getting name of host %q to get host from informer: %v", key, err)
			}

			host, err := ctrl.hostLister.Get(name)
			if err == nil {

				// The host still exists in informer cache, the event must have
				// been add/update/sync
				err = ctrl.syncHost(host)
				if err != nil {
					ctrl.recorder.Eventf(host, corev1.EventTypeWarning, failedSync, messageResourceSyncFailed, key, err)

					return fmt.Errorf("error getting name of san %q to sync san: %v", key, err)
				}

				//ctrl.recorder.Event(host, corev1.EventTypeNormal, successSynced, messageResourceSynced)

				return nil
			}
			if !errors.IsNotFound(err) {
				return fmt.Errorf("error getting host %q from informer: %v", key, err)
			}

			// The host is not in informer cache, the event must have been "delete"
			obj, found, err := ctrl.hostStore.GetByKey(key)
			if err != nil {
				klog.V(2).Infof("error getting host %q from cache: %v", key, err)
				return err
			}
			if !found {
				// The controller has already processed the delete event and
				// deleted the claim from its cache
				klog.V(2).Infof("deletion of host %q was already processed", key)
				return nil
			}
			host, ok = obj.(*hostv1.Host)
			if !ok {
				klog.Errorf("expected host, got %+v", obj)
				return nil
			}
			ctrl.recorder.Event(host, corev1.EventTypeWarning, "not find host", "should not happen here")
			klog.Warning(":%s:should not happen there.not find host", host.GetName())
			return nil
			// return ctrl.deleteHost(host)
		}()

		if err == nil {
			klog.Infof("hostgroupWorker successfully synced '%s'", keyObj)
		} else {
			klog.Error("hostgroupWorker  %s fail:%s", keyObj, err.Error())
		}

		ctrl.hostQueue.Forget(keyObj)

		//if err == nil {
		//	ctrl.hostQueue.Forget(keyObj)
		//} else {
		//	runtime.HandleError(err)
		//
		//	if ctrl.hostQueue.NumRequeues(keyObj) < maxRetries {
		//		ctrl.hostQueue.AddRateLimited(keyObj)
		//	} else {
		//		ctrl.hostQueue.Forget(keyObj)
		//	}
		//}

		return false
	}
	for {
		if quit := workFunc(); quit {
			klog.Infof("pod worker queue shutting down")
			return
		}
	}
}

func (c *Controller) updateHostPhaseStatus(host *hostv1.Host, phase hostv1.HostPhase) error {
	updateHost := host.DeepCopy()
	updateHost.Status.Phase = phase

	var _, err = c.hostClient.HostV1alpha1().Hosts().UpdateStatus(context.TODO(), updateHost, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (ctrl *Controller) syncHost(host *hostv1.Host) error {
	if host.GetDeletionTimestamp() != nil {
		return ctrl.deleteHost(host)
	}

	if hostctrl.NeedSanUnregister(host) {
		err := ctrl.deleteHost(host)
		if err != nil {
			return err
		}
		err = ctrl.updateHostPhaseStatus(host, hostv1.SanUnRegistered)
		return err
	}

	if hostctrl.NeedSanRegister(host) {
		err := ctrl.addHost(host)
		if err != nil {
			return err
		}

		err = ctrl.updateHostPhaseStatus(host, hostv1.SanRegistered)
		return err
	}

	// node, err := ctrl.kubeClient.CoreV1().Nodes().Get(host.Name, metav1.GetOptions{})
	// if err != nil {
	// 	return err
	// }
	// if !isNodeReady(node) {
	// 	return fmt.Errorf("Node %s not ready,conditions:%s", node.Name, node.Status.Conditions)
	// }

	return nil
}

func (ctrl *Controller) addHost(host *hostv1.Host) error {
	runner, err := ctrl.runnerFromHost(*host)
	if err != nil {
		return err
	}

	err = runner.addHostCmd(host)
	if err == nil {
		ctrl.hostStore.Add(host)
	}

	return err
}

// isNodeReady returns true if a node is ready; false otherwise.
func isNodeReady(node *corev1.Node) bool {
	for _, c := range node.Status.Conditions {
		if c.Type == corev1.NodeReady {
			return c.Status == corev1.ConditionTrue
		}
	}

	return false
}

func (ctrl *Controller) hostByName(hostname string) (*hostv1.Host, error) {

	return ctrl.hostLister.Get(hostname)
}

func (ctrl *Controller) runnerByHostname(hostname string) (sanRunner, error) {
	host, err := ctrl.hostByName(hostname)
	if err != nil {
		return nil, err
	}

	return ctrl.runnerFromHost(*host)
}

func (ctrl *Controller) runnerFromHost(host hostv1.Host) (sanRunner, error) {
	if host.Spec.San == nil {
		return nil, fmt.Errorf("Host %s not support SanSystem", host.Name)
	}

	return ctrl.runnerBySan(host.Spec.San.San)
}

func (ctrl *Controller) deleteHost(host *hostv1.Host) error {
	if host.Spec.San == nil {
		return nil
	}

	runner, err := ctrl.runnerFromHost(*host)
	if err != nil {
		return err
	}

	err = runner.delHostCmd(host.Spec.San.HostGroup, host.Name)
	if err == nil {
		ctrl.hostStore.Delete(host)
	} else {
		ctrl.recorder.Eventf(host, corev1.EventTypeWarning, "Delete Host Failed", messageResourceSyncFailed, host.Name, err)
	}

	return err
}

func HostLocalVG(host *hostv1.Host, claim *corev1.PersistentVolumeClaim) (string, error) {
	list := []v1alpha1.Level{v1alpha1.MediumPerformance, v1alpha1.HighPerformance, v1alpha1.LowPerformance}

	if claim != nil {
		typ, ok := v1alpha1.VGSourcePerformance(claim)
		if ok {
			list = []v1alpha1.Level{typ}
		}
	}

	for _, typ := range list {
		for _, vg := range host.Spec.LocalVGs {
			if v1alpha1.Level(vg.Level) == typ {

				return vg.Name, nil
			}
		}
	}

	return "", fmt.Errorf("Not found VG %s in host %s", list, host.Name)
}

package v1alpha1

import (
	"context"
	"fmt"
	"strconv"
	"time"

	hostv1 "github.com/upmio/dbscale-kube/pkg/apis/host/v1alpha1"
	"github.com/upmio/dbscale-kube/pkg/apis/san/v1alpha1"
	clientset "github.com/upmio/dbscale-kube/pkg/client/san/v1alpha1/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
)

// sanWorker processes items from  sanQueue. It must run only once,
// syncSan is not assured to be reentrant.
func (ctrl *Controller) sanWorker() {
	workFunc := func() bool {
		keyObj, quit := ctrl.sanQueue.Get()
		if quit {
			return true
		}
		defer ctrl.sanQueue.Done(keyObj)

		err := func() error {

			key, ok := keyObj.(string)
			if !ok {
				// As the item in the sanQueue is actually invalid, we call
				// Forget here else we'd go into a loop of attempting to
				// process a work item that is invalid.
				ctrl.sanQueue.Forget(keyObj)

				return fmt.Errorf("expected string in sanQueue but got %#v", keyObj)
			}

			klog.V(5).Infof("sanWorker[%s]", key)

			_, name, err := cache.SplitMetaNamespaceKey(key)
			if err != nil {
				return fmt.Errorf("error getting name of san %q to get san from informer: %v", key, err)
			}

			san, err := ctrl.sanLister.Get(name)
			if err == nil {
				// The san still exists in informer cache, the event must have
				// been add/update/sync
				err = ctrl.syncSan(san)
				if err != nil {
					ctrl.recorder.Eventf(san, corev1.EventTypeWarning, failedSync, messageResourceSyncFailed, key, err)
					return fmt.Errorf("error getting name of san %q to sync san: %v", key, err)
				}

				ctrl.recorder.Event(san, corev1.EventTypeNormal, successSynced, messageResourceSynced)
				return nil
			}

			if errors.IsNotFound(err) {
				return nil
			}

			return fmt.Errorf("error getting san %q from informer: %v", key, err)
		}()

		if err == nil {
			klog.Infof("sanWorker successfully synced '%s'", keyObj)
		} else {
			klog.Error("sanWorker  %s fail:%s", keyObj, err.Error())
		}

		ctrl.sanQueue.Forget(keyObj)
		//if err == nil {
		//	ctrl.sanQueue.Forget(keyObj)
		//	klog.Infof("sanWorker successfully synced '%s'", keyObj)
		//} else {
		//	runtime.HandleError(err)
		//
		//	if ctrl.sanQueue.NumRequeues(keyObj) < maxRetries {
		//		ctrl.sanQueue.AddRateLimited(keyObj)
		//	} else {
		//		ctrl.sanQueue.Forget(keyObj)
		//	}
		//}

		return false
	}
	for {
		if quit := workFunc(); quit {
			klog.Infof("san worker queue shutting down")
			return
		}
	}
}

func (ctrl *Controller) syncAllSan() {
	sans, err := ctrl.sanLister.List(labels.Everything())
	if err != nil {
		klog.Errorf("syncAllSan list sans fail:%s", err.Error())
		return
	}

	for _, san := range sans {
		hs, err := newSanRunner(ctrl.key, ctrl.script, san, ctrl.clientset, ctrl.recorder)
		if err != nil {
			ctrl.recorder.Eventf(san, corev1.EventTypeWarning, "force-sync", "get newSanRunner fail:%s", err.Error())
			continue
		}
		_, err = hs.SyncInfo()
		if err != nil {
			ctrl.recorder.Eventf(san, corev1.EventTypeWarning, "force-sync", "sync fail:%s", err.Error())
		}
	}
}

func (ctrl *Controller) syncSan(san *v1alpha1.SanSystem) error {
	if san.Status.Connected || san.GetDeletionTimestamp() != nil {
		return nil
	}

	_, err := newSanRunner(ctrl.key, ctrl.script, san, ctrl.clientset, ctrl.recorder)
	if err != nil {
		clone := san.DeepCopy()
		clone.Status.Connected = false
		_, _err := ctrl.clientset.SanV1alpha1().SanSystems().UpdateStatus(context.TODO(), clone, metav1.UpdateOptions{})
		if _err != nil {
			return _err
		}
	}

	return err
}

func (ctrl *Controller) runnerBySan(name string) (sanRunner, error) {
	san, err := ctrl.sanLister.Get(name)
	if err != nil {
		return nil, err
	}

	if san.Status.Disable {
		return nil, fmt.Errorf("San %s is disabled", san.Name)
	}

	return newSanRunner(ctrl.key, ctrl.script, san, ctrl.clientset, ctrl.recorder)
}

/*
func (ctrl *Controller) createLungroup(pod *corev1.Pod, claim *corev1.PersistentVolumeClaim) error {
	if pod.Spec.NodeName == "" {
		return fmt.Errorf("Pod NodeName is unknown")
	}

	q, err := v1alpha1.PVCStorageQuantity(claim)
	if err != nil {
		return err
	}

	level, _ := v1alpha1.VGSourcePerformance(claim)

	host, err := ctrl.hostByName(pod.Spec.NodeName)
	if err != nil {
		return err
	}

	runner, err := ctrl.runnerFromHost(*host)
	if err != nil {
		return err
	}

	san := runner.SAN()

	lg := &v1alpha1.Lungroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:        claim.Spec.VolumeName,
			Labels:      claim.GetLabels(),
			Annotations: claim.GetAnnotations(),
		},
		Spec: v1alpha1.LungroupSpec{
			Type:     v1alpha1.PVCAllocType(claim), // default thick
			Hostname: pod.Spec.NodeName,
			PVName:   claim.Spec.VolumeName,
			VGName:   claim.Name,
			San:      san.GetName(),
			Vendor:   san.Spec.Auth.Vendor,
			FsType:   v1alpha1.FileSystemType(claim),
			Capacity: q,
			Level:    level,
		},
	}

	lg, err = ctrl.clientset.SanV1alpha1().Lungroups().Create(lg)
	if err == nil {
		ctrl.recorder.Event(lg, corev1.EventTypeNormal, "Lungroup Creating", "")
	}

	return err
}
*/
type storageOperator interface {
	Info() (*v1alpha1.SanSystem, error)
	SAN() *v1alpha1.SanSystem
	storagePoolInfoCmd(pool string) ([]v1alpha1.StoragePool, error)

	addHostCmd(host hostv1.Host) error
	delHostCmd(group string, hosts ...string) error
	listHostgroupCmd(group string) (v1alpha1.HostgroupInfo, error)

	listLungroupCmd(group string) (v1alpha1.LungroupInfo, error)
	createLunsCmd(lr lunRequest) error
	expandLunsCmd(lr lunRequest) error
	delLunCmd(lr v1alpha1.Lungroup) error

	createMappingView(lungroup, hostgroup string) error
	delMappingView(lungroup, hostgroup string) error
}

type sanRunner interface {
	SAN() *v1alpha1.SanSystem
	SyncInfo() (*v1alpha1.SanSystem, error)

	addHostCmd(host *hostv1.Host) error
	delHostCmd(group string, hosts ...string) error

	syncLungroupCapacityCmd(lg *v1alpha1.Lungroup) error
	deleteLungroupCmd(lg *v1alpha1.Lungroup) error

	createMappingView(lg *v1alpha1.Lungroup, hostgroup, hostname string) error
	delMappingView(lg *v1alpha1.Lungroup) error
}

func newSanRunner(key, script string, san *v1alpha1.SanSystem,
	client clientset.Interface, recorder record.EventRecorder) (sanRunner, error) {
	san = san.DeepCopy()
	hw := newHUAWEI(key, script, san, recorder)

	if hw.Status.Connected {
		return &huaweiSan{
			san:      hw,
			client:   client,
			recorder: recorder,
		}, nil
	}

	hs := &huaweiSan{
		san:      newHUAWEI(key, script, san, recorder),
		client:   client,
		recorder: recorder,
	}

	san, err := hs.SyncInfo()
	if err != nil {
		return nil, err
	}

	return &huaweiSan{
		san:      newHUAWEI(key, script, san, recorder),
		client:   client,
		recorder: recorder,
	}, nil

	//	return nil, fmt.Errorf("vendor %s isnot supported yet", san.Spec.Auth.Vendor)
}

type huaweiSan struct {
	san      storageOperator
	client   clientset.Interface
	recorder record.EventRecorder
}

func (hs huaweiSan) SAN() *v1alpha1.SanSystem {
	return hs.san.SAN()
}

func (hs *huaweiSan) SyncInfo() (*v1alpha1.SanSystem, error) {
	clone, err := hs.san.Info()

	san, _err := hs.client.SanV1alpha1().SanSystems().UpdateStatus(context.TODO(), clone, metav1.UpdateOptions{})
	if err == nil {
		return san, _err
	}

	return clone, err
}

func (hs *huaweiSan) addHostCmd(host *hostv1.Host) error {

	err := hs.san.addHostCmd(*host)
	if err != nil {
		hs.recorder.Eventf(host, corev1.EventTypeWarning, cmdFailed, messageActionFailed, "addHostCmd", err)
	}

	return err
}

func (hs *huaweiSan) delHostCmd(group string, hosts ...string) error {
	return hs.san.delHostCmd(group, hosts...)
}

func (hs *huaweiSan) syncLungroupCmd(lg *v1alpha1.Lungroup) error {
	info, err := hs.san.listLungroupCmd(lg.Name)
	if err != nil {
		hs.recorder.Eventf(lg, corev1.EventTypeWarning, cmdFailed, messageActionFailed, "listLungroupCmd", err)
		return err
	}

	if lg.Update(info) {
		clone, err := hs.client.SanV1alpha1().Lungroups().UpdateStatus(context.TODO(), lg, metav1.UpdateOptions{})
		if err == nil && clone != nil {
			clone.DeepCopyInto(lg)
		}

		return err
	}

	return nil
}

func (hs *huaweiSan) syncLungroupCapacityCmd(lg *v1alpha1.Lungroup) error {
	defer func(start time.Time) {
		klog.Info("syncLungroupCapacityCmd", time.Since(start))
	}(time.Now())

	request := lg.Capacity()

	/*	if lg.Spec.Hostname == "" ||
		lg.Spec.MigrateProcess ||
		(lg.Status.MappingView != nil &&
			lg.Spec.Hostname == lg.Status.MappingView.Name &&
			request <= lg.Size()) {

		// skip sync lungroup with san system
		return nil
	}*/

	err := hs.syncLungroupCmd(lg)
	if err != nil {
		return err
	}

	// create & expand lungroup
	if n := request - lg.CurSize(); n > 0 {
		err = hs.createLungroupCmd(lg, n)
		if err != nil {
			return err
		}

		err = hs.syncLungroupCmd(lg)
		if err != nil {
			return err
		}
	}

	return err
}

func selectPool(pools []v1alpha1.StoragePool, list []v1alpha1.StoragePoolWithLevel, level v1alpha1.Level) v1alpha1.StoragePool {
	max := v1alpha1.StoragePool{}

loop:
	for i := range list {

		if level != v1alpha1.NonePerformance && level != list[i].Level {
			continue loop
		}

		for l := range pools {
			if pools[l].Name != list[i].Name && pools[l].ID != list[i].Name {
				continue
			}

			if max.Free < pools[l].Free {
				max = pools[l]
			}

			continue loop
		}
	}

	return max
}

func (hs *huaweiSan) createLungroupCmd(lg *v1alpha1.Lungroup, request int64) error {
	if request <= 0 {
		return nil
	}

	pools, err := hs.san.storagePoolInfoCmd("")
	if err != nil {
		return err
	}

	san := hs.SAN()
	max := selectPool(pools, san.Spec.StoragePoolList, lg.Spec.Level)
	if max.Free < request {
		return fmt.Errorf("SAN %s/%s hasn't enough space for more allocation,%d<%d", san.Namespace, san.Name, max.Free, request)
	}

	num := len(lg.Status.Luns)
	lr := lunRequest{}
	lr.Data.Group = lg.Name
	lr.Data.Type = lg.Spec.Type
	lr.Data.Luns = []v1alpha1.Lun{
		{
			Name:        strconv.Itoa(num),
			StoragePool: max.Name,
			Capacity:    request,
		}}

	if num == 0 {
		err = hs.san.createLunsCmd(lr)
	} else {
		err = hs.san.expandLunsCmd(lr)
	}
	if err != nil {
		hs.recorder.Eventf(lg, corev1.EventTypeWarning, cmdFailed, messageActionFailed, "createLungroupCmd", err)
	} else {
		hs.recorder.Eventf(lg, corev1.EventTypeNormal, successSynced, "createLungroupCmd: %v", lr.Data.Luns)
		// refresh san pools
		//_, err = hs.SyncInfo()
	}

	return err
}

func (hs *huaweiSan) deleteLungroupCmd(lg *v1alpha1.Lungroup) (err error) {
	update := false
	lg = lg.DeepCopy()

	defer func() {
		if update {
			_err := hs.syncLungroupCmd(lg)
			if _err != nil {
				err = _err
			}
		}

		if errors.IsNotFound(err) {
			err = nil
		}
	}()

	err = hs.syncLungroupCmd(lg)
	if err != nil {
		return err
	}

	if lg.Status.MappingView != nil {
		hs.recorder.Eventf(lg, corev1.EventTypeNormal, "delMappingView", "start delMappingView")
		err = hs.san.delMappingView(lg.Name, lg.Status.MappingView.HostGroup)
		if err != nil {
			hs.recorder.Eventf(lg, corev1.EventTypeWarning, cmdFailed, messageActionFailed, "deleteLungroupMappingCmd", err)
			return err
		}

		update = true
		lg.Status.MappingView = nil
		hs.recorder.Eventf(lg, corev1.EventTypeNormal, "delMappingView", " delMappingView ok")
	}

	if len(lg.Status.Luns) > 0 {
		hs.recorder.Eventf(lg, corev1.EventTypeNormal, "delLun", "start delLun")
		err = hs.san.delLunCmd(*lg)
		if err != nil {
			hs.recorder.Eventf(lg, corev1.EventTypeWarning, cmdFailed, messageActionFailed, "deleteLungroupCmd", err)
			return err
		}

		update = true
		lg.Status.Luns = nil
		hs.recorder.Eventf(lg, corev1.EventTypeNormal, "delLun", "delLun ok")

	}

	/*	if lg.Status.MappingView == nil && len(lg.Status.Luns) == 0 {
		err = hs.client.SanV1alpha1().Lungroups().Delete(lg.Name, &metav1.DeleteOptions{})
		if err == nil || errors.IsNotFound(err) {
			update = false
		}
	}*/

	return err
}

func (hs *huaweiSan) createMappingView(lg *v1alpha1.Lungroup, hostgroup, hostname string) error {

	if lg.Status.MappingView != nil &&
		lg.Status.MappingView.HostGroup == hostgroup &&
		lg.Status.MappingView.Name == hostname {
		return nil
	}

	hs.recorder.Eventf(lg, corev1.EventTypeNormal, "createMappingView", "start createMappingView")
	err := hs.san.createMappingView(lg.Name, hostgroup)
	if err != nil {

		hs.recorder.Eventf(lg, corev1.EventTypeWarning, cmdFailed, messageActionFailed, "createMappingView", err)
		return err
	}

	lg.Status.MappingView = &v1alpha1.MappingView{
		Name:      hostname,
		HostGroup: hostgroup,
	}

	clone, err := hs.client.SanV1alpha1().Lungroups().UpdateStatus(context.TODO(), lg, metav1.UpdateOptions{})
	if err == nil {
		clone.DeepCopyInto(lg)
	}

	hs.recorder.Eventf(lg, corev1.EventTypeNormal, "createMappingView", "createMappingView ok")

	return err
}

func (hs *huaweiSan) delMappingView(lg *v1alpha1.Lungroup) error {
	if lg.Status.MappingView == nil {
		return nil
	}

	err := hs.san.delMappingView(lg.Name, lg.Status.MappingView.HostGroup)
	if err != nil {
		hs.recorder.Eventf(lg, corev1.EventTypeWarning, cmdFailed, messageActionFailed, "delMappingView", err)
		return err
	}

	lg.Status.MappingView = nil
	clone, err := hs.client.SanV1alpha1().Lungroups().UpdateStatus(context.TODO(), lg, metav1.UpdateOptions{})
	if err == nil && clone != nil {
		clone.DeepCopyInto(lg)
	}
	hs.recorder.Eventf(lg, corev1.EventTypeNormal, "delMappingView", "delMappingView ok")
	return err
}

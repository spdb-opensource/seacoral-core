package v1alpha1

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/api/resource"
	"path/filepath"

	stderrors "errors"
	"time"

	"k8s.io/klog/v2"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"

	"k8s.io/client-go/tools/cache"

	vpv1 "github.com/upmio/dbscale-kube/pkg/apis/volumepath/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	// "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	VPShellFile            = "VPMGR/VPMGR"
	checkMountIntervalTime = 150 * time.Second
)

func (c *Controller) volumePathHandler(key string) error {

	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}
	vp, err := c.VolumePathLister.Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	if vp.Status.BindingNode != c.HostName && vp.Spec.Node != c.HostName {
		return nil
	}
	klog.V(4).Infof("volumePathHandler key:%s ,the VolumePath spec:%v,status:%v", key, vp.Spec, vp.Status)

	vpexecfile := filepath.Join(c.ShellDir, VPShellFile)
	//强制同步status跟spec一致
	if vp.Spec.ActCode == vpv1.CopySpecToStatusCode &&
		vp.Spec.Node == c.HostName {
		klog.V(2).Infof("start  sync status %s", key)

		//cfg := generateCommonActCfg(vp)
		device := vp.Spec.VgName + "/" + vp.GetName()
		vp, err = c.updateVp(vp, vpv1.VpBinding, c.HostName, vp.Spec.LunIDs, vp.Spec.Size, device, "")
		if err != nil {
			c.recorder.Eventf(vp, corev1.EventTypeWarning, "syncStatusFail", "syncStatusFail:", err.Error())
			return err
		}

		vp, err = c.updateVpReset(vp, vpv1.VpBinding, 0)
		if err != nil {
			c.recorder.Eventf(vp, corev1.EventTypeWarning, "syncStatusFail", "syncStatusFail:", err.Error())
			return err
		}

		c.recorder.Event(vp, corev1.EventTypeNormal, "syncStatusOK", "syncStatusOK")

		return nil

	}
	//状态置位"" 或 binding
	if vp.Status.BindingNode == c.HostName && vp.Spec.ActCode == vpv1.ResetStatusCode {

		status := vp.Status.Status

		switch vp.Status.Status {
		case vpv1.VpCreateFail:
			status = vpv1.VpEmtpy
		case vpv1.VpActivatFail:
			status = vpv1.VpDeactivated
		case vpv1.VpRemovFail:
			status = vpv1.VpDeactivated
		case vpv1.VpDeactivatFail:
			status = vpv1.VpBinding
		case vpv1.VpExtendFail:
			status = vpv1.VpBinding
		}

		//		if vp.Status.Status == vpv1.VpCreateFail {
		//			status = vpv1.VpEmtpy
		//		} else if vp.Status.Status == vpv1.VpActivatFail || vp.Status.Status == vpv1.VpRemovFail {
		//			status = vpv1.VpDeactivated
		//		} else if vp.Status.Status == vpv1.VpDeactivatFail {

		//		} else if vp.Status.Status == vpv1.VpExtendFail {
		//			status = vpv1.VpBinding
		//		}

		_, err := c.updateVpReset(vp, status, 0)
		if err != nil {
			c.recorder.Event(vp, corev1.EventTypeWarning, "resetStatus", err.Error())
			return err
		}

		c.recorder.Eventf(vp, corev1.EventTypeNormal, "resetStatus", "old status %s", vp.Status.Status)
		return nil
	}
	//扩容流程
	if vp.Status.BindingNode == c.HostName &&
		vp.Status.BindingNode == vp.Spec.Node &&
		vp.Spec.Size.Cmp(vp.Status.CurSize) > 0 &&
		(vp.Status.Status == vpv1.VpExtending || vp.Status.Status == vpv1.VpBinding) {

		if vp.Spec.Type == "remote" {
			if len(vp.Status.LunIDs) >= len(vp.Spec.LunIDs) {
				return stderrors.New("bad spec config: len(vp.Status.LunIDs) >= len(vp.Spec.LunIDs)")
			}
		}

		if vp.Status.Status != vpv1.VpExtending {
			vp, err = c.updateVpStatus(vp, vpv1.VpExtending)
			if err != nil {
				return err
			}

			return nil
		}

		extendIds := []string{}
		if vp.Spec.Type == "remote" {
			for _, specId := range vp.Spec.LunIDs {

				find := false
				for _, statusId := range vp.Status.LunIDs {
					if specId == statusId {
						find = true
						break
					}
				}

				if !find {
					extendIds = append(extendIds, specId)
				}

			}

		}

		klog.V(2).Infof("key:%s starting extend ；spec:%v,status:%v", key, vp.Spec, vp.Status)
		c.recorder.Event(vp, corev1.EventTypeNormal, "Extending", "start Extending")
		cfg := generateCommonActCfg(vp)
		cfg.Vg.ExtendLunIDs = extendIds

		if err := expandVP(vpexecfile, cfg); err != nil {
			c.recorder.Event(vp, corev1.EventTypeWarning, string(vpv1.VpExtendFail), err.Error())
			c.updateVpStatus(vp, vpv1.VpExtendFail)
			return err
		}

		// if err := c.updateHostPathVolume(vp.GetName(), cfg.Size, cfg.MounterDir); err != nil {
		// 	c.recorder.Event(vp, corev1.EventTypeWarning, "ExtendRelatedPVFail", err.Error())
		// 	klog.V(1).Infof("updateHostPathVolume fail %s", err.Error())
		// }

		vp, err = c.updateVp(vp, vpv1.VpBinding, vp.Status.BindingNode, vp.Spec.LunIDs, vp.Spec.Size, vp.Status.Deivce, vp.Status.MouterPath)
		if err != nil {
			return err
		}

		c.recorder.Event(vp, corev1.EventTypeNormal, "extended", "extend ok")

		return nil
	}
	//创建流程
	if (vp.Status.BindingNode == "" && vp.Spec.Node == c.HostName && vp.Status.Status == "") ||
		(vp.Status.BindingNode == c.HostName && (vp.Status.Status == "" || vp.Status.Status == vpv1.VpCreating)) {

		if vp.Status.Status != vpv1.VpCreating {
			vp, err = c.updateVp(vp, vpv1.VpCreating, c.HostName, vp.Status.LunIDs, vp.Status.CurSize, vp.Status.Deivce, vp.Status.MouterPath)
			if err != nil {
				return err
			}

			return nil
		}

		klog.V(2).Infof("key:%s starting createSanVg ；spec:%v,status:%v", key, vp.Spec, vp.Status)
		c.recorder.Event(vp, corev1.EventTypeNormal, "Creating", "start creating")

		//create pv
		cfg := generateCommonActCfg(vp)
		cfg.Vg.LunIDs = vp.Spec.LunIDs
		execResult, err := createVP(vpexecfile, cfg)
		if err != nil {
			c.recorder.Event(vp, corev1.EventTypeWarning, string(vpv1.VpCreateFail), err.Error())
			c.updateVpStatus(vp, vpv1.VpCreateFail)
			return err
		}
		//真实device,mounter
		mounterDir := execResult.Mounter
		device := execResult.Device
		// if err := c.updateHostPathVolume(vp.GetName(), cfg.Size, mounterDir); err != nil {
		// 	//			return err
		// 	c.recorder.Event(vp, corev1.EventTypeWarning, "CreateRelatedPVFail", err.Error())
		// 	klog.V(1).Infof("create HostPath Volume fail %s", err.Error())
		// }

		if _, err := c.updateVp(vp, vpv1.VpBinding, c.HostName, vp.Spec.LunIDs, vp.Spec.Size, device, mounterDir); err != nil {
			return err
		}

		c.recorder.Event(vp, corev1.EventTypeNormal, "created", "create ok")

		klog.V(2).Infof("key:%s createSanVp ok", key)
		return nil
	}

	//deActivateSanVp流程
	if vp.Status.BindingNode == c.HostName &&
		vp.Status.BindingNode != vp.Spec.Node &&
		!vp.Spec.ForceMigarete &&
		(vp.Status.Status == vpv1.VpBinding ||
			vp.Status.Status == vpv1.VpDeactivating ||
			vp.Status.Status == vpv1.VpCreateFail) {

		if vp.Spec.Type == "local" {
			klog.V(2).Infof("trigger   deActivate action  key:%s the vp is local type(spec:%v).do nothing", key, vp.Spec)
			return nil
		}

		if len(vp.Status.LunIDs) != len(vp.Spec.LunIDs) {
			klog.V(2).Infof(" warn : len(vp.Status.LunIDs) != len(vp.Spec.LunIDs)")
		}

		if vp.Status.Status != vpv1.VpDeactivating {
			vp, err = c.updateVpStatus(vp, vpv1.VpDeactivating)
			if err != nil {
				return err
			}

			return nil
		}
		time.Sleep(3 * time.Second)
		klog.V(2).Infof("key:%s sleep 3s && starting deActivateSanVp ；spec:%v,status:%v", key, vp.Spec, vp.Status)
		c.recorder.Event(vp, corev1.EventTypeNormal, "Deactivating", "start deActivating")

		cfg := generateCommonActCfg(vp)
		if len(vp.Status.LunIDs) == 0 {
			cfg.Vg.LunIDs = vp.Spec.LunIDs
		} else {
			cfg.Vg.LunIDs = vp.Status.LunIDs
		}
		if err := deActivateVP(vpexecfile, cfg); err != nil {
			c.recorder.Event(vp, corev1.EventTypeWarning, string(vpv1.VpDeactivatFail), err.Error()+" && time sleep 12s")
			time.Sleep(12 * time.Second)
			// if vp.Status.Status != vpv1.VpDeactivatFail {
			// 	c.updateVpStatus(vp, vpv1.VpDeactivatFail)
			// }

			return err
		}

		vp, err := c.updateVpStatus(vp, vpv1.VpDeactivated)
		if err != nil {
			return err
		}

		c.recorder.Event(vp, corev1.EventTypeNormal, "Deactivated", "Deactivated ok")

		return nil

	}

	//删除vg
	if vp.Spec.ActCode == vpv1.DeleteCode {

		// if vp.Spec.BackupType == "remote" && vp.Status.Status == vpv1.VpRemovFail {
		// 	return nil
		// }

		//sleep 3 wait for umount,firsttime.
		defaulttime := 3 * time.Second
		if vp.Status.Status == vpv1.VpRemovFail {
			defaulttime = 5 * time.Second
		}

		klog.V(2).Infof("%s:time sleep %f second and trigger delete action", vp.GetName(), defaulttime.Seconds())
		time.Sleep(defaulttime)

		if vp.Spec.Type == "remote" && vp.Status.Status != vpv1.VpDeactivated {
			err := fmt.Errorf("the vp(%s) is remote but status is not VpDeactivated,ignore delete", key)
			klog.V(2).Infof(err.Error())
			c.recorder.Eventf(vp, corev1.EventTypeWarning, "status is not VpDeactivated", err.Error())
			return nil
		}

		cfg := generateCommonActCfg(vp)
		cfg.Vg.LunIDs = vp.Spec.LunIDs

		c.recorder.Event(vp, corev1.EventTypeNormal, "deleting ", "start deleteing")
		if err := deleteVP(vpexecfile, cfg); err != nil {
			c.recorder.Event(vp, corev1.EventTypeWarning, string(vpv1.VpRemovFail), err.Error())
			c.updateVpStatus(vp, vpv1.VpRemovFail)
			return err
		}

		// if err := c.deleteHostPathVolume(name); err != nil {
		// 	c.recorder.Event(vp, corev1.EventTypeWarning, "remvoeRelatedVPFail", err.Error())
		// 	return err
		// }

		if err := c.VpClientSet.LvmV1alpha1().VolumePaths().Delete(context.TODO(), name, v1.DeleteOptions{}); err != nil {
			return err
		}

		klog.V(2).Infof("graceful delete vp %s ok ", key)

		return nil
	}

	//clean
	if vp.Status.BindingNode == c.HostName &&
		vp.Spec.ActCode == vpv1.CleanCode &&
		vp.Status.Status == vpv1.VpDeactivated {

		cfg := generateCommonActCfg(vp)
		cfg.Vg.LunIDs = vp.Status.LunIDs

		if vp.Spec.Type == "local" {
			klog.V(2).Infof("trigger   clean action : key:%s the vp is local type(spec:%v).do nothing", key, vp.Spec)
			_, err := c.updateVpReset(vp, vp.Status.Status, 0)
			return err
		}

		c.recorder.Event(vp, corev1.EventTypeNormal, "cleaning", "start cleanning")
		if err := cleanVP(vpexecfile, cfg); err != nil {
			c.recorder.Event(vp, corev1.EventTypeWarning, "cleanFail", err.Error())
			return err
		}

		_, err := c.updateVpReset(vp, vp.Status.Status, 0)
		if err != nil {
			c.recorder.Eventf(vp, corev1.EventTypeWarning, "cleanFail", "reset actCode fail", err.Error())
			return err
		}

		c.recorder.Event(vp, corev1.EventTypeNormal, "clean", "clean sucess")
		return nil
	}

	//ActivateSanVp流程
	if vp.Spec.Node == c.HostName &&
		(vp.Status.Status == vpv1.VpDeactivated ||
			vp.Status.Status == vpv1.VpActivating ||
			vp.Spec.ForceMigarete) {

		if vp.Spec.Type == "local" {
			klog.V(2).Infof("trigger Activate action  key:%s the vp is local type(spec:%v).do nothing", key, vp.Spec)
			return nil
		}

		if len(vp.Status.LunIDs) != len(vp.Spec.LunIDs) {
			return fmt.Errorf("should not happend : len(vp.Status.LunIDs) != len(vp.Spec.LunIDs)")
		}

		if vp.Status.Status != vpv1.VpActivating {
			vp, err = c.updateVpStatus(vp, vpv1.VpActivating)
			if err != nil {
				return err
			}

			return nil
		}

		klog.V(2).Infof("key:%s starting ActivateSanVp ；spec:%v,status:%v", key, vp.Spec, vp.Status)

		c.recorder.Event(vp, corev1.EventTypeNormal, "Activating", "start Activating")

		cfg := generateCommonActCfg(vp)
		cfg.Vg.LunIDs = vp.Status.LunIDs
		if err := activateVP(vpexecfile, cfg); err != nil {
			c.recorder.Event(vp, corev1.EventTypeWarning, string(vpv1.VpActivatFail), err.Error())

			if vp.Status.Status != vpv1.VpActivatFail {
				c.updateVpStatus(vp, vpv1.VpActivatFail)
			}

			return err
		}

		vp, err := c.updateVp(vp, vpv1.VpBinding, vp.Spec.Node, vp.Spec.LunIDs, vp.Status.CurSize, vp.Status.Deivce, vp.Status.MouterPath)
		if err != nil {
			return err
		}

		c.recorder.Event(vp, corev1.EventTypeNormal, "Activated", "Activated ok")
		klog.V(2).Infof("key:%s  ActivateSanVp ok", key)

		return nil

	}

	// //check and mounter
	// if vp.Status.Status == vpv1.VpBinding && vp.Status.BindingNode == c.HostName && vp.Spec.Node == c.HostName {
	// 	cfg := generateCommonActCfg(vp)
	// 	if err := checkVP(c.ShellFile, cfg); err != nil {
	// 		klog.V(2).Infof("[warn] %s check(mount) fail :%s", key, err.Error())
	// 	}
	// }

	klog.V(4).Infof("key:%s  nothing to do", key)
	return nil
}

func (c *Controller) updateVpBindNode(vp *vpv1.VolumePath, bingNode string) (*vpv1.VolumePath, error) {
	return c.updateVp(vp, vp.Status.Status, bingNode, vp.Status.LunIDs, vp.Status.CurSize, vp.Status.Deivce, vp.Status.MouterPath)
}

func (c *Controller) updateVpReset(vp *vpv1.VolumePath, status vpv1.VpStatus, actCode int64) (*vpv1.VolumePath, error) {
	updateVp := vp.DeepCopy()
	updateVp.Spec.ActCode = actCode

	vp1, err := c.VpClientSet.LvmV1alpha1().VolumePaths().Update(context.TODO(), updateVp, metav1.UpdateOptions{})
	if err != nil {

		klog.V(1).Infof("updateVpReset spec:actCode key:%s fail: %s", vp.GetNamespace()+"/"+vp.GetName(), err.Error())
		return nil, err
	}

	if status == vp.Status.Status {
		return vp1, nil
	}

	newUpdate := vp1.DeepCopy()
	newUpdate.Status.Status = status
	vp2, err := c.VpClientSet.LvmV1alpha1().VolumePaths().UpdateStatus(context.TODO(), newUpdate, metav1.UpdateOptions{})
	if err != nil {
		klog.V(1).Infof("updateVpReset status:status key:%s fail: %s", vp.GetNamespace()+"/"+vp.GetName(), err.Error())
		return nil, err
	}
	return vp2, nil
}

func (c *Controller) updateVpStatus(vp *vpv1.VolumePath, status vpv1.VpStatus) (*vpv1.VolumePath, error) {
	return c.updateVp(vp, status, vp.Status.BindingNode, vp.Status.LunIDs, vp.Status.CurSize, vp.Status.Deivce, vp.Status.MouterPath)
}

func (c *Controller) updateVpMounter(vp *vpv1.VolumePath, mounter string) (*vpv1.VolumePath, error) {
	return c.updateVp(vp, vp.Status.Status, vp.Status.BindingNode, vp.Status.LunIDs, vp.Status.CurSize, vp.Status.Deivce, mounter)
}

func (c *Controller) updateVpLunIDs(vp *vpv1.VolumePath, hostLunID []string) (*vpv1.VolumePath, error) {
	return c.updateVp(vp, vp.Status.Status, vp.Status.BindingNode, hostLunID, vp.Status.CurSize, vp.Status.Deivce, vp.Status.MouterPath)
}

func (c *Controller) updateVp(vp *vpv1.VolumePath, status vpv1.VpStatus, bingNode string, LunIDs []string, tocalsize resource.Quantity, device, mounter string) (*vpv1.VolumePath, error) {
	updateVp := vp.DeepCopy()

	updateVp.Status.Status = status
	updateVp.Status.BindingNode = bingNode

	updateVp.Status.LunIDs = LunIDs

	updateVp.Status.CurSize = tocalsize

	updateVp.Status.MouterPath = mounter
	updateVp.Status.Deivce = device

	newvp, err := c.VpClientSet.LvmV1alpha1().VolumePaths().UpdateStatus(context.TODO(), updateVp, metav1.UpdateOptions{})
	if err != nil {
		klog.V(1).Infof("updateVpStatus  key:%s fail: %s ,(status:%s,bingNode:%s,cursize:%d,LunIDs:%v)",
			vp.GetNamespace()+"/"+vp.GetName(), err.Error(), status, bingNode, tocalsize, LunIDs)
		return nil, err
	}

	return newvp, nil
}

func (c *Controller) deleteVpObjectHandle(obj interface{}) {

	vp, ok := obj.(*vpv1.VolumePath)

	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			runtime.HandleError(fmt.Errorf(" Couldn't get object from tombstone %v", obj))
			return
		}

		vp, ok = tombstone.Obj.(*vpv1.VolumePath)
		if !ok {
			runtime.HandleError(fmt.Errorf(" Tombstone contained object that is not a VolumePath %v", obj))
			return
		}
	}

	if vp.Status.BindingNode != c.HostName || vp.Spec.ActCode == vpv1.DeleteCode {
		return
	}

	klog.V(4).Infof("[use spec.deleteCode better] deleteVpObjectHandle %v(vp:%s)", obj, vp.GetName())

	cfg := generateCommonActCfg(vp)
	cfg.Vg.LunIDs = vp.Status.LunIDs
	vpexecfile := filepath.Join(c.ShellDir, VPShellFile)
	if err := deleteVP(vpexecfile, cfg); err != nil {
		runtime.HandleError(fmt.Errorf(" [use spec.deleteCode better] deleteVpObjectHandle  delete lv fail:%s", err.Error()))
		return
	}

	// if err := c.deleteHostPathVolume(vp.GetName()); err != nil {
	// 	runtime.HandleError(err)
	// 	return
	// }

}

/*

func (c *Controller) deleteHostPathVolume(name string) error {
	_, err := c.kubeclientset.CoreV1().PersistentVolumes().Get(name, v1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
	}

	err = c.kubeclientset.CoreV1().PersistentVolumes().Delete(name, &v1.DeleteOptions{})

	return err

}


func (c *Controller) updateHostPathVolume(name string, size int, path string) error {

	volume, err := c.kubeclientset.CoreV1().PersistentVolumes().Get(name, v1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			hostpathtype := corev1.HostPathDirectory

			pv := &corev1.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "PersistentVolume",
					APIVersion: "v1",
				},
				Spec: corev1.PersistentVolumeSpec{
					//					PersistentVolumeReclaimPolicy: options.PersistentVolumeReclaimPolicy,
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Capacity: corev1.ResourceList{
						corev1.ResourceName(corev1.ResourceStorage): resource.MustParse(fmt.Sprintf("%v%s", size, "Mi")),
					},
					PersistentVolumeSource: corev1.PersistentVolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: path,
							Type: &hostpathtype,
						},
					},
				},
			}

			if _, err := c.kubeclientset.CoreV1().PersistentVolumes().Create(pv); err != nil {
				return err
			}

			return nil

		}

		return fmt.Errorf("get %s hostpath volume fail:%s", name, err.Error())

	}

	if volume.Spec.HostPath == nil {
		return fmt.Errorf("the  %s  volume is not a hostpath volume.", name)

	}

	volume.Spec.HostPath.Path = path
	volume.Spec.Capacity[corev1.ResourceStorage] = resource.MustParse(fmt.Sprintf("%v%s", size, "Mi"))

	if _, err := c.kubeclientset.CoreV1().PersistentVolumes().Update(volume); err != nil {
		return err
	}

	return nil

}
*/

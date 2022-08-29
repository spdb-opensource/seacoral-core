package v1alpha4

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/upmio/dbscale-kube/pkg/structs"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	// "k8s.io/apimachinery/pkg/api/errors"
	// metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	unitv4 "github.com/upmio/dbscale-kube/pkg/apis/unit/v1alpha4"
	executil "github.com/upmio/dbscale-kube/pkg/utils/exec"
	podutil "github.com/upmio/dbscale-kube/pkg/utils/pod"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

var (
	ContainerNotReadyErr = errors.New("container not ready")
)

func (ctrl *Controller) startServer(unit *unitv4.Unit, pod *corev1.Pod) error {
	errs := []error{}
	for _, containerstatus := range pod.Status.ContainerStatuses {
		if !containerstatus.Ready {
			cmd := []string{}
			var err error

			//main container
			if containerstatus.Name == unit.Spec.MainContainerName {
				cmd, err = structs.GetExecCmd(structs.CommonType, structs.ServiceStartCmd)
				if err != nil {
					return err
				}
			} else {
				//sidebar container
				cmd, err = structs.GetExecCmd(structs.CommonType, structs.SideBarServiceStartCmd)
				if err != nil {
					return err
				}
			}

			_, err = ctrl.podExec(pod, containerstatus.Name, cmd)
			if err == nil {
				ctrl.recorder.Eventf(unit, corev1.EventTypeNormal, "start service", "start %s service ok", containerstatus.Name)
			} else {
				errs = append(errs, fmt.Errorf("start %s  container service fail:%s", containerstatus.Name, err.Error()))
			}
		}

	}
	return utilerrors.NewAggregate(errs)
}

func (ctrl *Controller) stopServer(unit *unitv4.Unit, pod *corev1.Pod) error {
	errs := []error{}

	for _, containerstatus := range pod.Status.ContainerStatuses {
		if containerstatus.Ready {
			cmd := []string{}
			var err error
			if containerstatus.Name == unit.Spec.MainContainerName {
				cmd, err = structs.GetExecCmd(structs.CommonType, structs.ServiceStopCmd)
				if err != nil {
					return err
				}
			} else {
				cmd, err = structs.GetExecCmd(structs.CommonType, structs.SideBarServiceStopCmd)
				if err != nil {
					return err
				}
			}

			_, err = ctrl.podExec(pod, containerstatus.Name, cmd)
			if err == nil {
				ctrl.recorder.Eventf(unit, corev1.EventTypeNormal, "stop service", "stop %s service ok", containerstatus.Name)
			} else {
				errs = append(errs, fmt.Errorf("stop %s  container service fail:%s", containerstatus.Name, err.Error()))
			}
		}
	}

	// if len(errs) > 0 {
	// 	ctrl.recorder.Eventf(unit, corev1.EventTypeNormal, "start service", "start some container fails:%s", utilerrors.NewAggregate(errs))
	// }

	return utilerrors.NewAggregate(errs)
}

func (ctrl *Controller) ensureUnitServer(unit *unitv4.Unit) error {
	pod, err := ctrl.podLister.Pods(unit.Namespace).Get(unitv4.GetPodName(unit))
	if err != nil {
		return err
	}

	if !unit.Spec.UnService {
		return ctrl.startServer(unit, pod)
	}

	return ctrl.stopServer(unit, pod)
}

func (ctrl *Controller) UnitExec(unit *unitv4.Unit, containerName string, cmd []string) (string, error) {
	pod, err := ctrl.podLister.Pods(unit.Namespace).Get(unitv4.GetPodName(unit))
	if err != nil {
		return "", err
	}

	return ctrl.podExec(pod, containerName, cmd)
}

func (ctrl *Controller) podExec(pod *corev1.Pod, containerName string, cmd []string) (string, error) {

	if !podutil.IsContainerRunning(pod, containerName) {
		return "", fmt.Errorf("cmd:%s exec fail:%s(pod:%s,container:%s)", cmd, ContainerNotReadyErr, pod.Name, containerName)
	}
	klog.V(4).Infof("%s unit exec cmd:%v", pod.Name, cmd)

	stderr, stdout := bytes.NewBuffer(nil), bytes.NewBuffer(nil)
	ok, err := executil.NewExecInContainer(ctrl.config).Exec(
		executil.NewExecOptions(
			pod.Namespace, pod.Name, containerName, cmd, stderr, stdout))
	if ok && err == nil {
		return stdout.String(), nil
	}
	if ok && err != nil {
		err = fmt.Errorf("Pod %s Container %s exec %s,Output:%s,Stderr:%s,%s", pod.Name, containerName, cmd, stdout.String(), stderr.String(), err)
		ctrl.recorder.Eventf(pod, corev1.EventTypeWarning, "RunInContainer", "Container %s %s,%v", containerName, cmd, err)
	}

	return "", err
}

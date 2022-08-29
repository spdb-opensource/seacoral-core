package main

/*
import (
	"context"
	"fmt"
	"os/exec"

	// "path/filepath"
	"strings"

	// vpact "github.com/upmio/dbscale-kube/cluster_engine/storage/agent/agentController/v1alpha1"
	unitv4 "github.com/upmio/dbscale-kube/pkg/apis/unit/v1alpha4"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
//copy from volumepath project
// VPBaseMounterDir = "/volumepath/k8s/"
)

func CheckvolumePathMount(kubeclient *Kubeclient, podNameSpace, podName string) error {

	pod, err := kubeclient.kubeClient.CoreV1().Pods(podNameSpace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	for _, volume := range pod.Spec.Volumes {

		if volume.PersistentVolumeClaim == nil {
			continue
		}

		pvc, err := kubeclient.kubeClient.CoreV1().PersistentVolumeClaims(podNameSpace).Get(context.TODO(), volume.PersistentVolumeClaim.ClaimName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("check pvc fail: pod(%s) %s get pvc  err:%s", podName, volume.PersistentVolumeClaim.ClaimName, err.Error())
		}

		volumpathName, ok := pvc.Annotations[unitv4.VolumePathAnnotation]
		if !ok {
			continue
		}

		vp, err := kubeclient.vpClient.LvmV1alpha1().VolumePaths().Get(context.TODO(), volumpathName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("check volumepath fail: pod(%s) %s get volumepath  err:%s", podName, volumpathName, err.Error())
		}

		mounter := vp.Status.MouterPath
		if mounter == "" {
			return fmt.Errorf("check volumepath fail:  pod(%s) vp(%s %s):mounter is empty", podName, volumpathName, vp.Status.Deivce)
		}

		if !IsMount(mounter) {
			return fmt.Errorf("check volumepath fail:  pod(%s) vp(%s): not find  mounter  %s", podName, volumpathName, mounter)
		}

		// execfile := filepath.Join(kubeclient.conf.ShellDir, vpact.DefaultsShellFile)
		// mounted, err := vpact.CheckVPMount(execfile, vp)
		// if err != nil {
		// 	return err
		// }

		// if !mounted {
		// 	return fmt.Errorf("check volumepath  pod(%s) vp(%s): not mounted yet", podName, volumpathName)
		// }
	}

	return nil

}

func IsMount(mountpath string) bool {
	script := fmt.Sprintf("cat /proc/mounts | grep %s", mountpath)
	command := exec.Command("/bin/bash", "-c", script)

	out, err := command.Output()
	if err != nil {
		return false
	}

	return strings.Contains(string(out), mountpath)
}
*/

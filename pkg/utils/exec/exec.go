package exec

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	"k8s.io/klog/v2"
	"os/exec"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	typecorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

func CommonShellExec(cmd []string, timeout time.Duration, input, output interface{}) error {
	if input != nil {
		jsonargs, err := json.Marshal(input)
		if err != nil {
			return errors.Wrapf(err, "Marshal fail")
		}
		cmd = append(cmd, fmt.Sprintf("%s", jsonargs))
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	klog.Infof("%v", cmd)

	out, err := exec.CommandContext(ctx, cmd[0], cmd[1:]...).CombinedOutput()
	if err != nil {
		return fmt.Errorf(" exec fail:%s(execfile: %v)(out:%s)", err.Error(), cmd, out)
	}

	if output != nil {
		err = json.Unmarshal(out, output)
		if err != nil {
			return errors.Wrapf(err, fmt.Sprintf("Unmarshal fail(data:%s)", out))
		}
	}

	return nil

}

func NewExecInContainer(config *restclient.Config) ExecInContainer {
	return ExecInContainer{
		config: config,
	}
}

type ExecInContainer struct {
	config *restclient.Config
}

func NewExecOptions(namespace, podName, containerName string, command []string, stderr io.ReadWriter, stdout io.Writer) ExecOptions {
	return ExecOptions{
		Namespace:     namespace,
		PodName:       podName,
		ContainerName: containerName,
		Command:       command,
		Stderr:        stderr,
		Stdout:        stdout,
	}
}

type ExecOptions struct {
	Namespace     string
	PodName       string
	ContainerName string
	Command       []string
	Stdin         io.Reader
	Stderr        io.ReadWriter
	Stdout        io.Writer
}

func (ec ExecInContainer) Exec(opts ExecOptions) (bool, error) {
	client, err := typecorev1.NewForConfig(ec.config)
	if err != nil {
		return false, err
	}

	pod, err := client.Pods(opts.Namespace).Get(context.TODO(), opts.PodName, metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	if pod.Status.Phase == corev1.PodPending || pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
		return false, fmt.Errorf("cannot exec into a container in a completed pod; current phase is %s", pod.Status.Phase)
	}

	containerName := opts.ContainerName
	if len(containerName) == 0 {
		if len(pod.Spec.Containers) > 1 {
			usageString := fmt.Sprintf("Defaulting container name to %s.", pod.Spec.Containers[0].Name)
			fmt.Fprintf(opts.Stderr, "%s\n", usageString)
		}

		containerName = pod.Spec.Containers[0].Name
	}

	const tty = false

	// TODO: consider abstracting into a client invocation or client helper
	req := client.RESTClient().Post().
		Resource("pods").
		Name(pod.Name).
		Namespace(pod.Namespace).
		SubResource("exec").
		Param("container", containerName)
	req.VersionedParams(&corev1.PodExecOptions{
		Container: containerName,
		Command:   opts.Command,
		Stdin:     opts.Stdin != nil,
		Stdout:    opts.Stdout != nil,
		Stderr:    opts.Stderr != nil,
		TTY:       tty,
	}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(ec.config, "POST", req.URL())
	if err != nil {
		return false, fmt.Errorf("error while creating Executor: %v", err)
	}

	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  opts.Stdin,
		Stdout: opts.Stdout,
		Stderr: opts.Stderr,
		Tty:    tty,
	})
	if err != nil {
		var data []byte
		if opts.Stderr != nil {
			data, _ = ioutil.ReadAll(opts.Stderr)
			opts.Stderr.Write(data)
		}
		return true, fmt.Errorf("error in Stream: %v,stderr:%s", err, string(data))
	}

	return true, nil
}

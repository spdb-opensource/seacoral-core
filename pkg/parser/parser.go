package parser

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	netv1 "github.com/upmio/dbscale-kube/pkg/apis/networking/v1alpha1"
	"github.com/upmio/dbscale-kube/pkg/structs"
	podutil "github.com/upmio/dbscale-kube/pkg/utils/pod"

	unitv4 "github.com/upmio/dbscale-kube/pkg/apis/unit/v1alpha4"
	netclientset "github.com/upmio/dbscale-kube/pkg/client/networking/v1alpha1/clientset/versioned"

	// metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type Parser interface {
	ParseData(data string) error
	GenerateConfig() (string, error)
	Marshal() (string, error)
	Set(key string, val interface{}) error
	Get(key string) (string, bool)
}
type ParserClient struct {
	KubeClient kubernetes.Interface
	NetClient  netclientset.Interface
}

func NewParser(ctx context.Context, client ParserClient, unit *unitv4.Unit, template *corev1.ConfigMap) (Parser, error) {
	switch unit.Spec.MainContainerName {
	case structs.MysqlServiceType:
		return &MysqlParser{
			ctx:      ctx,
			template: template,
			client:   client,
			Unit:     unit,
		}, nil
	case structs.ProxysqlServiceType:
		return &ProxysqlParser{
			ctx:      ctx,
			template: template,
			client:   client,
			Unit:     unit,
		}, nil
	case structs.CmhaServiceType:
		return &CmhaParser{
			ctx:      ctx,
			template: template,
			client:   client,
			Unit:     unit,
		}, nil
	}
	return nil, fmt.Errorf("not support the %s type", unit.Spec.MainContainerName)
}

//{"default/pod1":1,"default/pod2":2}
func getPodGroups(unit *unitv4.Unit) (map[string]int, error) {
	group := map[string]int{}
	value, ok := unit.Annotations[unitv4.PodGroupAnnotation]
	if !ok {
		klog.Errorf("not find %s  annotation on unit", unitv4.PodGroupAnnotation)
		return group, fmt.Errorf("not find %s  annotation on unit", unitv4.PodGroupAnnotation)
	}
	err := json.Unmarshal([]byte(value), &group)
	if err != nil {
		klog.Errorf("%s  :%s Unmarshal fail:%s ", unitv4.PodGroupAnnotation, value, err.Error())
		return group, fmt.Errorf("%s  :%s Unmarshal fail:%s ", unitv4.PodGroupAnnotation, value, err.Error())
	}
	return group, nil
}

func getConfigOptions(pod *corev1.Pod) (map[string]string, error) {
	opt := map[string]string{}
	value, ok := pod.Annotations[unitv4.PodOptionsAnnotation]
	if !ok {
		return opt, nil
	}
	err := json.Unmarshal([]byte(value), &opt)
	if err != nil {
		return opt, fmt.Errorf("%s  :%s Unmarshal fail:%s ", unitv4.PodOptionsAnnotation, value, err.Error())
	}
	return opt, nil
}

func waitUntilPodScheduled(kubeClient kubernetes.Interface, podkey string) (*corev1.Pod, error) {
	var pod *corev1.Pod
	var err error
	namespace, podName, err := cache.SplitMetaNamespaceKey(podkey)
	if err != nil {
		return nil, err
	}

	err = wait.PollImmediate(500*time.Millisecond, 2*time.Second, func() (bool, error) {
		pod, err = kubeClient.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}

		if pod.Spec.NodeName == "" || !podutil.IsCreated(pod) {
			return false, nil
		}

		return true, nil
	})

	if err != nil {
		err = fmt.Errorf("waitUntilPodScheduled %s fail: %s", podName, err.Error())
	}

	return pod, err
}

func waitUntilNetworkClaimsReady(ctx context.Context, client netclientset.Interface, pod *corev1.Pod) (string, error) {
	var err error
	var ip string
	interncKey, ok := pod.Annotations[netv1.NetworkClaimLabelInternal]
	if !ok {
		klog.Errorf("pod not find Annotations %s", netv1.NetworkClaimLabelInternal)
		return "", fmt.Errorf("pod not find Annotations %s", netv1.NetworkClaimLabelInternal)
	}

	err = wait.PollImmediate(500*time.Millisecond, 2*time.Second, func() (bool, error) {
		claim, err := client.NetworkingV1alpha1().NetworkClaims().Get(ctx, interncKey, metav1.GetOptions{})
		if err != nil && errors.IsNotFound(err) {
			return false, nil
		}

		if err != nil {
			return false, err
		}

		if claim.Status.BindIP == "" || claim.Status.Used == "" {
			return false, nil
		}

		podkey := fmt.Sprintf("%s/%s", pod.Namespace, pod.GetName())
		if claim.Status.Used != podkey {
			klog.Errorf("not used by the %s pod(cur:%s)", podkey, claim.Status.Used)
			return false, fmt.Errorf("not used by the %s pod(cur:%s)", podkey, claim.Status.Used)
		}

		ip = claim.Status.BindIP
		return true, nil
	})

	if err != nil {
		err = fmt.Errorf("waitUntilNetowrkClaimsReady %s fail:%s", pod.GetName(), err.Error())
		klog.Error(err)
	}

	return ip, err
}

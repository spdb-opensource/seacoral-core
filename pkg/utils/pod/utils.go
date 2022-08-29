/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package pod

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

func IsContainerRunningAndReady(pod *v1.Pod, containerName string) bool {
	return IsContainerRunning(pod, containerName) && IsContainerReady(pod, containerName)
}

func IsContainerRunning(pod *v1.Pod, containerName string) bool {
	if containerName == "" {
		return pod.Status.Phase == v1.PodRunning
	}

	for _, status := range pod.Status.ContainerStatuses {
		if status.Name == containerName {
			return status.State.Running != nil
		}
	}

	klog.Warningf("IsContainerRunning :%s not find the %s container", pod.Name, containerName)
	return false
}

func IsContainerReady(pod *v1.Pod, containerName string) bool {
	if containerName == "" {
		_, condition := GetPodCondition(&pod.Status, v1.ContainersReady)

		return pod.Status.Phase == v1.PodRunning &&
			condition != nil && condition.Status == v1.ConditionTrue
	}

	for _, status := range pod.Status.ContainerStatuses {
		if status.Name == containerName {
			return status.Ready
		}
	}

	klog.Warningf("IsContainerReady :%s not find the %s container", pod.Name, containerName)
	return false
}

// // IsRunning returns true if pod is in the PodRunning Phase
func IsRunning(pod *v1.Pod) bool {
	return pod.Status.Phase == v1.PodRunning
}

// isRunningAndReady returns true if pod is in the PodRunning Phase, if it has a condition of PodReady.
func IsRunningAndReady(pod *v1.Pod) bool {
	return pod.Status.Phase == v1.PodRunning && IsPodReady(pod)
}

// isCreated returns true if pod has been created and is maintained by the API server
func IsCreated(pod *v1.Pod) bool {
	return pod.Status.Phase != ""
}

// isFailed returns true if pod has a Phase of PodFailed
func IsFailed(pod *v1.Pod) bool {
	return pod.Status.Phase == v1.PodFailed
}

// isTerminating returns true if pod's DeletionTimestamp has been set
func IsTerminating(pod *v1.Pod) bool {
	return pod.DeletionTimestamp != nil
}

// // isHealthy returns true if pod is running and ready and has not been terminated
func IsHealthy(pod *v1.Pod) bool {
	return IsRunningAndReady(pod) && !IsTerminating(pod)
}

//---------------------------------------------------
//copy from  k8s.io/kubernetes/pkg/api/v1/pod

// IsPodReady returns true if a pod is ready; false otherwise.
func IsPodReady(pod *v1.Pod) bool {
	return IsPodReadyConditionTrue(pod.Status)
}

// IsPodReadyConditionTrue returns true if a pod is ready; false otherwise.
func IsPodReadyConditionTrue(status v1.PodStatus) bool {
	condition := GetPodReadyCondition(status)
	return condition != nil && condition.Status == v1.ConditionTrue
}

// GetPodReadyCondition extracts the pod ready condition from the given status and returns that.
// Returns nil if the condition is not present.
func GetPodReadyCondition(status v1.PodStatus) *v1.PodCondition {
	_, condition := GetPodCondition(&status, v1.PodReady)
	return condition
}

// GetPodCondition extracts the provided condition from the given status and returns that.
// Returns nil and -1 if the condition is not present, and the index of the located condition.
func GetPodCondition(status *v1.PodStatus, conditionType v1.PodConditionType) (int, *v1.PodCondition) {
	if status == nil {
		return -1, nil
	}
	return GetPodConditionFromList(status.Conditions, conditionType)
}

// GetPodConditionFromList extracts the provided condition from the given list of condition and
// returns the index of the condition and the condition. Returns -1 and nil if the condition is not present.
func GetPodConditionFromList(conditions []v1.PodCondition, conditionType v1.PodConditionType) (int, *v1.PodCondition) {
	if conditions == nil {
		return -1, nil
	}
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return i, &conditions[i]
		}
	}
	return -1, nil
}

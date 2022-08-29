package core

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewObjectReference(obj metav1.Object, meta metav1.TypeMeta) *corev1.ObjectReference {
	return &corev1.ObjectReference{
		Kind:            meta.Kind,
		APIVersion:      meta.APIVersion,
		Namespace:       obj.GetNamespace(),
		Name:            obj.GetName(),
		UID:             obj.GetUID(),
		ResourceVersion: obj.GetResourceVersion(),
	}
}

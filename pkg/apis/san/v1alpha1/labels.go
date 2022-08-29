package v1alpha1

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

type Level string

const (
	PVCTypeLabel              = "type"
	FsTypeLabel               = "upm.io.fstype"
	VolumeSourceLabel         = "upm.io.storage.volume.source"
	StorageRequestLabel       = "upm.io.storage.request"
	PVCRemoteAllocTypeLabel   = "upm.io.storage.alloc.type"
	PVCSourcePerformanceLabel = "upm.io.storage.performance"

	DefaultFsType = "xfs"
	AllocThick    = "thick"
	AllocThin     = "thin"
	VolumePath    = "volumepath"
	LocalSource   = "local"
	RemoteSource  = "remote"
	LocalType     = "upm.io.storage.local"
	RemoteType    = "upm.io.storage.remote"

	HighPerformance   Level = "high"
	MediumPerformance Level = "medium"
	LowPerformance    Level = "low"
	NonePerformance   Level = ""
)

func getLabel(claim *corev1.PersistentVolumeClaim, label string) (string, bool) {
	v, exist := claim.Labels[label]
	if exist {
		return v, exist
	}

	v, exist = claim.Annotations[label]

	return v, exist
}

// return MB size
func StorageRequest(claim *corev1.PersistentVolumeClaim) (int64, error) {
	str, _ := getLabel(claim, StorageRequestLabel)

	q, err := PVCStorageQuantity(claim)
	if err != nil {
		return 0, fmt.Errorf("Label %s:'%s',error:%s", StorageRequestLabel, str, err)
	}

	total, ok := q.AsInt64()
	if !ok {
		return 0, fmt.Errorf("PVC %s %s", claim.Name, str)
	}

	// MB
	return total >> 20, nil
}

func PVCStorageQuantity(claim *corev1.PersistentVolumeClaim) (resource.Quantity, error) {
	str, ok := getLabel(claim, StorageRequestLabel)
	if !ok {
		return claim.Spec.Resources.Requests[corev1.ResourceStorage], nil
	}

	return resource.ParseQuantity(str)
}

func FileSystemType(claim *corev1.PersistentVolumeClaim) string {
	v, ok := getLabel(claim, FsTypeLabel)
	if ok {
		return v
	}

	return DefaultFsType
}

func IsPVCLocalType(claim *corev1.PersistentVolumeClaim) bool {
	v, _ := getLabel(claim, PVCTypeLabel)

	return v == LocalType
}

func IsPVCRemoteType(claim *corev1.PersistentVolumeClaim) bool {
	v, _ := getLabel(claim, PVCTypeLabel)

	return v == RemoteType
}

func PVCVolumeSourceType(claim *corev1.PersistentVolumeClaim) string {
	v, _ := getLabel(claim, VolumeSourceLabel)

	return v
}

func PVCAllocType(claim *corev1.PersistentVolumeClaim) string {
	v, _ := getLabel(claim, PVCRemoteAllocTypeLabel)

	return v
}

func IsVolumePathSource(claim *corev1.PersistentVolumeClaim) bool {
	v, _ := getLabel(claim, VolumeSourceLabel)

	return v == VolumePath
}

func VGSourcePerformance(claim *corev1.PersistentVolumeClaim) (Level, bool) {
	typ, ok := getLabel(claim, PVCSourcePerformanceLabel)
	if !ok {
		return NonePerformance, false
	}

	return Level(typ), ok
}

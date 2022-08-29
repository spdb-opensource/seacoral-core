package v1alpha1

import (
	"flag"

	// "strings"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	DefaultResourceLowThreshold     float64 = 0.1 // 10%
	DefaultResourceMediumThreshold  float64 = 0.5 // 50%
	DefaultResourceLowLabelValue            = "low"
	DefaultResourceMediumLabelValue         = "medium"
	DefaultResourceHighLabelValue           = "high"
)

var (
	resourceLowThreshold    float64
	resourceMediumThreshold float64
)

func init() {
	flag.Float64Var(&resourceLowThreshold, "resourceLowThreshold", DefaultResourceLowThreshold, "resource low threshold")
	flag.Float64Var(&resourceMediumThreshold, "resourceMediumThreshold", DefaultResourceMediumThreshold, "resource medium threshold")
}

type HostPhase string
type Level string

const (

	//used for kube-scheduler
	NodeLocalVolumeAnnotation = "vg.localvolume.cability"
	NodeUsageLimtAnnotation   = "node.usage.limit"

	VGsAnnotation              = "upm.host.vgs"
	SanInitiatorAnnotation     = "upm.host.san.initiator"
	SanInitiatorTypeAnnotation = "upm.host.san.initiator.type"

	HostcomponentAnnotation = "upm.host.component"
	HostMaxUnitAnnotation   = "upm.host.maxunit"
	HostNetworkMode         = "upm.network.mode"

	HostReady        HostPhase = "ready"
	HostDeployFailed HostPhase = "failed"

	HostInstalling HostPhase = "installing"
	HostInstalled  HostPhase = "installed"

	// SanRegistering HostPhase = "san_registering"
	SanRegistered HostPhase = "san_registered"
	// HostInstallFailed HostPhase = "installFailed"

	// SanUnRegistering HostPhase = "san_unregistering"
	SanUnRegistered  HostPhase = "san_unregistered"
	HostUnInstalling HostPhase = "uninstalling"
	// HostUnInstalled  HostPhase = "uninstalled"
	// HostUnInstallFailed HostPhase = "uninstallFailed"

	PerformanceHigh   Level = "high"
	PerformanceMedium Level = "medium"
	PerformanceLow    Level = "low"
	PerformanceNone   Level = ""

	DeletCode = 943
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Host struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Spec              HostSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status            HostStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// HostList is a collection of Host.
type HostList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Items           []Host `json:"items" protobuf:"bytes,2,rep,name=items"`
}

type NFS struct {
	IP      string `json:"ip"`
	Source  string `json:"source"`
	Options string `json:"options"`
	Target  string `json:"target"`
}

type HostSpec struct {
	ActCode int64 `json:"actCode,omitempty"`
	// HostName  string `json:"hostName"`
	MaxPod        int64  `json:"max_pod,omitempty"`
	NtpServer     string `json:"ntp_server"`
	NetworkMode   string `json:"network_mode"`
	Unschedulable bool   `json:"unschedulable"`

	HostIP   string `json:"host_ip"`
	HostPort int64  `json:"host_port"`
	//加密后的值
	OsUser string `json:"os_user"`
	//加密后的值
	OsPassword string `json:"os_password"`
	//none: 不加密
	//默认:AES
	// SSHCryptoMode string `json:"cryptoMode,omitempty"`
	//Nfs      *NFS     `json:"nfs,omitempty"`
	San      *SanSpec `json:"san,omitempty"`
	LocalVGs []VGSpec `json:"local_vgs"`
}

type SanSpec struct {
	Os        string    `json:"os_type,omitempty"`
	Desc      string    `json:"description,omitempty"`
	Location  string    `json:"location,omitempty"`
	Network   string    `json:"network_name,omitempty"`
	Model     string    `json:"model,omitempty"`
	San       string    `json:"san,omitempty"`
	HostGroup string    `json:"hostgroup_name"`
	Initiator Initiator `json:"initiator"`
}

type UsageLimit struct {
	CPU     int `json:"cpu"`
	Storage int `json:"storage"`
	Memory  int `json:"memory"`
	Units   int `json:"units"`
}

type VGSpec struct {
	Name    string   `json:"name"`
	Level   Level    `json:"level"`
	Devices []string `json:"devices"`
}

type Initiator struct {
	Type string   `json:"type"`
	ID   []string `json:"ids"`
}

type HostStatus struct {
	Phase       HostPhase             `json:"phase"`
	NodeReady   bool                  `json:"node_ready"`
	Capacity    ResouceStatus         `json:"capacity"`
	Allocatable ResouceStatus         `json:"allocatable"`
	NodeInfo    corev1.NodeSystemInfo `json:"nodeInfo,omitempty"`
}

type ResouceStatus struct {
	Units    resource.Quantity `json:"units"`
	Memery   resource.Quantity `json:"memery"`
	Cpu      resource.Quantity `json:"cpu"`
	Pods     resource.Quantity `json:"pods"`
	LocalVGs []VGStatus        `json:"local_vgs"`
}

type VGStatus struct {
	Name  string            `json:"name"`
	Level Level             `json:"level"`
	Size  resource.Quantity `json:"size"`
}

// lijj32: GetResourceAllocatableLevel compares allocatable resources(cpu, memory, disk) with total capacity,
// and returns one of "low, medium, high" appropriately with the following rules:
// 1. if any one of allocatable resources is less than resourceLowWaterMark(default 10%),
// the host will be marked a label DefaultResourceLabelName=DefaultResourceLowLabelValue,
// 2. if all allocatable resources are higher than resourceMediumWaterMark(default 50%),
// the host will be marked a label DefaultResourceLabelName=DefaultResourceHighLabelValue,
// 3. otherwise, the host will be marked a label DefaultResourceLabelName=DefaultResourceMediumLabelValue.
func (h *Host) GetResourceAllocatableLevel() string {
	var (
		diskAlloc float64
		diskTotal float64
	)

	cpuAlloc := float64(h.Status.Allocatable.Cpu.MilliValue())
	memAlloc := float64(h.Status.Allocatable.Memery.Value())
	for _, localVGAlloc := range h.Status.Allocatable.LocalVGs {
		diskAlloc += float64(localVGAlloc.Size.Value())
	}

	cpuTotal := float64(h.Status.Capacity.Cpu.MilliValue())
	memTotal := float64(h.Status.Capacity.Memery.Value())
	for _, localVGTotal := range h.Status.Capacity.LocalVGs {
		diskTotal += float64(localVGTotal.Size.Value())
	}

	if cpuAlloc/cpuTotal > resourceMediumThreshold ||
		memAlloc/memTotal > resourceMediumThreshold ||
		diskAlloc/diskTotal > resourceMediumThreshold {
		return DefaultResourceLowLabelValue
	}

	if cpuAlloc/cpuTotal < resourceLowThreshold &&
		memAlloc/memTotal < resourceLowThreshold &&
		diskAlloc/diskTotal < resourceLowThreshold {
		return DefaultResourceHighLabelValue
	}

	return DefaultResourceMediumLabelValue
}

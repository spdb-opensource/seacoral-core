package v1alpha1

import (
	// corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type NetworkClaimStatusType string
type NetworkModeType string

const (
	Pending NetworkClaimStatusType = "pending"
	Fail    NetworkClaimStatusType = "failed"
	Passing NetworkClaimStatusType = "passing"
	Using   NetworkClaimStatusType = "using"

	NetworkClaimLabelInternal = "upm.networkClaim.internal"
	NetworkClaimLabelExternal = "upm.networkClaim.external"

	NetworkPhaseAnnotation                  = "network.alloc.phase"
	NetworkmaintenancePhase                 = "maintenance"
	NetworkServingPhase                     = "serving"
	CalicoNetworkMode       NetworkModeType = "calico"
	MacVlanNetworkMode      NetworkModeType = "macvlan"
	SriovNetworkMode        NetworkModeType = "sriov"
)

var (
	NetworkHealthStatus = "health"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Network is a specification for a Network resource
type Network struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NetworkSpec   `json:"spec"`
	Status NetworkStatus `json:"status"`
}

// NetworkSpec is the spec for a Network resource
type NetworkSpec struct {
	Mode    NetworkModeType `json:"mode"`
	StartIP string          `json:"startIP"`
	EndIP   string          `json:"endIP"`
	Route   string          `json:"route,omitempty"`

	Mask int32 `json:"mask,omitempty"`
	Vlan int32 `json:"vlan,omitempty"`

	DisabledIP []string `json:"disabled_ip"`
}

// NetworkStatus is the status for a Network resource
type NetworkStatus struct {
	UsedIPCount int32 `json:"usedIPCount"`

	AllIPCounts int32 `json:"allIPCounts"`
	//冲突IP地址
	Conflicts []string `json:"conflicts"`

	Status string `json:"status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// NetworkList is a list of Network resources
type NetworkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Network `json:"items"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NetworkClaim is a specification for a NetworkClaim resource
type NetworkClaim struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NetworkClaimSpec   `json:"spec"`
	Status NetworkClaimStatus `json:"status"`
}

// NetworkClaimSpec is the spec for a NetworkClaim resource
type NetworkClaimSpec struct {
	Mode      NetworkModeType `json:"mode"`
	Network   string          `json:"network"`
	Type      string          `json:"type,omitempty"`
	Bandwidth int32           `json:"bandwidth,omitempty"`
}

// NetworkClaimStatus is the status for a NetworkClaim resource
type NetworkClaimStatus struct {
	BindIP string                 `json:"bindIP"`
	Status NetworkClaimStatusType `json:"status"`
	//被哪个pod 使用
	Used string `json:"used"`

	HostDevice   string `json:"hostDevice,omitempty"`
	Host         string `json:"host,omitempty"`
	CurBandwidth int32  `json:"curBandwidth,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NetworkClaimList is a list of NetworkClaim resources
type NetworkClaimList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []NetworkClaim `json:"items"`
}

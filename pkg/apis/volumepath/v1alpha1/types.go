package v1alpha1

import (
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type VpStatus string

var VPCSIDriverName = "upm.volumepath.csi"
var DefaultVolumePathNameSpace = "default"

const (
	VpBinding VpStatus = "binding"
	VpEmtpy   VpStatus = ""

	VpCreating   VpStatus = "creating"
	VpCreateFail VpStatus = "createfailed"

	VpExtending  VpStatus = "vgExtending"
	VpExtendFail VpStatus = "VpExtendFailed"

	VpDeactivated   VpStatus = "deactivated"
	VpDeactivating  VpStatus = "deactivating"
	VpDeactivatFail VpStatus = "deactivaefailed"

	VpActivating  VpStatus = "activating"
	VpActivatFail VpStatus = "activatefailed"

	VpRemoving  VpStatus = "removing"
	VpRemovFail VpStatus = "removfailed"

	//删除
	DeleteCode int64 = 943

	//重置statu状态为binding且重新设置ResetStatusCode为0
	//作用: 创建，扩展，activate/deactivate 失败后设置该值可触发重做
	ResetStatusCode int64 = 111

	//原目标主机清理
	CleanCode int64 = 222

	//强制把status同步为spec，且status为binding，
	//作用：异常情况下，人工介入解决。
	CopySpecToStatusCode int64 = 609
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

//VolumePath  resources
type VolumePath struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VolumePathSpec   `json:"spec"`
	Status VolumePathStatus `json:"status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VolumePathList is a list of VolumePath resources
type VolumePathList struct {
	metav1.TypeMeta `json:",inline"`

	metav1.ListMeta `json:"metadata"`

	Items []VolumePath `json:"items"`
}

type VolumePathSpec struct {
	//本地:local, 远端存储：remote
	Type string `json:"type"`

	Size   resource.Quantity `json:"size"`
	FsType string            `json:"fs_type"`

	VgName string `json:"vgName"`
	Node   string `json:"node"`

	LunIDs []string `json:"lunIDs"`
	Vendor string   `json:"vendor"`

	//远端存储类型
	InitiatorType string `json:"initiatorType"`

	ActCode int64 `json:"actCode,omitempty"`

	//强制迁移开关
	ForceMigarete bool `json:"forcemigrate"`
}

type VolumePathStatus struct {
	Status  VpStatus          `json:"status"`
	CurSize resource.Quantity `json:"curSize"`

	BindingNode string   `json:"bindingNode"`
	LunIDs      []string `json:"LunIDs"`

	Deivce     string `json:"device"`
	MouterPath string `json:"mouter,omitempty"`
}

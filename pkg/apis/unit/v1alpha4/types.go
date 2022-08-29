/*
Copyright 2017 The Kubernetes Authors.

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

package v1alpha4

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	VGRequestAnnotation          string = "vg.localvolume.request"
	ScriptSyncTab                string = "sync-from"
	ScriptSyncResourceVersionTab string = "sync-resourceversion"
	ScriptDataTab                string = "scripts"

	PodOptionsAnnotation string = "options"
	PodGroupAnnotation   string = "groups"

	ConfigDataTab     string = "content"
	ConfigFilePathTab string = "filepath"

	ConfigMapConfigBackupTab string = "backups"
	ConfigBackupAnnotation   string = "backup-config"

	OptionsAnnotation string = "options"
	LabelGroup        string = "dbscale.service.group"
	StorageLocalType  string = "upm.io.storage.local"
)

type Phase string

const (
// UnitCreating         Phase = "creating"
// UnitDelete           Phase = "deleting"
// UnitServiceMigrating Phase = "migrating"
// UnitRebuild          Phase = "rebuilding"

// UnitServiceRunning Phase = "running"
// UnitServiceStoping Phase = "stopinging"
)

type NetworkType string
type NetworkModeType string

const (
	VolumePathAnnotation             = "volumepath"
	NetworkInternal      NetworkType = "upm.networkClaim.internal"
	NetworkExternal      NetworkType = "upm.networkClaim.external"

	CalicoNetworkMode  NetworkModeType = "calico"
	MacVlanNetworkMode NetworkModeType = "macvlan"
	SriovNetworkMode   NetworkModeType = "sriov"
)

type ConditionStatus string
type ConditionType string
type Condition struct {
	Type   ConditionType
	Status ConditionStatus
}

const (
	ConditionUnset ConditionStatus = ""
	ConditionTrue  ConditionStatus = "True"
	ConditionFalse ConditionStatus = "False"

	InitStartCondition ConditionType = "initStart"
	// InitConfigCondition ConditionType = "initconfig"
	// PullConfigCondition ConditionType = "pullconfig"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Unit is a specification for a Unit resource
type Unit struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UnitSpec   `json:"spec"`
	Status UnitStatus `json:"status"`
}

// UnitSpec is the spec for a Unit resource
type UnitSpec struct {
	MainContainerName string `json:"mainContainerName"`
	MainImageVerison  string `json:"mainImageVersion"`
	UnService         bool   `json:"unservice"`

	Template     corev1.PodTemplateSpec `json:"template"`
	Networking   NetworkingRequest      `json:"networking,omitempty"`
	VolumeClaims []PVCRequest           `json:"claims,omitempty"`
	Action       Action                 `json:"action"`
}
type MigrateAction struct {
	NodeName string `json:"nodeName,omitempty"`
	Force    bool   `json:"force,omitempty"`
}

type RebuildAction struct {
	Force    bool    `json:"force,omitempty"`
	NodeName *string `json:"nodeName,omitempty"`

	RetainVolume *bool `json:"retain_volume,omitempty"`
}

type DeleteAction struct {
	Force   bool               `json:"force,omitempty"`
	PreStop *corev1.ExecAction `json:"exec"`
}

type Action struct {
	Delete            *DeleteAction            `json:"delete,omitempty"`
	Rebuild           *RebuildAction           `json:"rebuild,omitempty"`
	Migrate           *MigrateAction           `json:"migrate,omitempty"`
	ReuseRetainVolume *ReuseRetainVolumeAction `json:"reuse_retain_volume,omitempty"`
}

type ReuseRetainVolumeAction struct {
	Force bool `json:"force,omitempty"`
}

type NetworkingRequest struct {
	Mode      NetworkModeType `json:"mode"`
	Bandwidth int32           `json:"bandwidth,omitempty"`
	Network   string          `json:"network"`
	Type      NetworkType     `json:"type,omitempty"` // default NetworkInternal
}

type PVCRequest struct {
	Name             string  `json:"name"`
	StorageClassName string  `json:"storageClassName"` // volumepath
	Storage          Storage `json:"storage"`
	FsType           string  `json:"fsType"`
	Mounter          string  `json:"mounter,omitempty"`

	AccessModes []corev1.PersistentVolumeAccessMode `json:"accessModes"`
}

type Storage struct {
	Type string `json:"type"` // "upm.io.storage.local" / "upm.io.storage.remote"
	// if Type == "upm.io.storage.local", relate to Host.Spec.LocalVG.Level
	// if Type == upm.io.storage.local , relate to SanSystem.Spec.StoragePoolEnabledList.Level
	Level     string `json:"level"`     // high-performance / normal-performance / low-performance
	AllocType string `json:"allocType"` // "thick / thin

	Request resource.Quantity `json:"request"`
}

func AddErrMsg(unit *Unit, msg ErrMsg) {
	if len(msg.Err) > 300 {
		msg.Err = msg.Err[:300]
	}

	if len(unit.Status.ErrMsgs) == 0 {
		unit.Status.ErrMsgs = []ErrMsg{msg}
		return
	}

	msgs := []ErrMsg{msg}
	for i, curMsg := range unit.Status.ErrMsgs {
		if i > 3 {
			break
		}
		if curMsg.Time.Second()+30*60 > time.Now().Second() {
			msgs = append(msgs, curMsg)
		}
	}
	unit.Status.ErrMsgs = msgs
}

func SetContionStatus(unit *Unit, conditon Condition) {

	for i := range unit.Status.Conditions {
		if unit.Status.Conditions[i].Type == conditon.Type {
			unit.Status.Conditions[i].Status = conditon.Status
			return
		}
	}

	unit.Status.Conditions = append(unit.Status.Conditions, conditon)

}

func GetContionStatus(unit *Unit, ty ConditionType) ConditionStatus {
	for _, condition := range unit.Status.Conditions {
		if condition.Type == ty {
			return condition.Status
		}
	}
	return ConditionUnset
}

type ErrMsg struct {
	Time metav1.Time `json:"time"`
	Err  string      `json:"err"`
	Mode string      `json:"mode"`
}

type RebuildVolumeStatus struct {
	RetainVolumeSuffix int `json:"retain_volume_suffix"`
	CurVolumeSuffix    int `json:"cur_volume_suffix"`
}

// UnitStatus is the status for a Unit resource
type UnitStatus struct {
	RebuildStatus *RebuildVolumeStatus `json:"volume_suffix,omitempty"`
	Conditions    []Condition          `json:"conditions"`
	// Phase      string      `json:"phase"`
	ErrMsgs []ErrMsg `json:"err_msgs"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// UnitList is a list of Unit resources
type UnitList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Unit `json:"items"`
}

package v1alpha1

import (
	"strings"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type SanSystem struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec SanSystemSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`

	Status SanSystemStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

type SanSystemSpec struct {
	//	Name string `json:"name"` // default equal to SanSystem.Name
	Auth Auth `json:"auth_info"`

	StoragePoolList []StoragePoolWithLevel `json:"storage_pool_enabled_list"`
}

type StoragePoolWithLevel struct {
	Enabled bool   `json:"enabled"`
	Name    string `json:"name"`
	Level   Level  `json:"level"` // high-performance / normal-performance / low-performance
}

type Auth struct {
	Port       int    `json:"port"`
	IP         string `json:"ip"`
	User       string `json:"username"`
	Password   string `json:"password"`
	Vstorename string `json:"vstorename"`
	Vendor     string `json:"vendor"`
	Version    string `json:"API_version"`
}

type StoragePool struct {
	Total   int64  `json:"total_capacity_MB"`
	Free    int64  `json:"free_capacity_MB"`
	ID      string `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"disk_type"`
	Health  string `json:"health_status,omitempty"`
	Running string `json:"running_status,omitempty"`
	Desc    string `json:"description,omitempty"`
	Level   Level  `json:"level,omitempty"`
}

type SanSystemStatus struct {
	Disable   bool  `json:"disabled"`
	Connected bool  `json:"connected"`
	Total     int64 `json:"total_MB"` // MB
	Free      int64 `json:"free_MB"`  // MB

	Pools []StoragePool `json:"pools"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SanSystemList is a collection of SanSystem.
type SanSystemList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Items           []SanSystem `json:"items" protobuf:"bytes,2,rep,name=items"`
}

type HostSpec struct {
	Name      string    `json:"name"` // hostname
	IP        string    `json:"ip"`
	Os        string    `json:"os_type,omitempty"`
	Desc      string    `json:"description,omitempty"`
	Location  string    `json:"location,omitempty"`
	Network   string    `json:"network_name,omitempty"`
	Model     string    `json:"model,omitempty"`
	Initiator Initiator `json:"initiator"`
	HostGroup string    `json:"hostgroup_name"`
}

type Initiator struct {
	Type string   `json:"type"`
	ID   []string `json:"id"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Lungroup struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Spec              LungroupSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status            LungroupStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

type MigrateAction struct {
	Force bool   `json:"force,omitempty"`
	Node  string `json:"node"`
}

type DeleteAction struct {
	Force bool `json:"force,omitempty"`
}

type Action struct {
	Migrate *MigrateAction `json:"migrate,omitempty"`
	Delete  *DeleteAction  `json:"delete,omitempty"`
}

type LungroupSpec struct {
	Action   Action            `json:"action,omitempty"`
	VGName   string            `json:"vg_name"`
	PVName   string            `json:"pv_name"`
	Hostname string            `json:"hostname"`
	San      string            `json:"san"`
	Vendor   string            `json:"vendor"`
	Type     string            `json:"alloc_type"` // thick
	FsType   string            `json:"fstype"`
	Level    Level             `json:"level"`
	Capacity resource.Quantity `json:"capacity"`
}

type LungroupStatus struct {
	Phase       LunPhase     `json:"phase"`
	Luns        []Lun        `json:"luns,omitempty"`
	MappingView *MappingView `json:"mapping_view,omitempty"`
}

func (lg LungroupStatus) IsMappingAllowed() bool {
	return len(lg.Luns) > 0 &&
		(lg.MappingView == nil || lg.MappingView.HostGroup == "")
}

func (lg LungroupStatus) IsMappinged() bool {
	return lg.MappingView != nil && lg.MappingView.HostGroup != ""
}

type LunPhase string

const (
	LunGroupReady = "ready"

	LunGroupCreating    = "creating"
	LunGroupCreatFailed = "create_failed"

	LunGroupExtending    = "extending"
	LunGroupExtendFailed = "extend_failed"

	LunGroupMigrating    = "migrating"
	LunGroupMigratFailed = "migrate_failed"

	LungroupAllocating     = "alloc_lungroup"
	LungroupAllocated      = "alloc_lungroup_success"
	LungroupMapping        = "mapping_lungroup"
	LungroupMappingVeiw    = "mapping_lungroup_success"
	LungroupDelMappingView = "delete_lungroup_mappingveiw"
	LungroupDeleteing      = "deleting_lungroup"
	LungroupExpanding      = "lungroup_expanding"
	LungroupExpand         = "lungroup_expand_success"
	LungroupMigrating      = "lungroup_migrating"
	LungroupMigrated       = "lungroup_migrate_success"
)

type Lun struct {
	Bad         bool   `json:"-"` // mark as bad
	Capacity    int64  `json:"capacity_MB"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	StoragePool string `json:"storagepool_name"`
	Health      string `json:"health_status,omitempty"`
	Running     string `json:"running_status,omitempty"`
	Desc        string `json:"description,omitempty"`
	//	CreatedAt   metav1.Time `json:"created_at,omitempty"`
}

func (lg Lungroup) CurSize() int64 {
	size := int64(0)

	for i := range lg.Status.Luns {
		size += lg.Status.Luns[i].Capacity
	}

	return size
}

func (lg Lungroup) Capacity() int64 {
	v := lg.Spec.Capacity.Value()

	return v >> 20
}

type MappingView struct {
	Name      string `json:"Name"`
	Desc      string `json:"hostgroup_description,omitempty"`
	HostGroup string `json:"hostgroup_name"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// LungroupList is a collection of Lungroup.
type LungroupList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Items           []Lungroup `json:"items" protobuf:"bytes,2,rep,name=items"`
}

type HostgroupInfo struct {
	Group    string     `json:"name"`
	Hosts    []HostSpec `json:"hosts"`
	Mappings []Mapping  `json:"mapping_lungroup"`
}

type Mapping struct {
	Name  string `json:"name"`
	Group []Lun  `json:"luns"`
}

type HostgroupMapping struct {
	Name  string     `json:"name"`
	Hosts []HostSpec `json:"hosts"`
}

type LungroupInfo struct {
	NotExist bool              `json:"not_exist,omitempty"`
	Group    string            `json:"name"`
	Type     string            `json:"alloc_type"` // thick
	Luns     []Lun             `json:"luns"`
	Mapping  *HostgroupMapping `json:"mapping_host,omitempty"`
}

func (lg *Lungroup) Update(info LungroupInfo) bool {
	if info.NotExist {

		if len(lg.Status.Luns) == 0 &&
			(lg.Status.MappingView == nil || lg.Status.MappingView.HostGroup == "") {

			lg.Status.MappingView = nil
			lg.Status.Luns = nil

			return false
		}

		lg.Status.MappingView = nil
		lg.Status.Luns = nil

		return true
	}

	update := LunsNotEqual(lg.Status.Luns, info.Luns)
	lg.Status.Luns = info.Luns

	if info.Mapping == nil || info.Mapping.Name == "" {

		if lg.Status.MappingView != nil {

			update = true
			lg.Status.MappingView = nil
		}

	} else {

		if lg.Status.MappingView == nil ||
			lg.Status.MappingView.HostGroup != info.Mapping.Name {

			update = true

			if lg.Status.MappingView != nil {
				lg.Status.MappingView.HostGroup = info.Mapping.Name
			} else {
				lg.Status.MappingView = &MappingView{
					HostGroup: info.Mapping.Name,
				}

				if len(info.Mapping.Hosts) == 1 {
					lg.Status.MappingView.Name = info.Mapping.Hosts[0].Name
				} else {
					for _, host := range info.Mapping.Hosts {
						if host.Name == lg.Spec.Hostname || host.IP == lg.Spec.Hostname {
							lg.Status.MappingView.Name = host.Name
							break
						}
					}
				}
			}
		}
	}

	return update
}

func (lg *Lungroup) LunIDs() []string {
	if lg == nil || len(lg.Status.Luns) == 0 {
		return nil
	}

	luns := make([]string, 0, len(lg.Status.Luns))
	for i := range lg.Status.Luns {

		if id := strings.TrimSpace(lg.Status.Luns[i].ID); id != "" {
			luns = append(luns, id)
		}
	}

	return luns
}

func LunsNotEqual(a, b []Lun) bool {
	if len(a) != len(b) {
		return true
	}

loop:
	for i := range a {

		for j := range b {
			if a[i].ID == b[j].ID &&
				a[i].Name == b[j].Name &&
				a[i].Capacity == b[j].Capacity &&
				a[i].StoragePool == b[j].StoragePool {

				continue loop
			}
		}

		return true
	}

	return false
}

package api

import (
	"sync"
	"time"

	stderror "github.com/pkg/errors"
	"github.com/xeipuuv/gojsonschema"
	"golang.org/x/xerrors"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

type Performance string
type State string

var appJsonSchemas map[string][]byte

func (s State) ToString() string {
	return string(s)
}

var jsonSchemaOnce sync.Once

func jsonSchemaLoad() {
	appJsonSchemas = make(map[string][]byte)
	schema, _ := GetJsonSchema("app.json")
	appJsonSchemas["app.json"] = schema
	schema, _ = GetJsonSchema("appStateOptions.json")
	appJsonSchemas["appStateOptions.json"] = schema
	schema, _ = GetJsonSchema("appResourcesOptions.json")
	appJsonSchemas["appResourcesOptions.json"] = schema
}

const (
	PerformanceNone Performance = ""

	StateNotused   State = "notused"
	StatePreparing State = "preparing"
	StatePassing   State = "passing"
	StateRestoring State = "restoring"
	StateOrch      State = "orchestration"
	StateUnknown   State = "unknown"
	StateCritical  State = "critical"
	StateWarning   State = "warning"

	StorageTypeRemote = "remote"
	StorageTypeHost   = "host"

	ServiceRoleMaster = "master"
	ServiceRoleSlave  = "slave"
)

type AppsResponse []Application

type Application struct {
	ID             string `json:"id"`
	SubscriptionId string `json:"subscription_id,omitempty"`
	Name           string `json:"name"`
	Desc           string `json:"desc"`
	Arch           string `json:"arch"`
	State          State  `json:"state"`

	Spec   AppSpec   `json:"spec"`
	Status AppStatus `json:"status"`

	Task     TaskBrief `json:"task"`
	Created  Editor    `json:"created"`
	Modified Editor    `json:"modified"`
}

type AppSpec struct {
	Proxy    *GroupSpec `json:"proxy,omitempty"`
	Cmha     *GroupSpec `json:"cmha,omitempty"`
	Database *GroupSpec `json:"database"`
}

type AppStatus struct {
	Proxy     DatabaseServices `json:"proxy,omitempty"`
	Cmha      DatabaseServices `json:"cmha,omitempty"`
	Databases DatabaseServices `json:"database"`
}

type DatabaseServices struct {
	Services []GroupStatus `json:"services,omitempty"`
}

type GroupSpec struct {
	Image    ImageVersion `json:"image"`
	Services ServiceSpec  `json:"services"`
}

type ServiceSpec struct {
	Num        int             `json:"num"`
	Arch       Arch            `json:"arch"`
	Ports      []ContainerPort `json:"ports"`
	Conditions Conditions      `json:"conditions"`
	Units      UnitSpec        `json:"units"`
	//Schemas    []AppSchemaConfig `json:"schema"`
	//Users      []DatabaseUser    `json:"user"`
	Backup  Backup            `json:"backup"`
	Options map[string]string `json:"options"`
}

type Backup struct {
	Storage BackupStorage
}

type BackupStorage struct {
	Type        string      `json:"type"`
	Performance Performance `json:"performance"`
	Capacity    int64       `json:"capacity"`
}

type UnitSpec struct {
	Ha             bool      `json:"ha"`
	ReadinessState State     `json:"readiness_state"`
	Resources      Resources `json:"resources"`
}

type UserSpec struct {
	Name string `json:"name"`
	Ip   string `json:"ip"`
	Pwd  string `json:"pwd"`
}

type UserPrivilegeSpec struct {
	DbName     string   `json:"db_name"`
	Privileges []string `json:"privileges"`
}

type GroupStatus struct {
	Name  string          `json:"name"`
	Arch  Arch            `json:"arch"`
	Ports []ContainerPort `json:"ports"`
	Units []UnitInfo      `json:"units"`
}

type Arch struct {
	Replicas int `json:"replicas"`
	// enum: single,clone,replication_async,repliction_semi_sync
	Mode string `json:"mode"`
}

type ContainerPort struct {
	Port int32  `json:"port"`
	Name string `json:"name"`
}

type Resources struct {
	Requests ResourceRequirements `json:"requests"`
}

type ResourceRequirements struct {
	CPU       int64               `json:"cpu"`
	Bandwidth *int32              `json:"net_bandwidth"`
	Memory    int64               `json:"memory"`
	Storage   *StorageRequirement `json:"storage"`
}

type StorageRequirement struct {
	// enum: remote,host
	// require: false
	Type string `json:"type"`
	// require: false
	Performance Performance `json:"performance"`
	// require: true
	Volumes []VolumeRequirement `json:"volumes"`
}

type VolumeRequirement struct {
	Capacity int64  `json:"capacity"`
	Type     string `json:"type"`
}

type Conditions struct {
	//NetworkMode   string    `json:"network_mode,omitempty"`
	NodeRole      string    `json:"node_role,omitempty"`
	Cluster       Condition `json:"cluster"`
	Host          Condition `json:"host"`
	Network       Condition `json:"network"`
	RemoteStorage Condition `json:"storage_remote"`
}

type Condition struct {
	HighAvailability bool     `json:"high_availability"`
	Candidates       []string `json:"candidates_id,omitempty"`
}

type UnitInfo struct {
	ID             string               `json:"id"`
	Ha             bool                 `json:"ha"`
	Namespace      string               `json:"namespace"`
	Image          ImageVersion         `json:"image"`
	IP             IP                   `json:"ip"`
	Resources      ResourceRequirements `json:"resources"`
	Node           NodeBrief            `json:"node"`
	Replication    *Replication         `json:"replication,omitempty"`
	ReadinessState State                `json:"state"`
	PodState       string               `json:"pod_state"`
	Task           TaskBrief            `json:"task"`
	CreatedAt      Time                 `json:"start_at"`
}

type Replication struct {
	/*
		{
		  "role": "slave",
		  "master_ip": "192.168.100.1",
		  "master_port": 3306,
		  "self_ip": "192.168.100.2",
		  "self_port": 3306,
		  "slave_io_running": "Yes",
		  "slave_sql_running": "Yes",
		  "slave_io_state": "",
		  "slave_sql_running_state": "",
		  "seconds_behind_master": 0,
		  "master_log_file": "",
		  "relay_master_log_file": "",
		  "master_log_pos": 100,
		  "relay_log_file": "",
		  "relay_log_pos": 200,
		  "last_io_error": "",
		  "last_sql_error": ""
		}
	*/

	Role       string `json:"role"`
	MasterIP   string `json:"master_ip"`
	MasterPort int    `json:"master_port"`
	SelfIP     string `json:"self_ip"`
	SelfPort   int    `json:"self_port"`

	*ReplicationSlaveInfo
}

type ReplicationSlaveInfo struct {
	SlaveIORunning       string `json:"slave_io_running"`
	SlaveSqlRunning      string `json:"slave_sql_running"`
	SlaveIOState         string `json:"slave_io_state"`
	SlaveSqlRunningState string `json:"slave_sql_running_state"`
	SecondsBehindMaster  int    `json:"seconds_behind_master"`
	MasterLogFile        string `json:"master_log_file"`
	RelayMasterLogFile   string `json:"relay_master_log_file"`
	MasterLogPos         int    `json:"master_log_pos"`
	RelayLogFile         string `json:"relay_log_file"`
	RelayLogPos          int    `json:"relay_log_pos"`
	LastIOError          string `json:"last_io_error"`
	LastSqlError         string `json:"last_sql_error"`
}

type UnitVolumeUsage struct {
	/*
		{
			"type": "data",
			"target_dir": "/mysqldata",
			"source_dev": "/dev/mapper/local_medium_VG-mdtow001--database0000--9feae593--0--data--0",
			"fs_type": "xfs",
		 	"capacity": 100,
			"available": 60,
			"used": 40,
			"percent": 60
		}
	*/
	Capacity  int     `json:"capacity"`
	Used      int     `json:"used"`
	Available int     `json:"available"`
	Percent   float64 `json:"percent"`
	Type      string  `json:"type"`
	Dir       string  `json:"target_dir"`
	Dev       string  `json:"source_dev"`
	FsType    string  `json:"fs_type"`
}

type NodeBrief struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	IP      string `json:"host_ip"`
	Cluster IDName `json:"cluster"`
	Site    IDName `json:"site"`
}

type AppConfig struct {
	Name string  `json:"name"`
	Desc string  `json:"desc"`
	User string  `json:"created_user"`
	Arch string  `json:"arch"`
	Spec AppSpec `json:"spec"`
}

func (app AppConfig) Valid() error {
	jsonSchemaOnce.Do(jsonSchemaLoad)

	appJsonSchema, ok := appJsonSchemas["app.json"]
	if !ok {
		return stderror.New("Failed to load json schema")
	}

	schemaLoader := gojsonschema.NewBytesLoader(appJsonSchema)
	loader := gojsonschema.NewGoLoader(app)
	var errs []error

	result, err := gojsonschema.Validate(schemaLoader, loader)
	if err != nil {
		return err
	}

	if !result.Valid() {
		for _, err := range result.Errors() {
			errs = append(errs, stderror.Errorf("- %s\n", err))
		}
	}

	return utilerrors.NewAggregate(errs)
}

type AppImageOptions struct {
	// {
	//  "spec": {
	//    "database": {
	//      "image": {
	//        "type": "upredis",
	//        "major": 2,
	//        "minor": 0,
	//        "patch": 0,
	//        "build": 51
	//      }
	//    }
	//  }
	// }
	Spec struct {
		Proxy *struct {
			Image *ImageVersion `json:"image,omitempty"`
		} `json:"proxy,omitempty"`
		Cmha *struct {
			Image *ImageVersion `json:"image,omitempty"`
		} `json:"cmha,omitempty"`
		Database *struct {
			Image *ImageVersion `json:"image,omitempty"`
		} `json:"database,omitempty"`
	} `json:"spec"`
}

func (opts AppImageOptions) Valid() error {
	return nil
}

type AppArchOptions struct {
	Spec struct {
		Database *struct {
			Arch *Arch `json:"arch,omitempty"`
		} `json:"database"`
	} `json:"spec"`
}

type AppResourcesOptions struct {
	/*
	   {
	     "spec": {
	       "database": {
	         "services": {
	           "units": {
	             "resources": {
	               "requests": {
	                 "cpu": 0.5,
	                 "memory": 1000,
	                 "net_bandwidth": 100,
	                 "storage": {
	                   "type": "remote",
	                   "performance": "high",
	                   "volumes": [
	                     {
	                       "type": "data",
	                       "capacity": 100
	                     }
	                   ]
	                 }
	               }
	             }
	           }
	         }
	       }
	     }
	   }
	*/
	Spec struct {
		Proxy *struct {
			Services struct {
				Units struct {
					Resources struct {
						Requests ResourceRequirementsOptions `json:"requests,omitempty"`
					} `json:"resources,omitempty"`
				} `json:"units,omitempty"`
			} `json:"services,omitempty"`
		} `json:"proxy,omitempty"`
		Cmha *struct {
			Services struct {
				Units struct {
					Resources struct {
						Requests ResourceRequirementsOptions `json:"requests,omitempty"`
					} `json:"resources,omitempty"`
				} `json:"units,omitempty"`
			} `json:"services,omitempty"`
		} `json:"cmha,omitempty"`
		Database *struct {
			Services struct {
				Units struct {
					Resources struct {
						Requests ResourceRequirementsOptions `json:"requests,omitempty"`
					} `json:"resources,omitempty"`
				} `json:"units,omitempty"`
			} `json:"services,omitempty"`
		} `json:"database,omitempty"`
	} `json:"spec"`
}

func (app AppResourcesOptions) Valid() error {
	jsonSchemaOnce.Do(jsonSchemaLoad)

	appJsonSchema, ok := appJsonSchemas["appResourcesOptions.json"]
	if !ok {
		return stderror.New("Failed to load json schema")
	}

	schemaLoader := gojsonschema.NewBytesLoader(appJsonSchema)
	loader := gojsonschema.NewGoLoader(app)
	var errs []error

	result, err := gojsonschema.Validate(schemaLoader, loader)
	if err != nil {
		return err
	}

	if !result.Valid() {
		for _, err := range result.Errors() {
			errs = append(errs, stderror.Errorf("- %s\n", err))
		}
	}
	return utilerrors.NewAggregate(errs)
}

type ResourceRequirementsOptions struct {
	// require: false
	Bandwidth *int32 `json:"net_bandwidth,omitempty"`
	// require: false
	CPU *int64 `json:"cpu,omitempty"`
	// require: false
	Memory *int64 `json:"memory,omitempty"`
	// require: false
	Storage *StorageRequirement `json:"storage,omitempty"`
}

type UnitRebuildOptions struct {
	// require: false
	Image *string `json:"image,omitempty"`
	// require: false
	Node *string `json:"node,omitempty"`
	// require: false
	Resources *ResourceRequirementsOptions `json:"resources,omitempty"`
}

type UnitRestoreOptions struct {
	// require: true
	File string `json:"backup_file_id"`
	// require: false
	// 超时,Minute
	Timeout *int `json:"timeout,omitempty"`
}

type UnitMigrateOptions struct {
	// require: false
	Node *string `json:"node,omitempty"`
	// require: false
	MaxRetries *int `json:"max_retries,omitempty"`
}

type AppStateOptions struct {
	// enum: passing,critical,terminated
	State State  `json:"state"`
	User  string `json:"modified_user"`
}

func (opt AppStateOptions) Valid() error {
	jsonSchemaOnce.Do(jsonSchemaLoad)

	appJsonSchema, ok := appJsonSchemas["appStateOptions.json"]
	if !ok {
		return stderror.New("Failed to load json schema")
	}

	schemaLoader := gojsonschema.NewBytesLoader(appJsonSchema)
	loader := gojsonschema.NewGoLoader(opt)
	var errs []error

	result, err := gojsonschema.Validate(schemaLoader, loader)
	if err != nil {
		return err
	}

	if !result.Valid() {
		for _, err := range result.Errors() {
			errs = append(errs, stderror.Errorf("- %s\n", err))
		}
	}
	return utilerrors.NewAggregate(errs)
}

type AppActions struct {
	Force    bool          `json:"force"`
	Retries  int           `json:"retries"`
	Interval time.Duration `json:"interval"`
	Timeout  time.Duration `json:"timeout"`

	Actions []ExecAction `json:"actions,omitempty"`
}

type ExecAction struct {
	Unit      string   `json:"unit_name"`
	Type      string   `json:"type,omitempty"`
	Container string   `json:"container"`
	Command   []string `json:"command,omitempty"`
}

type AppUserConfig struct {
	// require: false
	Login *Login `json:"login,omitempty"`

	DatabaseUser
}

type Login struct {
	// example: admin
	User string `json:"user"`
	// example: admin
	Password string `json:"password"`
}

type AppUsersResponse []DatabaseUser

type AppUserPrivilegesOptions struct {
	Name string `json:"name"`
	IP   IP     `json:"ip"`

	Privileges []DatabasePrivilege `json:"db_privileges"`
}

type UpdateAppUserPrivileges struct {
	Name             string              `json:"name"`
	IP               IP                  `json:"ip"`
	AddPrivileges    []DatabasePrivilege `json:"add_privileges"`
	RemovePrivileges []DatabasePrivilege `json:"remove_privileges"`
}

type DatabaseUser struct {
	// example: zhang
	Name string `json:"name"`
	IP   IP     `json:"ip"`

	// example: mysql_native_password
	AuthType string `json:"auth_type"`

	// example: qwertyuiop
	Password string `json:"pwd,omitempty"`

	// example: ["192.168.0.1","192.168.0.2"]
	// require: false
	// Whitelist []string `json:"whitelist,omitempty"`

	Privileges []DatabasePrivilege `json:"db_privileges"`
}

type DatabasePrivilege struct {
	// example: abc
	DBName string `json:"db_name"`
	// example: ["SELECT","UPDATE"]
	Privileges []string `json:"privileges"`
}

func (config AppUserConfig) Valid() error {
	return nil
}

// {
//     "name": "xxx",
//     "character_set": "utf8mb4"
//  }

type DBSchemaResponse []Schema

type AppSchemaConfig Schema

func (AppSchemaConfig) Valid() error {
	return nil
}

type Schema struct {
	Name string `json:"name"`
	//example: utf8mb4
	Character string `json:"character_set"`
	Size      int    `json:"size"`
}

type DBSchemaDetail struct {
	Schema

	Tables []DBTable `json:"tables"`
}

type DBTable struct {
	Name string `json:"name"`
	Size int    `json:"size"`
}

type DBSchemaDetailResponse DBSchemaDetail

type AppUserResetConfig struct {
	Name     string `json:"name"`
	IP       string `json:"ip"`
	AuthType string `json:"auth_type"`
	Password string `json:"pwd"`
}

type UnitRoleSwitchConfig struct {
	Units []struct {
		ID   string `json:"id"`
		Role string `json:"role"`
	} `json:"units"`
}

type RoleMasterOptions struct {
	ArchMode string `json:"arch_mode"`
}

type RoleSlaveOptions struct {
	ArchMode string `json:"arch_mode"`
	MasterIP string `json:"master_ip"`
}

type RoleSetOptions struct {
	Role     string `json:"role"`
	MasterID string `json:"master_id"`
}

type UserAddOptions struct {
	IP []string `json:"ip"`
}

type ConfigMapResponse []KeySet

type ConfigMap struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	CanSet      bool   `json:"can_set"`
	MustRestart bool   `json:"must_restart"`
	Range       string `json:"range"`
	Default     string `json:"default"`
	Desc        string `json:"desc"`
}

type ConfigMapOptions struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

//cmha topology
type CmhaTopology struct {
	Service *ServiceTopology
	Nodes   []*NodesTopology
}

type ServiceTopology struct {
	Name               string    `json:"name"`
	Version            string    `json:"version"`
	CheckId            uint64    `json:"checkid"`
	SWMAddress         string    `json:"swm_address"`
	CreateIndex        time.Time `json:"create_index"`
	ModifyIndex        time.Time `json:"modify_index"`
	EnsureAvailability bool      `json:"ensure_availability"`
	EnsureConsistency  bool      `json:"ensure_consistency"`
}

type NodesTopology struct {
	Node          string               `json:"node"`
	Address       string               `json:"address"`
	Port          int                  `json:"port"`
	Status        string               `json:"status"`
	Replication   *TopologyReplication `json:"replication"`
	Candidate     bool                 `json:"candidate"`
	Role          string               `json:"role"`
	Isolate       bool                 `json:"isolate"`
	ReadWriteMode string               `json:"read_write_mode"`
	MasterHost    string               `json:"master_host"`
	Consul        *HealthCheck         `json:"consul"`
	Maintain      bool                 `json:"maintain"`
	Version       string               `json:"version"`
}

type TopologyReplication struct {
	IOThread       string `json:"io_thread"`
	SQLThread      string `json:"sql_thread"`
	ReplMode       string `json:"repl_mode"`
	RpelErrCounter int    `json:"rpel_err_counter"`
}

type HealthCheck struct {
	ServiceStatus string `json:"sstatus"`
	AgentStatus   string `json:"astatus"`
}

type CmhaReplModeOptions struct {
	//Mode   string `json:"mode"`
	UnitID string `json:"unit_id"`
}

func (c CmhaReplModeOptions) Valid() error {
	var errs []error

	if c.UnitID == "" {
		errs = append(errs, xerrors.New("unit id is required"))
	}

	//if c.Mode == "" {
	//	errs = append(errs, xerrors.New("replication mode is required"))
	//}
	//
	//if c.Mode != "" {
	//	if c.Mode != ReplModeAsync && c.Mode != ReplModeSemiSync {
	//		errs = append(errs, xerrors.Errorf("Replication mode not support %s yet, only support [%s] or [%s]",
	//			c.Mode, ReplModeSemiSync, ReplModeAsync))
	//	}
	//}

	return utilerrors.NewAggregate(errs)
}

type CmhaMaintenanceOptions struct {
	Maintenance *bool  `json:"maintenance"`
	UnitID      string `json:"unit_id"`
}

func (c CmhaMaintenanceOptions) Valid() error {
	var errs []error

	if c.UnitID == "" {
		errs = append(errs, xerrors.New("unit id is required"))
	}

	if c.Maintenance == nil {
		errs = append(errs, xerrors.New("replication mode is required"))
	}

	return utilerrors.NewAggregate(errs)
}

package api

import (
	"fmt"
	"github.com/upmio/dbscale-kube/pkg/vars"

	"golang.org/x/xerrors"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

type HostsResponse []Host

type Host struct {
	Enabled   bool   `json:"enabled"`
	ID        string `json:"id"`
	Desc      string `json:"desc"`
	Cluster   IDName `json:"cluster"`
	Arch      string `json:"arch"`
	NTPServer string `json:"ntp_server"`
	Role      string `json:"role"`
	Location

	// 本地存储
	HostStorages  []HostStorageBrief `json:"storage_host"`
	RemoteStorage IDName             `json:"storage_remote"`

	Node NodeInfo `json:"node"`
	//Units []UnitBrief `json:"units"`

	ResourceLimit

	Task TaskBrief `json:"task"`

	Created  Editor `json:"created"`
	Modified Editor `json:"modified"`
}

type HostDetail struct {
	ID       string `json:"id"`
	HostName string `json:"host_name"`
	Cluster  IDName `json:"cluster"`
	Enabled  bool   `json:"enabled"`
	Arch     string `json:"arch"`
	Desc     string `json:"desc"`

	Location

	NTPServer     string             `json:"ntp_server"`
	Role          string             `json:"role"`
	NetworkMode   string             `json:"network_mode"`
	HostStorages  []HostStorageBrief `json:"storage_host"`
	RemoteStorage IDName             `json:"storage_remote"`

	ResourceLimit

	Task TaskBrief `json:"task"`

	Created  Editor `json:"created"`
	Modified Editor `json:"modified"`

	Node  NodeInfo    `json:"node"`
	Units []UnitBrief `json:"units"`
	//ExistingUnit int         `json:"existing_unit"`
}

type UnitBrief struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Type  string `json:"type"`
	State string `json:"state"`

	CPU          int64 `json:"cpu"`
	Memory       int64 `json:"mem"`
	NetBandwidth int64 `json:"net_bandwidth"`
}

type NodeInfo struct {
	Name string `json:"name"`
	IP   string `json:"ip"`
	Os   string `json:"os"`

	Unit         ResourceStatus `json:"unit"`
	Pod          ResourceStatus `json:"pod"`
	CPU          ResourceStatus `json:"cpu"`
	Memory       ResourceStatus `json:"memory"`
	NetBandwidth ResourceStatus `json:"net_bandwidth"`
}

type ResourceStatus struct {
	Used     int64 `json:"used"`
	Capacity int64 `json:"capacity"`
}

type HostConfig struct {
	// 名称
	// Name    string `json:"name"`
	Cluster string `json:"cluster_id"`
	// 描述
	Desc      string `json:"desc,omitempty"`
	NTPServer string `json:"ntp_server"`

	// 系统帐号
	SSHConfig Auth `json:"ssh"`

	// 位置
	Location

	ResourceLimit
	// 远程存储系统
	RemoteStorage *string `json:"storage_remote_id,omitempty"`
	// 本地存储
	HostStorages []HostStorage `json:"storage_host"`
	Role         string        `json:"role"`

	Enabled bool   `json:"enabled"`
	User    string `json:"created_user"`
}

type ResourceLimit struct {
	MaxUnit int `json:"max_unit"`

	MaxUsage MaxUsage `json:"max_usage"`
}

type MaxUsage struct {
	CPU          int `json:"cpu"`
	Memory       int `json:"mem"`
	HostStorage  int `json:"storage_host"`
	NetBandwidth int `json:"net_bandwidth"`
}

// 位置
type Location struct {
	// 机房
	Room string `json:"room"`
	// 位
	Seat string `json:"seat"`
}

func (l Location) String() string {
	return fmt.Sprintf("Room:%s-Seat:%s", l.Room, l.Seat)

}

type HostStorageBrief struct {
	HostStorage
	ResourceStatus
}

type HostStorage struct {
	Performance Performance `json:"performance"`
	Paths       []string    `json:"paths"`
}

func (n HostConfig) Valid() error {
	var errs []error

	if n.Cluster == "" {
		errs = append(errs, xerrors.New("cluster is required"))
	}

	if string(n.SSHConfig.IP) == "" {
		errs = append(errs, xerrors.New("ip is required"))
	}

	if ip := n.SSHConfig.IP.Parse(); ip == nil {
		errs = append(errs, xerrors.Errorf("ip: %s parse error", n.SSHConfig.IP))
	}

	if n.SSHConfig.Port <= 0 {
		errs = append(errs, xerrors.New("port is required"))
	}

	if n.SSHConfig.User == "" {
		errs = append(errs, xerrors.New("ssh user is required"))
	}

	if n.SSHConfig.Password == "" {
		errs = append(errs, xerrors.New("ssh password is required"))
	}

	if n.Room == "" {
		errs = append(errs, xerrors.New("room is required"))
	}

	if n.Seat == "" {
		errs = append(errs, xerrors.New("seat is required"))
	}

	if n.HostStorages == nil {
		errs = append(errs, xerrors.New("storages are required"))
	}

	if n.HostStorages != nil {
		for i := range n.HostStorages {
			if n.HostStorages[i].Performance == "" {
				errs = append(errs, xerrors.New("storages performance is required"))
			}

			if n.HostStorages[i].Paths == nil {
				errs = append(errs, xerrors.New("storages path is required"))
			}
		}
	}

	if n.MaxUnit <= 0 {
		errs = append(errs, xerrors.New("units is required"))
	}

	if n.MaxUsage.Memory <= 0 {
		errs = append(errs, xerrors.New("memory is required"))
	}

	if n.MaxUsage.CPU <= 0 {
		errs = append(errs, xerrors.New("cpu is required"))
	}

	if n.Role != vars.NodeRolenode && n.Role != vars.NodeRoleSpare {
		errs = append(errs, xerrors.Errorf("Not  support: %s yet, only support [%s] or [%s]", n.Role, vars.NodeRolenode, vars.NodeRoleSpare))
	}

	return utilerrors.NewAggregate(errs)
}

type HostOptions struct {
	Room    *string `json:"room,omitempty"`
	Seat    *string `json:"seat,omitempty"`
	Desc    *string `json:"desc,omitempty"`
	MaxUnit *int    `json:"max_unit,omitempty"`

	MaxUsage struct {
		CPU          *int `json:"cpu,omitempty"`
		Memory       *int `json:"mem,omitempty"`
		HostStorage  *int `json:"storage_host,omitempty"`
		NetBandwidth *int `json:"net_bandwidth,omitempty"`
	} `json:"max_usage,omitempty"`

	Enabled *bool  `json:"enabled,omitempty"`
	Role    string `json:"role,omitempty"`
	User    string `json:"modified_user"`
}

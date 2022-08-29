package api

import (
	"net"

	"golang.org/x/xerrors"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

type NetworksResponse []Network

type Network struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Site    IDName `json:"site"`
	Cluster IDName `json:"cluster"`
	Desc    string `json:"desc"`
	Enabled bool   `json:"enabled"`
	// 拓扑
	Topology []string  `json:"topology"`
	IP       IPSummary `json:"ip_summary"`
	Created  Editor    `json:"created"`
	Modified Editor    `json:"modified"`
}

type IPSummary struct {
	Total int `json:"total"`
	Used  int `json:"used"`
	Route
}

type IP string

func (ip IP) String() string {
	return string(ip)
}

func (ip IP) Valid() error {
	if v := net.ParseIP(string(ip)); v != nil {
		return nil
	}

	return xerrors.Errorf("parse IP %s error", ip)
}

func (ip IP) Parse() net.IP {
	return net.ParseIP(string(ip))
}

type NetworkConfig struct {
	// 名称
	Site string `json:"site_id"`
	// 集群
	Cluster  string   `json:"cluster_id,omitempty"`
	Desc     string   `json:"desc"`
	Topology []string `json:"topology,omitempty"`
	// IP 信息
	IPSummary Route `json:"ip_summary"`
	//NetworkMode string `json:"network_mode"`

	Enabled bool   `json:"enabled"`
	User    string `json:"created_user"`
}

type Route struct {
	// 掩码
	Prefix int32 `json:"prefix"`
	// Vlan ID
	VLan int32 `json:"vlan"`
	// 起始 IP
	Start IP `json:"start_ip"`
	// 最大 IP
	End IP `json:"end_ip"`
	// 网关
	Gateway IP `json:"gateway"`
}

func (req NetworkConfig) Valid() error {
	errs := []error{}

	if req.Cluster == "" {
		errs = append(errs, xerrors.New("cluster_id is required"))
	}

	if err := req.IPSummary.Start.Valid(); err != nil {
		errs = append(errs, err)
	}

	if err := req.IPSummary.End.Valid(); err != nil {
		errs = append(errs, err)
	}

	if err := req.IPSummary.Gateway.Valid(); err != nil {
		errs = append(errs, err)
	}

	if req.IPSummary.Prefix < 16 || req.IPSummary.Prefix > 32 {
		errs = append(errs, xerrors.New("ip prefix only support [16-32]"))
	}

	if req.IPSummary.VLan < 0 || req.IPSummary.VLan > 4096 {
		errs = append(errs, xerrors.New("ip VLAN only support [0-4096]"))
	}

	return utilerrors.NewAggregate(errs)
}

type NetworkOptions struct {
	// 集群
	Cluster  *string  `json:"cluster_id,omitempty"`
	Desc     *string  `json:"desc,omitempty"`
	Topology []string `json:"topology,omitempty"`

	Enabled *bool  `json:"enabled,omitempty"`
	User    string `json:"modified_user"`
}

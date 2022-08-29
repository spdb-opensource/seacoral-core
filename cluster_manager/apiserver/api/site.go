package api

import (
	"golang.org/x/xerrors"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

type SitesResponse []Site

type Site struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Domain string `json:"domain"`
	Port   int    `json:"port"`
	Desc   string `json:"desc"`
	// example: SH
	Region        string `json:"region"`
	ImageRegistry string `json:"image_registry"`
	ProjectName   string `json:"project_name"`
	NetworkMode   string `json:"network_mode"`
	Version       string `json:"version"`
	State         string `json:"state"`
	Created       Editor `json:"created"`
	Modified      Editor `json:"modified"`

	Labels map[string]string `json:"-"`
}

const (
	KubernetesSite = "kubernetes"
	SwarmSite      = "swarm"
)

// SiteConfig
type SiteConfig struct {
	// 名称
	Name string `json:"name"`
	// 类型
	// enum: kubernetes,swarm
	Type string `json:"type"`
	// 连接域名
	// example: project.dbscale.com
	Domain string `json:"domain"`
	// 端口号
	// example: 6443
	Port int    `json:"port"`
	Path string `json:"path"`
	// 描述
	Desc string `json:"desc"`
	// example: SH
	Region string `json:"region"`

	//registry
	ImageRegistry string `json:"image_registry"`
	ProjectName   string `json:"project_name"`
	NetworkMode   string `json:"network_mode"`
	User          string `json:"created_user"`
}

func (c SiteConfig) Valid() error {
	var errs []error

	if c.Type != KubernetesSite {
		errs = append(errs, xerrors.Errorf("Not support %s yet,only support [%s]", c.Type, KubernetesSite))
	}

	if c.Domain == "" {
		errs = append(errs, xerrors.New("domain is required"))
	}

	if c.Port <= 0 {
		errs = append(errs, xerrors.New("port is required"))
	}

	if c.ImageRegistry == "" {
		errs = append(errs, xerrors.New("image_registry is required"))
	}

	if c.ProjectName == "" {
		errs = append(errs, xerrors.New("project_name is required"))
	}

	if c.NetworkMode == "" {
		errs = append(errs, xerrors.New("network_mode is required"))
	}

	return utilerrors.NewAggregate(errs)
}

type SiteOptions struct {
	// example: 6443
	Port *int `json:"port,omitempty"`

	// example: test-site001
	Name *string `json:"name,omitempty"`

	// example: project.dbscale.com
	Domain *string `json:"domain,omitempty"`

	Path string `json:"-"`

	Desc *string `json:"desc,omitempty"`

	// example: SH
	Region *string `json:"region,omitempty"`

	User string `json:"modified_user"`
}

type ListSiteRequest struct {
	// in: query
	// required: false
	ID string `json:"id"`

	// in: query
	// required: false
	Name string `json:"name"`
}

type UpdateSiteRequest struct {
	// in: path
	// required: true
	ID string `json:"id"`

	// in: body
	// required: true
	Body SiteOptions
}

type DeleteSiteRequest struct {
	// 对象 ID 或者 Name
	//
	// required: true
	// in: path
	ID string `json:"id"`
}

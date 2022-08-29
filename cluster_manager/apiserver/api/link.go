package api

import (
	"golang.org/x/xerrors"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

type LinkConfig struct {
	ServiceGroupType string   `json:"service_group_type"`
	Services         []string `json:"services"`
}

func (l LinkConfig) Valid() error {
	var errs []error

	if l.ServiceGroupType == "" {
		errs = append(errs, xerrors.New("service_group_type is required"))
	}

	//if l.ServiceGroupType != "" {
	//	if l.ServiceGroupType != Cmha && l.ServiceGroupType != ProxySQL {
	//		errs = append(errs, xerrors.Errorf("Not support %s yet,only support [%s] and [%s]", l.ServiceGroupType, Cmha, ProxySQL))
	//	}
	//}

	if l.Services == nil || len(l.Services) == 0 {
		errs = append(errs, xerrors.New("no service to link"))
	}

	return utilerrors.NewAggregate(errs)
}

type LinkCmdOptions struct {
	//ServiceGroupType string   `json:"service_group_type"`
	Services LinkServ `json:"services"`
}

type LinkServ struct {
	Cmha     IpsPort `json:"cmha"`
	Proxysql IpsPort `json:"proxysql"`
	Mysql    IpsPort `json:"mysql"`
}

type IpsPort struct {
	Ips  []string `json:"ips"`
	Port int32    `json:"port"`
}

type IPS struct {
	IP []string `json:"ip"`
}

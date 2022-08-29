package v1alpha1

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	hostv1 "github.com/upmio/dbscale-kube/pkg/apis/host/v1alpha1"
	"github.com/upmio/dbscale-kube/pkg/utils"
	"github.com/upmio/dbscale-kube/pkg/utils/crypto"
	"github.com/upmio/dbscale-kube/pkg/vars"
	log "k8s.io/klog/v2"
)

type HostCfg struct {
	SSHUser    string `json:"ssh_user"`
	SSHPasswod string `json:"ssh_password"`
	SSHPort    int64  `json:"ssh_port"`
	SSHIP      string `json:"host_ip"`

	HostName string `json:"host_name"`
	MaxPod   int64  `json:"max_pod"`

	NtpServer   string `json:"ntp_server"`
	NetworkMode string `json:"network_mode"`

	LocalVGs []LocalVGCfg `json:"vgs"`
}

type LocalVGCfg struct {
	Devices string `json:"dev_list"`
	Name    string `json:"vg_name"`
}

func generateHostCfg(host *hostv1.Host) (*HostCfg, error) {
	hostcfg := HostCfg{}
	user, err := crypto.AesDecrypto(host.Spec.OsUser, vars.SeCretAESKey)
	if err != nil {
		return nil, err
	}

	password, err := crypto.AesDecrypto(host.Spec.OsPassword, vars.SeCretAESKey)
	if err != nil {
		return nil, err
	}

	hostcfg.SSHPasswod = password
	hostcfg.SSHUser = user
	hostcfg.SSHIP = host.Spec.HostIP
	hostcfg.SSHPort = host.Spec.HostPort

	hostcfg.HostName = host.GetName()
	hostcfg.MaxPod = host.Spec.MaxPod
	hostcfg.NtpServer = host.Spec.NtpServer
	hostcfg.NetworkMode = host.Spec.NetworkMode

	hostcfg.LocalVGs = []LocalVGCfg{}
	for _, vg := range host.Spec.LocalVGs {
		hostcfg.LocalVGs = append(hostcfg.LocalVGs, LocalVGCfg{
			Name:    vg.Name,
			Devices: strings.Join(vg.Devices, ","),
		})
	}

	return &hostcfg, nil
}

func deployNode(cfg HostCfg) error {
	argsjson, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("%v Marshal fail:%s", cfg, err.Error())
	}
	execfile := filepath.Join(utils.GetNodeInitDir(), "host-init")
	execarg := []string{"install", fmt.Sprintf("%s", string(argsjson))}

	cfg.SSHPasswod = "****"
	printjson, _ := json.Marshal(cfg)
	log.V(4).Infof("cmd:%s install %s", execfile, printjson)

	out, err := exec.Command(execfile, execarg...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("installNode exec fail:%s(out:%s)", err.Error(), out)
	}

	return nil
}

func unDeployNode(cfg HostCfg) error {
	argsjson, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("%v Marshal fail:%s", cfg, err.Error())
	}

	execfile := filepath.Join(utils.GetNodeInitDir(), "host-init")
	execarg := []string{"uninstall", fmt.Sprintf("%s", string(argsjson))}

	cfg.SSHPasswod = "****"
	printjson, _ := json.Marshal(cfg)
	log.V(4).Infof("cmd:%s uninstall %s", execfile, printjson)

	// log.V(2).Infof("cmd:%s %v", execfile, strings.Replace(execarg[1], cfg.SSHPasswod, "*****", -1))
	out, err := exec.Command(execfile, execarg...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("uninstallNode exec fail:%s(out:%s)", err.Error(), out)
	}

	return nil
}

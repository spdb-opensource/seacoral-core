package v1alpha1

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	vpv1 "github.com/upmio/dbscale-kube/pkg/apis/volumepath/v1alpha1"
	"k8s.io/klog/v2"
)

var (
	BaseMounterDir = "/volumepath/k8s/"
	defaulttimeout = 10 * 60 * time.Second
)

type CmdResult struct {
	Code int    `json:"code"`
	Err  string `json:"error"`
}

type VpActCfg struct {
	VpName string `json:"name"`
	FsType string `json:"fs_type"`
	Size   int    `json:"size_MB"`
	Device string `json:"device"`
	//MounterDir string `json:"mount_dir"`

	Lv struct {
		Name string `json:"name"`
	} `json:"lv"`

	Vg struct {
		Name          string   `json:"name"`
		Type          string   `json:"type"`
		Vendor        string   `json:"vendor"`
		InitiatorType string   `json:"initiator_type"`
		ExtendLunIDs  []string `json:"add_LUN_ID"`
		LunIDs        []string `json:"LUN_ID"`
	} `json:"vg"`
}

type VPStatusResult struct {
	Mounter string `json:"mounter"`
	Mounted bool   `json:"mounted"`
}

type VPAddResult struct {
	Device  string `json:"device"`
	Mounter string `json:"mounter"`
}

func generateCommonActCfg(vp *vpv1.VolumePath) *VpActCfg {
	act := &VpActCfg{
		VpName: vp.GetName(),
		FsType: vp.Spec.FsType,
		Size:   int(vp.Spec.Size.Value()) >> 20,
		Device: vp.Status.Deivce,
		//MounterDir: filepath.Join(BaseMounterDir, vp.GetName()),
	}

	act.Lv.Name = vp.GetName()

	act.Vg.Vendor = vp.Spec.Vendor
	act.Vg.Name = vp.Spec.VgName
	act.Vg.Type = vp.Spec.Type
	act.Vg.InitiatorType = vp.Spec.InitiatorType

	return act
}

func cleanVP(shellfile string, cfg *VpActCfg) error {
	_, err := vpCommonAct(shellfile, "vp", "clean", cfg, defaulttimeout)
	return err
}

func createVP(shellfile string, cfg *VpActCfg) (VPAddResult, error) {
	ret := VPAddResult{}

	out, err := vpCommonAct(shellfile, "vp", "add", cfg, defaulttimeout)
	if err != nil {
		return ret, err
	}

	err = json.Unmarshal(out, &ret)
	if err != nil {
		return ret, fmt.Errorf("Unmarshal VPAddResult fail :%s(data:%s)", err.Error(), out)
	}

	if ret.Device == "" {
		return ret, fmt.Errorf("device  is emtpy(data:%s)", out)
	}

	return ret, nil
}

func deleteVP(shellfile string, cfg *VpActCfg) error {
	_, err := vpCommonAct(shellfile, "vp", "delete", cfg, defaulttimeout)
	return err
}

func expandVP(shellfile string, cfg *VpActCfg) error {
	_, err := vpCommonAct(shellfile, "vp", "expand", cfg, defaulttimeout)
	return err
}

func activateVP(shellfile string, cfg *VpActCfg) error {
	_, err := vpCommonAct(shellfile, "vp", "active", cfg, defaulttimeout)
	return err
}

func deActivateVP(shellfile string, cfg *VpActCfg) error {
	_, err := vpCommonAct(shellfile, "vp", "deactive", cfg, defaulttimeout)
	return err
}

func getVPStatus(shellfile string, cfg *VpActCfg) (VPStatusResult, error) {

	ret := VPStatusResult{}
	out, err := vpCommonAct(shellfile, "vp", "getStatus", cfg, defaulttimeout)
	if err != nil {
		return ret, err
	}

	err = json.Unmarshal(out, &ret)
	if err != nil {
		return ret, fmt.Errorf("VPStatusResult Unmarshal fail:%s(datas:%s)", err.Error(), string(out))
	}

	return ret, nil

}

func checkVP(shellfile string, cfg *VpActCfg) error {
	_, err := vpCommonAct(shellfile, "vp", "check", cfg, defaulttimeout)
	return err
}

func vpCommonAct(shellfile, model, act string, cfg *VpActCfg, timeout time.Duration) ([]byte, error) {

	argsjson, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("%v Marshal fail:%s", cfg, err.Error())
	}

	args := []string{model, act, fmt.Sprintf("%s", string(argsjson))}
	klog.V(4).Infof("%  vp: exec script_file:%s , args:%v", cfg.VpName, shellfile, args)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, shellfile, args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s fail:%s,%s(cmd: %s %v)", act, err.Error(), string(out), shellfile, args)
	}

	return out, nil
}

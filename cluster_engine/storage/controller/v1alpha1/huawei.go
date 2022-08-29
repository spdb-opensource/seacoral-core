package v1alpha1

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	hostv1 "github.com/upmio/dbscale-kube/pkg/apis/host/v1alpha1"
	"github.com/upmio/dbscale-kube/pkg/apis/san/v1alpha1"
	"github.com/upmio/dbscale-kube/pkg/utils"
	crypto "github.com/upmio/dbscale-kube/pkg/utils/crypto"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
)

type cmdType string

const (
	list_storage_Cmd cmdType = "storagepool_list_CMD"

	list_hostgroup_Cmd cmdType = "hostgroup_list_CMD"
	add_hostgroup_Cmd  cmdType = "hostgroup_add_CMD"
	del_hostgroup_Cmd  cmdType = "hostgroup_delete_CMD"

	list_lungroup_Cmd        cmdType = "lungroup_list_CMD"
	add_lungroup_Cmd         cmdType = "lungroup_add_CMD"
	expand_lungroup_Cmd      cmdType = "lungroup_expand_CMD"
	del_lungroup_Cmd         cmdType = "lungroup_delete_CMD"
	add_lungroup_mapping_Cmd cmdType = "lungroup_add_mapping_CMD"
	del_lungroup_mapping_Cmd cmdType = "lungroup_delete_mapping_CMD"
)

func newHUAWEI(key, script string, san *v1alpha1.SanSystem, recorder record.EventRecorder) *huawei {
	return &huawei{
		key:         key,
		script:      script,
		once:        new(sync.Once),
		SanSystem:   san,
		poolCache:   map[string]v1alpha1.StoragePool{},
		execContext: ExecContextTimeout,
		recorder:    recorder,
	}
}

type huawei struct {
	key    string
	script string
	once   *sync.Once
	auth   v1alpha1.Auth

	poolCache map[string]v1alpha1.StoragePool
	*v1alpha1.SanSystem

	recorder    record.EventRecorder
	execContext func(ctx context.Context, args ...string) ([]byte, error)
}

func (h huawei) cmd(typ cmdType) []string {
	switch typ {
	case list_storage_Cmd:
		return []string{h.script, "storagepool", "list", ""}

	case list_hostgroup_Cmd:
		return []string{h.script, "hostgroup", "list", ""}

	case add_hostgroup_Cmd:
		return []string{h.script, "hostgroup", "add", ""}

	case del_hostgroup_Cmd:
		return []string{h.script, "hostgroup", "delete", ""}

	case list_lungroup_Cmd:
		return []string{h.script, "lungroup", "list", ""}

	case add_lungroup_Cmd:
		return []string{h.script, "lungroup", "add", ""}

	case expand_lungroup_Cmd:
		return []string{h.script, "lungroup", "expand", ""}

	case del_lungroup_Cmd:
		return []string{h.script, "lungroup", "delete", ""}

	case add_lungroup_mapping_Cmd:
		return []string{h.script, "lungroup", "add_mapping", ""}

	case del_lungroup_mapping_Cmd:
		return []string{h.script, "lungroup", "delete_mapping", ""}
	default:
		return nil
	}
}

func (h *huawei) getAuthInfo() v1alpha1.Auth {
	h.once.Do(func() {
		h.auth = h.SanSystem.Spec.Auth

		if n := len(h.auth.Password); n > 0 && n%4 == 0 && h.key != "" {
			h.auth.Password, _ = crypto.AesDecrypto(h.auth.Password, h.key)
		}
	})

	return h.auth
}

func (h *huawei) Info() (*v1alpha1.SanSystem, error) {
	clone := h.SAN()

	list, err := h.storagePoolInfoCmd("")
	if err != nil {
		clone.Status.Connected = false
		return clone, err
	}
	clone.Status.Free = 0
	clone.Status.Total = 0
	pools := make([]v1alpha1.StoragePool, 0, len(clone.Spec.StoragePoolList))
loop:
	for i := range clone.Spec.StoragePoolList {
		for l := range list {
			if list[l].Name == clone.Spec.StoragePoolList[i].Name ||
				list[l].ID == clone.Spec.StoragePoolList[i].Name {

				list[l].Level = clone.Spec.StoragePoolList[i].Level
				pools = append(pools, list[l])
				clone.Status.Free += list[l].Free
				clone.Status.Total += list[l].Total
				continue loop
			}
		}
	}
	clone.Status.Pools = pools
	clone.Status.Connected = true
	//put to poolCache
	for i := range clone.Status.Pools {
		h.poolCache[clone.Status.Pools[i].Name] = clone.Status.Pools[i]
	}
	return clone, nil
}

func (h huawei) SAN() *v1alpha1.SanSystem {
	return h.SanSystem.DeepCopy()
}

type listInfoRequest struct {
	Auth v1alpha1.Auth `json:"auth_info"`
	Date struct {
		Name string `json:"name"`
	} `json:"data"`
}

func (h *huawei) listInfoCmd(name string, cmd cmdType, out interface{}) error {
	req := listInfoRequest{Auth: h.getAuthInfo()}
	req.Date.Name = name

	dat, err := h.execWithJsonParams(cmd, req)
	if err != nil {
		return err
	}

	err = json.Unmarshal(dat, out)

	return err
}

func (h *huawei) storagePoolInfoCmd(pool string) ([]v1alpha1.StoragePool, error) {
	var list []v1alpha1.StoragePool
	err := h.listInfoCmd(pool, list_storage_Cmd, &list)

	return list, err
}

func (h *huawei) listHostgroupCmd(group string) (v1alpha1.HostgroupInfo, error) {
	info := v1alpha1.HostgroupInfo{}
	err := h.listInfoCmd(group, list_hostgroup_Cmd, &info)

	return info, err
}

func (h *huawei) addHostCmd(host hostv1.Host) error {
	req := struct {
		Auth v1alpha1.Auth `json:"auth_info"`
		Data struct {
			Name  string              `json:"name"`
			Hosts []v1alpha1.HostSpec `json:"hosts"`
		} `json:"data"`
	}{
		Auth: h.getAuthInfo(),
	}
	req.Data.Name = host.Spec.San.HostGroup
	req.Data.Hosts = []v1alpha1.HostSpec{{
		Name:      host.Name,
		IP:        host.Spec.HostIP,
		Os:        host.Spec.San.Os,
		Desc:      host.Spec.San.Desc,
		Location:  host.Spec.San.Location,
		Network:   host.Spec.San.Network,
		Model:     host.Spec.San.Model,
		Initiator: v1alpha1.Initiator(host.Spec.San.Initiator),
		HostGroup: host.Spec.San.HostGroup,
	}}

	_, err := h.execWithJsonParams(add_hostgroup_Cmd, req)

	return err
}

func (h *huawei) delHostCmd(group string, hosts ...string) error {
	req := struct {
		Auth v1alpha1.Auth `json:"auth_info"`
		Data struct {
			Name  string   `json:"name"`
			Hosts []string `json:"hosts_name"`
		} `json:"data"`
	}{
		Auth: h.getAuthInfo(),
	}

	req.Data.Name = group
	req.Data.Hosts = hosts

	_, err := h.execWithJsonParams(del_hostgroup_Cmd, req)

	return err
}

func (h *huawei) listLungroupCmd(group string) (v1alpha1.LungroupInfo, error) {
	info := v1alpha1.LungroupInfo{}
	err := h.listInfoCmd(group, list_lungroup_Cmd, &info)

	return info, err
}

type lunRequest struct {
	Auth v1alpha1.Auth `json:"auth_info"`
	Data struct {
		Group string         `json:"name"`
		Type  string         `json:"alloc_type,omitempty"`
		Luns  []v1alpha1.Lun `json:"luns"`
	} `json:"data"`
}

func (h *huawei) createLunsCmd(lr lunRequest) error {
	lr.Auth = h.getAuthInfo()

	_, err := h.execWithJsonParams(add_lungroup_Cmd, lr)

	return err
}

func (h *huawei) expandLunsCmd(lr lunRequest) error {
	lr.Auth = h.getAuthInfo()

	_, err := h.execWithJsonParams(expand_lungroup_Cmd, lr)

	return err
}

func (h *huawei) delLunCmd(lr v1alpha1.Lungroup) error {
	req := struct {
		Auth v1alpha1.Auth `json:"auth_info"`
		Data struct {
			Group string   `json:"name"`
			Luns  []string `json:"luns_id"`
		} `json:"data"`
	}{
		Auth: h.getAuthInfo(),
	}

	req.Data.Group = lr.Name
	req.Data.Luns = make([]string, len(lr.Status.Luns))
	for i := range lr.Status.Luns {
		req.Data.Luns[i] = lr.Status.Luns[i].ID
	}

	_, err := h.execWithJsonParams(del_lungroup_Cmd, req)

	return err
}

type mappingViewRequest struct {
	Auth v1alpha1.Auth `json:"auth_info"`
	Data struct {
		Lungroup  string `json:"name"`
		Hostgroup string `json:"hostgroup_name"`
	} `json:"data"`
}

func (h *huawei) createMappingView(lungroup, hostgroup string) error {
	req := mappingViewRequest{Auth: h.getAuthInfo()}
	req.Data.Lungroup = lungroup
	req.Data.Hostgroup = hostgroup

	_, err := h.execWithJsonParams(add_lungroup_mapping_Cmd, req)

	return err
}

func (h *huawei) delMappingView(lungroup, hostgroup string) error {
	req := mappingViewRequest{Auth: h.getAuthInfo()}
	req.Data.Lungroup = lungroup
	req.Data.Hostgroup = hostgroup

	_, err := h.execWithJsonParams(del_lungroup_mapping_Cmd, req)

	return err
}

func (h huawei) execWithJsonParams(cmd cmdType, v interface{}) ([]byte, error) {
	in, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	args := h.cmd(cmd)
	if len(args) == 0 {
		return nil, fmt.Errorf("cmdType '%s' isnot supported yet", cmd)
	}

	args[len(args)-1] = fmt.Sprintf("'%s'", in)

	dat, err := h.execContext(context.Background(), args...)

	out, _ := utils.MaskJsonSecret(in)
	args[len(args)-1] = fmt.Sprintf("'%s'", out)

	if err != nil {
		h.recorder.Eventf(h.SanSystem, corev1.EventTypeWarning, cmdFailed, messageCmdExecFailed, args, dat, err)
	}

	klog.V(5).Infof("CMD:%s,Output:%s,Error:%v", args, dat, err)

	return dat, err
}

// ExecContext returns a context command to execute a script.
func ExecContext(ctx context.Context, args ...string) *exec.Cmd {
	shell, flag := "/bin/bash", "-c"

	if runtime.GOOS == "windows" {
		shell = "cmd"
		flag = "/C"
	}

	return exec.CommandContext(ctx, shell, flag, strings.Join(args, " "))
}

// ExecContextTimeout exec command with timeout
func ExecContextTimeout(ctx context.Context, args ...string) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	var out []byte
	buf := bytes.NewBuffer(nil)
	cmd := ExecContext(ctx, args...)
	cmd.Stdout = buf
	cmd.Stderr = buf

	err := cmd.Start()
	if err != nil {
		return nil, err
	}

	wait := make(chan error, 1)
	go func() {
		wait <- cmd.Wait()
		close(wait)
	}()

	select {
	case err = <-wait:
		out = buf.Bytes()
	case <-ctx.Done():
		go func() {
			for range wait {
			}
		}()

		return nil, ctx.Err()
	}

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok && exitError.Success() {
			return out, nil
		} else if cmd.ProcessState != nil && cmd.ProcessState.Success() {
			return out, nil
		}
	}

	return out, err
}

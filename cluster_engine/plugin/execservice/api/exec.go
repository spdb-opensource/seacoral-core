package api

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"time"

	"github.com/upmio/dbscale-kube/pkg/server/client"
	"k8s.io/klog/v2"
)

type PodExecOption struct {
	Namespace string   `json:"namespace"`
	PodName   string   `json:"podName"`
	Container string   `json:"container"`
	Command   []string `json:"command"`
	Stdin     string   `json:"stdin"`
	Stderr    bool     `json:"stderr"`
	Stdout    bool     `json:"stdout"`
}

type PodExecResponse struct {
	Done bool `json:"done"`

	Namespace string   `json:"namespace"`
	PodName   string   `json:"podName"`
	Command   []string `json:"command"`

	Stdout []byte `json:"stdout"`
	Stderr []byte `json:"stderr"`
	Errors string `json:"error"`
}

type DeployImageOption struct {
	ImageRegistry string `json:"image_registry"`
	ProjectName   string `json:"project_name"`
	Type          string `json:"type"`
	Arch          string `json:"arch"`
	Version       string `json:"version"`
}

type DeployImageOptionResponse struct {
	Errors  string `json:"error"`
	Outputs string `json:"outputs"`
}

type LegalizeHostOption struct {
	SSHUser    string       `json:"ssh_user"`
	SSHPasswod string       `json:"ssh_password"`
	SSHPort    int64        `json:"ssh_port"`
	SSHIP      string       `json:"host_ip"`
	HostName   string       `json:"host_name"`
	MaxUnit    int          `json:"max_unit"`
	NtpServer  string       `json:"ntp_server"`
	LocalVGs   []LocalVGCfg `json:"vgs"`

	CheckType string `json:"check_type"`
}

type LegalizeHostExecOption struct {
	SSHUser    string       `json:"ssh_user"`
	SSHPasswod string       `json:"ssh_password"`
	SSHPort    int64        `json:"ssh_port"`
	SSHIP      string       `json:"host_ip"`
	HostName   string       `json:"host_name"`
	MaxUnit    int          `json:"max_unit"`
	NtpServer  string       `json:"ntp_server"`
	LocalVGs   []LocalVGCfg `json:"vgs"`
}

type LocalVGCfg struct {
	Devices string `json:"dev_list"`
	Name    string `json:"vg_name"`
}

type LegalizeHostResponse struct {
	Errors  string `json:"errors"`
	Outputs string `json:"outputs"`
}

func (pe PodExecResponse) Error() string {
	return fmt.Sprintf("Pod %s/%s exec done:%t %s,error:%s,Output:%s;%s",
		pe.Namespace, pe.PodName, pe.Done, pe.Command, pe.Errors, pe.Stderr, pe.Stdout)
}

type ExecClient interface {
	Exec(ctx context.Context, opts PodExecOption) (PodExecResponse, error)
	DeployImage(ctx context.Context, opts DeployImageOption) (DeployImageOptionResponse, error)
	LegalizeHost(ctx context.Context, opts LegalizeHostOption) (LegalizeHostResponse, error)
}

func NewClient(addr string, config *tls.Config) ExecClient {
	c := client.NewClient(addr, time.Minute*15, config)

	return &execClient{client: c}
}

type execClient struct {
	client client.Client
}

func (c *execClient) Exec(ctx context.Context, opts PodExecOption) (PodExecResponse, error) {
	resp, err := c.client.Post(ctx, "/v1.0/engine/exec", opts)
	if err != nil {
		client.EnsureBodyClose(resp)

		return PodExecResponse{}, err
	}
	defer resp.Body.Close()

	out := PodExecResponse{}

	err = json.NewDecoder(resp.Body).Decode(&out)

	return out, err
}

func (c *execClient) DeployImage(ctx context.Context, opts DeployImageOption) (DeployImageOptionResponse, error) {

	resp, err := c.client.Post(ctx, "/v1.0/image/deploy", opts)
	klog.Infof("DeployImage: client.Post ...")
	if err != nil {
		klog.Errorf("DeployImage: client.Post ERR: %s", err)
		client.EnsureBodyClose(resp)

		return DeployImageOptionResponse{}, err
	}
	defer resp.Body.Close()

	out := DeployImageOptionResponse{}
	klog.Infof("&&&&DeployImage: resp body: %s", resp.Body)

	err = json.NewDecoder(resp.Body).Decode(&out)

	return out, err
}

func (c *execClient) LegalizeHost(ctx context.Context, opts LegalizeHostOption) (LegalizeHostResponse, error) {

	resp, err := c.client.Post(ctx, "/v1.0/host/legalization", opts)
	klog.Infof("LegalizeHost: client.Post ...")
	if err != nil {
		klog.Errorf("LegalizeHost: client.Post ERR: %s", err)
		client.EnsureBodyClose(resp)

		return LegalizeHostResponse{}, err
	}
	defer resp.Body.Close()

	out := LegalizeHostResponse{}

	err = json.NewDecoder(resp.Body).Decode(&out)
	klog.Infof("LegalizeHost: resp out: %v", out.Outputs)

	return out, err
}

package exec

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"path/filepath"
	"strconv"
	//"sync/atomic"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/upmio/dbscale-kube/pkg/utils"
	"k8s.io/klog/v2"

	"github.com/upmio/dbscale-kube/cluster_engine/plugin/execservice/api"
	"github.com/upmio/dbscale-kube/pkg/server/router"
	executil "github.com/upmio/dbscale-kube/pkg/utils/exec"
	restclient "k8s.io/client-go/rest"
)

/*var serverReady = &atomicBool{0}

// atomicBool uses load/store operations on an int32 to simulate an atomic boolean.
type atomicBool struct {
	v int32
}

// set sets the int32 to the given boolean.
func (a *atomicBool) set(value bool) {
	if value {
		atomic.StoreInt32(&a.v, 1)
		return
	}
	atomic.StoreInt32(&a.v, 0)
}

// get returns true if the int32 == 1
func (a *atomicBool) get() bool {
	return atomic.LoadInt32(&a.v) == 1
}*/

func RegisterRouter(config *restclient.Config, routers router.Adder) {
	er := &execRouter{
		config: config,
	}

	er.routes = []router.Route{
		router.NewPostRoute("/engine/exec", er.Exec),
		router.NewPostRoute("/image/deploy", er.DeployImage),
		router.NewPostRoute("/host/legalization", er.LegalizeHost),
		router.NewGetRoute("/healthz", er.healthzHandler),
	}

	routers.AddRouter(er)
}

type execRouter struct {
	config *restclient.Config

	routes []router.Route
}

func (er execRouter) Routes() []router.Route {
	return er.routes
}

func (er execRouter) DeployImage(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	req := api.DeployImageOption{}
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	argjson, err := json.Marshal(req)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	dir := utils.GetBaseKubeScriptDir()
	execfile := filepath.Join(dir, "imageMGR", "imageMGR")
	//args := []string{"image", "deploy", fmt.Sprintf("%s", string(argjson))}
	args := []string{"image", "pull", fmt.Sprintf("%s", string(argjson))}

	resp := api.DeployImageOptionResponse{}
	out, err := exec.Command(execfile, args...).CombinedOutput()
	klog.Infof("DeployImage info: exec deploy image args: %s; OUT: %s; Err: %v", args, out, err)
	if err != nil {
		klog.Errorf("DeployImage err: %s, out: %s", err, out)
		resp.Errors = err.Error()
	}

	resp.Outputs = string(out)

	return http.StatusOK, resp, nil
}

func (er execRouter) Exec(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	req := api.PodExecOption{}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	stderr, stdout := bytes.NewBuffer(nil), bytes.NewBuffer(nil)

	ok, err := executil.NewExecInContainer(er.config).Exec(
		executil.NewExecOptions(
			req.Namespace, req.PodName, req.Container,
			req.Command, stderr, stdout))

	resp := api.PodExecResponse{
		Namespace: req.Namespace,
		PodName:   req.PodName,
		Done:      ok,
		Command:   req.Command,
		Stderr:    stderr.Bytes(),
		Stdout:    stdout.Bytes(),
		Errors:    "",
	}

	if err != nil {
		resp.Errors = err.Error()
	}

	return http.StatusOK, resp, nil
}

func (er execRouter) LegalizeHost(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	req := api.LegalizeHostOption{}
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return http.StatusBadRequest, nil, err
	}

	resp := api.LegalizeHostResponse{
		Errors:  "",
		Outputs: "Success to legalize host! ",
	}

	execOpts := api.LegalizeHostExecOption{
		SSHUser:    req.SSHUser,
		SSHPasswod: req.SSHPasswod,
		SSHPort:    req.SSHPort,
		SSHIP:      req.SSHIP,
		HostName:   req.HostName,
		MaxUnit:    req.MaxUnit,
		NtpServer:  req.NtpServer,
		LocalVGs:   req.LocalVGs,
	}

	argsjson, err := json.Marshal(execOpts)
	if err != nil {
		resp.Errors = err.Error()
		resp.Outputs = "Marshal request options failed: "
		return http.StatusOK, resp, nil
	}
	execfile := filepath.Join(utils.GetNodeInitDir(), "host-init")

	switch req.CheckType {
	case "user":
		klog.Info("start check user,name,password...")
		hostKeyCallbk := func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		}

		config := ssh.ClientConfig{
			User:            execOpts.SSHUser,
			Auth:            []ssh.AuthMethod{ssh.Password(execOpts.SSHPasswod)},
			HostKeyCallback: hostKeyCallbk,
			Timeout:         time.Second * 2,
		}

		addr := net.JoinHostPort(execOpts.SSHIP, strconv.Itoa(int(execOpts.SSHPort)))

		client, err := ssh.Dial("tcp", addr, &config)
		if err != nil {
			resp.Errors = err.Error()
			resp.Outputs = "Username or password or port authentication failed: "
			return http.StatusOK, resp, nil
		}
		defer client.Close()

		return http.StatusOK, resp, nil

	case "port":
		klog.Info("LegalizeHost start check port...")
		execOpts.SSHPasswod = "****"
		printjson, _ := json.Marshal(execOpts)
		klog.Infof("LegalizeHost execfile:%s input:%s", execfile, printjson)

		execarg := []string{"check_port", fmt.Sprintf("%s", string(argsjson))}
		out, err := exec.Command(execfile, execarg...).CombinedOutput()
		if err != nil {
			resp.Errors = err.Error()
			resp.Outputs = "Fail to check port:" + string(out)
			return http.StatusOK, resp, nil
		}

		return http.StatusOK, resp, nil

	case "network":
		klog.Info("LegalizeHost start check network...")
		execOpts.SSHPasswod = "****"
		printjson, _ := json.Marshal(execOpts)
		klog.Infof("LegalizeHost execfile:%s input:%s", execfile, printjson)

		execarg := []string{"check_network", fmt.Sprintf("%s", string(argsjson))}
		out, err := exec.Command(execfile, execarg...).CombinedOutput()
		if err != nil {
			resp.Errors = err.Error()
			resp.Outputs = "Fail to check network:" + string(out)
			return http.StatusOK, resp, nil
		}

		return http.StatusOK, resp, nil

	case "storage":
		klog.Info("LegalizeHost start check storage...")
		execOpts.SSHPasswod = "****"
		printjson, _ := json.Marshal(execOpts)
		klog.Infof("LegalizeHost execfile:%s input:%s", execfile, printjson)

		execarg := []string{"check_storage", fmt.Sprintf("%s", string(argsjson))}
		out, err := exec.Command(execfile, execarg...).CombinedOutput()
		if err != nil {
			resp.Errors = err.Error()
			resp.Outputs = "Fail to check storage:" + string(out)
			return http.StatusOK, resp, nil
		}

		return http.StatusOK, resp, nil

	default:
		return http.StatusOK, api.LegalizeHostResponse{
			Errors:  "Fail to check",
			Outputs: fmt.Sprintf("Not support check %s", req.CheckType),
		}, nil
	}
}

// healthHandler response with a 200 if the UDP server is ready. It also serves
// as a health check of the HTTP server by virtue of being a HTTP handler.
func (er execRouter) healthzHandler(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string) (int, interface{}, error) {
	//if serverReady.get() {
	//	w.WriteHeader(200)
	return http.StatusOK, nil, nil
	//}
	//w.WriteHeader(http.StatusPreconditionFailed)
	//return http.StatusPreconditionFailed, nil, nil
}

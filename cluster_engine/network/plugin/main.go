package main

import (
	"context"
	"encoding/json"
	stderr "errors"
	"flag"
	"fmt"
	"net"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/upmio/dbscale-kube/pkg/utils"
	"github.com/upmio/dbscale-kube/pkg/vars"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	result "github.com/containernetworking/cni/pkg/types/020"
	"github.com/containernetworking/cni/pkg/version"

	networkv1 "github.com/upmio/dbscale-kube/pkg/apis/networking/v1alpha1"
	networkingClient "github.com/upmio/dbscale-kube/pkg/client/networking/v1alpha1/clientset/versioned"
)

var (
	versionFlag bool

	addnetworkFile   = "netdevMGR"
	delnetwokrFile   = "netdevMGR"
	getNetDeviceFile = "netdevMGR"

	LackDeviceErr = stderr.New("lack of net device")
)

func init() {
	flag.BoolVar(&versionFlag, "version", false, "show the version ")
}

// NetConf is whatever you expect your configuration json to be. This is whatever
// is passed in on stdin. Your plugin may wish to expose its functionality via
// runtime args, see CONVENTIONS.md in the CNI spec.
type NetConf struct {
	types.NetConf

	ShellDir string `json:"shellDir"`

	KubeConfig string `json:"kubeConfig"`
	Master     string `json:"master"`

	VolumepathNoCheck bool `json:"noCheckVolumepath"`
}

type networkRuntimeCfg struct {
	IfName  string          `json:"kube_dev_name"`
	Netns   string          `json:"network_namespace"`
	Devices []runtimeConfig `json:"network_devices"`

	podIP   string
	podMask string
}

type runtimeConfig struct {
	Ip    string `json:"ip_address"`
	Mask  string `json:"prefix"`
	Route string `json:"gateway"`

	Device string `json:"native_dev"`

	Bandwidth   int32  `json:"bandwidth_Mb"`
	Vlan        int32  `json:"vlan_id"`
	NetWorkType string `json:"network_type"`
}

type NetDevice struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	SriovMode   bool   `json:"sriov_mode"`
	NetWorkType string `json:"network_type"`
}

type NetDeviceList struct {
	NativeDevices []NetDevice `json:"native_devices"`
}

/*
// 执行脚本命令，没这问题
func init() {
	// this ensures that main runs only on main thread (thread group leader).
	// since namespace ops (unshare, setns) are done for a single thread, we
	// must ensure that the goroutine does not jump from OS thread to thread
	runtime.LockOSThread()
}
*/

type Kubeclient struct {
	networkingClient networkingClient.Interface
	kubeClient       kubernetes.Interface
	//vpClient         volumepathClientset.Interface
	conf *NetConf
}

func initKubeclient(conf *NetConf) (*Kubeclient, error) {
	cfg, err := clientcmd.BuildConfigFromFlags(conf.Master, conf.KubeConfig)
	if err != nil {
		return nil, fmt.Errorf("Error building kubeconfig: %v", err)
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("Error building kubernetes clientset: %v", err)
	}

	networkClient, err := networkingClient.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("Error building network clientset: %v", err)
	}

	//var volumepathClient volumepathClientset.Interface
	//
	////默认都检查vp是否正常挂载
	//if !conf.VolumepathNoCheck {
	//	volumepathClient, err = volumepathClientset.NewForConfig(cfg)
	//	if err != nil {
	//		return nil, fmt.Errorf("Error building volumepath clientset: %v", err)
	//	}
	//}

	return &Kubeclient{
		networkingClient: networkClient,
		kubeClient:       kubeClient,
		conf:             conf,
		//vpClient:         volumepathClient,
	}, nil

}

func parseConfig(stdin []byte) (*NetConf, error) {

	conf := NetConf{}

	if err := json.Unmarshal(stdin, &conf); err != nil {
		return nil, fmt.Errorf("failed to parse network configuration: %s", err.Error())
	}

	return &conf, nil
}

func parseArgs(args string) (string, string, error) {

	pairs := strings.Split(args, ";")

	podName := ""
	podNameSpace := ""

	for _, pair := range pairs {
		kv := strings.Split(pair, "=")
		if len(kv) != 2 || kv[0] == "" || kv[1] == "" {
			return "", "", fmt.Errorf("invalid CNI_ARGS pair %q", pair)
		}

		switch kv[0] {
		case "K8S_POD_NAME":
			podName = kv[1]
		case "K8S_POD_NAMESPACE":
			podNameSpace = kv[1]
		}
	}

	if podName == "" || podNameSpace == "" {
		return "", "", fmt.Errorf("%s:K8S_POD_NAME or K8S_POD_NAMESPACE is empty", args)
	}

	return podName, podNameSpace, nil
}

func waitForClaim(client *Kubeclient, pod *v1.Pod, label string, tryTimes int) (*networkv1.NetworkClaim, error) {
	cliamkey, ok := pod.Annotations[label]
	if !ok || cliamkey == "" {
		return nil, fmt.Errorf("not find claim from pod.Annotations %s", label)
	}

	for i := 0; i < tryTimes; i++ {
		cliam, err := client.networkingClient.NetworkingV1alpha1().NetworkClaims().Get(context.TODO(), cliamkey, metav1.GetOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return nil, err
		}

		if cliam != nil && cliam.Status.Status == networkv1.Using {
			return cliam, nil
		}

		time.Sleep(600 * time.Millisecond)
	}

	return nil, fmt.Errorf("not find the  networkcliam or waiting for cliam.Status.Status=using")
}

func updateCliamStatus(kubeclient *Kubeclient, networkClaim *networkv1.NetworkClaim, host, device string, bandwidth int32) error {

	toUpdate := networkClaim.DeepCopy()
	toUpdate.Status.Host = host
	toUpdate.Status.HostDevice = device
	toUpdate.Status.CurBandwidth = bandwidth

	_, err := kubeclient.networkingClient.NetworkingV1alpha1().NetworkClaims().UpdateStatus(context.TODO(), toUpdate, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("Networkclaims UpdateStatus HostDevice fail:%s", err.Error())
	}

	return nil
}

func prepareNetworkeConfig(kubeclient *Kubeclient, podName, podNameSpace string) (*networkRuntimeCfg, error) {
	networkcfg := &networkRuntimeCfg{}
	pod, err := kubeclient.kubeClient.CoreV1().Pods(podNameSpace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return networkcfg, err
	}

	// 内网必须有
	cfg, err := prepareRunTimeConfig(kubeclient, pod, networkv1.NetworkClaimLabelInternal)
	if err != nil {
		return nil, err
	}
	networkcfg.Devices = append(networkcfg.Devices, cfg)

	//pod默认IP地址
	networkcfg.podIP = cfg.Ip
	networkcfg.podMask = cfg.Mask

	//外网可选
	// excfg, err := prepareRunTimeConfig(kubeclient, pod, networkv1.NetworkClaimLabelExternal)
	// if err != nil && (apierrors.IsNotFound(err) || strings.Contains(err.Error(), notFound)) {
	// 	return networkcfg, nil
	// }
	// if err != nil {
	// 	return nil, err
	// }
	// networkcfg.Devices = append(networkcfg.Devices, excfg)

	return networkcfg, nil

}

func prepareRunTimeConfig(kubeclient *Kubeclient, pod *v1.Pod, label string) (runtimeConfig, error) {
	runtimeCfg := runtimeConfig{}
	podkey := fmt.Sprintf("%s/%s", pod.GetNamespace(), pod.GetName())

	claim, err := waitForClaim(kubeclient, pod, label, 3)
	if err != nil {
		return runtimeCfg, err
	}

	network, err := kubeclient.networkingClient.NetworkingV1alpha1().Networks().Get(context.TODO(), claim.Spec.Network, metav1.GetOptions{})
	if err != nil {
		return runtimeCfg, err
	}

	if claim.Status.Status != networkv1.Using ||
		claim.Status.Used != podkey {
		return runtimeCfg, fmt.Errorf(" the claim status:%s(should be %s) ; uesd by %s(should be %s)", claim.Status.Status, networkv1.Using, claim.Status.Used, pod.Name)
	}

	if claim.Status.Host == pod.Spec.NodeName {

		if claim.Spec.Bandwidth != claim.Status.CurBandwidth {
			if err := updateCliamStatus(kubeclient, claim, claim.Status.Host, claim.Status.HostDevice, claim.Spec.Bandwidth); err != nil {
				return runtimeCfg, err
			}
		}

		runtimeCfg.Device = claim.Status.HostDevice
	} else {
		//to lock
		if err := utils.LockFile(); err != nil {
			return runtimeCfg, err
		}
		defer utils.UnlockFile()

		networktype := "internal"
		if label == networkv1.NetworkClaimLabelExternal {
			networktype = "external"
		}

		device, err := kubeclient.getUnusedNetDevice(pod.Spec.NodeName, networktype, "")
		if err != nil {
			return runtimeCfg, err
		}

		if err := updateCliamStatus(kubeclient, claim, pod.Spec.NodeName, device, claim.Spec.Bandwidth); err != nil {
			return runtimeCfg, err
		}

		time.Sleep(600 * time.Millisecond)

		runtimeCfg.Device = device
	}

	runtimeCfg.Vlan = network.Spec.Vlan
	runtimeCfg.Route = network.Spec.Route
	runtimeCfg.Bandwidth = claim.Spec.Bandwidth
	runtimeCfg.Ip = claim.Status.BindIP
	runtimeCfg.Mask = strconv.Itoa(int(network.Spec.Mask))

	return runtimeCfg, nil
}

func (client *Kubeclient) getUnusedNetDevice(nodeName string, netwokType, deviceType string) (string, error) {

	devices, err := listNetDevice(client.conf.ShellDir, netwokType, deviceType)
	if err != nil {
		return "", err
	}

	if len(devices) == 0 {
		return "", fmt.Errorf("don't find any related device(fileter: %s)", netwokType)
	}

	cliams, err := client.networkingClient.NetworkingV1alpha1().NetworkClaims().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return "", stderr.New("list cliams fail:" + err.Error())
	}

	usedDevies := []string{}
	for _, claim := range cliams.Items {
		if claim.Status.Host == nodeName && claim.Status.HostDevice != "" {
			usedDevies = append(usedDevies, claim.Status.HostDevice)
		}
	}

	for _, _device := range devices {

		device := _device.Name

		if device == "" {
			continue
		}

		used := false
		for _, usedivce := range usedDevies {
			if device == usedivce {
				used = true
				break
			}
		}

		if !used {
			return device, nil
		}

	}

	return "", fmt.Errorf("all related device(%v) is used", devices)

}

// cmdAdd is called for ADD requests
func cmdAdd(args *skel.CmdArgs) error {

	conf, err := parseConfig(args.StdinData)
	if err != nil {
		return err
	}

	podName, podNameSpace, err := parseArgs(args.Args)
	if err != nil {
		return err
	}

	kubeclient, err := initKubeclient(conf)
	if err != nil {
		return err
	}

	//默认都检查vp是否正常挂载
	//if !conf.VolumepathNoCheck {
	//	if err := CheckvolumePathMount(kubeclient, podNameSpace, podName); err != nil {
	//		return err
	//	}
	//}

	//获取执行配置
	networkRuntimeCfg, err := prepareNetworkeConfig(kubeclient, podName, podNameSpace)
	if err != nil {
		return err
	}

	networkRuntimeCfg.Netns = args.Netns
	networkRuntimeCfg.IfName = args.IfName

	//执行脚本
	execFilePath := filepath.Join(conf.ShellDir, addnetworkFile)
	if err := doAddNetwork(execFilePath, networkRuntimeCfg); err != nil {
		return err
	}

	//update Bandwidth status
	// if err := updateCliamBandwidthStatus(kubeclient, podName, podNameSpace, runtimeCfg.Bandwidth); err != nil {
	// 	return err
	// }

	//返回结果
	_, ipnet, err := net.ParseCIDR(networkRuntimeCfg.podIP + "/" + networkRuntimeCfg.podMask)
	if err != nil {
		return fmt.Errorf("ParseCIDR(%s/%s) fail:%s", networkRuntimeCfg.podIP, networkRuntimeCfg.podMask, err.Error())
	}

	result := &result.Result{
		CNIVersion: "0.2.0",
		IP4: &result.IPConfig{
			IP: *ipnet,
		},
	}

	return result.Print()
}

func listNetDevice(shellDir, networktype, deviceType string) ([]NetDevice, error) {

	execFilePath := filepath.Join(shellDir, getNetDeviceFile)
	filter, err := json.Marshal(NetDevice{Type: deviceType, NetWorkType: networktype})
	if err != nil {
		return nil, fmt.Errorf("listNetDevice Marshal fail:%s", err.Error())
	}

	args := []string{"network", "list", fmt.Sprintf("%s", string(filter))}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	datas, err := exec.CommandContext(ctx, execFilePath, args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("listNetDevice  fail:%s(execfile: %s，args:%v),out:%s", err.Error(), execFilePath, args, datas)
	}

	type DevicesResult struct {
		NetDevices []NetDevice `json:"native_devices"`
	}

	result := DevicesResult{}
	err = json.Unmarshal(datas, &result)
	if err != nil {
		return nil, fmt.Errorf("listNetDevice Unmarshal fail:%s(datas:%s)", err.Error(), datas)
	}

	if len(result.NetDevices) == 0 {
		return nil, fmt.Errorf("listNetDevice get 0 device(execfile: %s，args:%v),out:%s", execFilePath, args, datas)
	}

	return result.NetDevices, nil

}

func doAddNetwork(execFilePath string, runtimeCfg *networkRuntimeCfg) error {

	cfgjson, err := json.Marshal(runtimeCfg)
	if err != nil {
		return fmt.Errorf("doAddNetwork Marshal fail:%s", err.Error())
	}

	args := []string{"network", "add", fmt.Sprintf("%s", cfgjson)}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if out, err := exec.CommandContext(ctx, execFilePath, args...).CombinedOutput(); err != nil {
		return fmt.Errorf("doAddNetwork  fail:%s(execfile: %s, args: %v)(out:%s)", err.Error(), execFilePath, args, out)
	}

	return nil
}

// cmdCheck is called for CHECK requests
func cmdCheck(args *skel.CmdArgs) error {
	return nil
}

// cmdDel is called for DELETE requests
func cmdDel(args *skel.CmdArgs) error {
	// conf, err := parseConfig(args.StdinData)
	// if err != nil {
	// 	return err
	// }
	// _ = conf

	// Do your delete here
	return nil
}

func main() {
	flag.Parse()

	if versionFlag {
		fmt.Println("the build version:", vars.GITCOMMIT, vars.BUILDTIME)
		return
	}

	skel.PluginMain(cmdAdd, cmdCheck, cmdDel, version.PluginSupports("0.1.0", "0.2.0", version.Current()), "network-plugin")
}

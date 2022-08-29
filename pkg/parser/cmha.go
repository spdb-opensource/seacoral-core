package parser

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/astaxie/beego/config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	unitv4 "github.com/upmio/dbscale-kube/pkg/apis/unit/v1alpha4"
	"github.com/upmio/dbscale-kube/pkg/structs"
)

type CmhaParser struct {
	ctx    context.Context
	config config.Configer

	template *corev1.ConfigMap
	client   ParserClient
	Unit     *unitv4.Unit
}

func (p *CmhaParser) ParseData(data string) error {
	configer, err := config.NewConfigData("json", []byte(data))
	if err != nil {
		return err
	}

	p.config = configer

	return nil
}

func (cp *CmhaParser) GenerateConfig() (string, error) {
	content, ok := cp.template.Data[unitv4.ConfigDataTab]
	if !ok {
		return "", fmt.Errorf(":%s(key:%s): not find content data", cp.template.GetName(), unitv4.ConfigDataTab)
	}

	err := cp.ParseData(content)
	if err != nil {
		return "", err
	}

	tmpl, err := cp.Marshal()
	if err != nil {
		return "", fmt.Errorf("marshal config data err: %s", err)
	}

	var configjson cmhaConfig

	err = json.Unmarshal([]byte(tmpl), &configjson)
	if err != nil {
		return "", fmt.Errorf("unmarshal template to json struct err: %s", err)
	}

	addrs, err := cp.getPodsAddr()
	if err != nil {
		return "", err
	}

	if addrs == nil {
		return "", fmt.Errorf("not get consul IP")
	}

	configjson.Consul.ClusterServers = addrs

	port := 9100
	find := false
	for _, container := range cp.Unit.Spec.Template.Spec.Containers {
		if container.Name == cp.Unit.Spec.MainContainerName {
			find = true
			for _, p := range container.Ports {
				if p.ContainerPort != 0 {
					port = int(p.ContainerPort)
					break
				}
			}
		}
	}

	if !find {
		klog.Errorf("not find %s container", cp.Unit.Spec.MainContainerName)
		return "", fmt.Errorf("not find %s container", cp.Unit.Spec.MainContainerName)
	}

	configjson.Switchmanager.Port = port

	pod, err := cp.client.KubeClient.CoreV1().Pods(cp.Unit.Namespace).Get(cp.ctx, cp.Unit.PodName(), metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	podIp, err := waitUntilNetworkClaimsReady(cp.ctx, cp.client.NetClient, pod)
	if err != nil {
		klog.Errorf("Cmha GenerateConfig waitUntilNetworkClaimsReady ERR: %s", err)
		return "", err
	}

	klog.Infof("pod.Status.PodIP: %s", podIp)
	configjson.Switchmanager.Address = podIp
	configjson.Consul.Address = podIp
	configjson.Discovery.URI = fmtDiscoveryUri(podIp)

	if gn, ok := cp.Unit.Labels[unitv4.LabelGroup]; ok {
		configjson.Service = append(configjson.Service, gn)
	}

	out, err := json.Marshal(configjson)

	return string(out), err
}

func fmtDiscoveryUri(podIP string) string {
	return fmt.Sprintf("consul://%s:8500/cmha/service", podIP)
}

func (cp *CmhaParser) getPodsAddr() ([]string, error) {
	ret, err := getPodGroups(cp.Unit)
	if err != nil {
		return nil, err
	}

	done := 0
	var addrs []string
	for k, _ := range ret {
		podname := strings.Split(k, "/")[1]
		pod, err := cp.client.KubeClient.CoreV1().Pods(cp.Unit.Namespace).Get(cp.ctx, podname, metav1.GetOptions{})
		if err != nil {
			klog.Errorf("GenerateConfig:CmhaParser:get pod: %s ERR: %v", podname, err)
			continue
		}

		ip, err := waitUntilNetworkClaimsReady(cp.ctx, cp.client.NetClient, pod)
		if err != nil {
			klog.Errorf("GenerateConfig:CmhaParser:get pod: %s IP ERR: %v", podname, err)
			continue
		}

		if ip == "" {
			klog.Errorf("GenerateConfig:CmhaParser: pod: %s doesn't have ip yet!!!", podname)
			continue
		}
		addrs = append(addrs, ip)
		done++

		if done != len(ret) {
			continue
		}
	}

	return addrs, nil
}

func (cp *CmhaParser) Marshal() (string, error) {
	file, err := ioutil.TempFile("", "serviceConfig")
	if err != nil {
		return "", err
	}
	file.Close()
	defer os.Remove(file.Name())

	err = cp.config.SaveConfigFile(file.Name())
	if err != nil {
		return "", err
	}

	data, err := ioutil.ReadFile(file.Name())
	return string(data), err
}

func (cp *CmhaParser) Set(key string, val interface{}) error {
	return fmt.Errorf("%s not support set config yet!", structs.CmhaServiceType)
}

func (cp *CmhaParser) Get(key string) (string, bool) {

	return "", false
}

type cmhaConfig struct {
	Switchmanager struct {
		Address       string `json:"address"`
		Port          int    `json:"port"`
		Logfile       string `json:"logfile"`
		Alertslogfile string `json:"alertslogfile"`
	} `json:"switchmanager"`
	Consul struct {
		Address        string   `json:"address"`
		Port           int      `json:"port"`
		Datacenter     string   `json:"datacenter"`
		Token          string   `json:"token"`
		ClusterServers []string `json:"cluster_servers"`
	} `json:"consul"`
	Discovery struct {
		Replication bool          `json:"replication"`
		URI         string        `json:"uri"`
		TTL         string        `json:"ttl"`
		Heartbeat   string        `json:"heartbeat"`
		Options     []interface{} `json:"options"`
	} `json:"discovery"`
	Service []string `json:"service"`
}

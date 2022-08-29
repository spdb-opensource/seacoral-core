package parser

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/astaxie/beego/config"
	"github.com/pkg/errors"
	unitv4 "github.com/upmio/dbscale-kube/pkg/apis/unit/v1alpha4"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ProxysqlParser struct {
	ctx    context.Context
	config config.Configer

	template *corev1.ConfigMap
	client   ParserClient
	Unit     *unitv4.Unit
}

func (p *ProxysqlParser) ParseData(data string) error {
	configer, err := config.NewConfigData("ini", []byte(data))
	if err != nil {
		return err
	}

	p.config = configer

	return nil
}

func (p *ProxysqlParser) GenerateConfig() (string, error) {
	content, ok := p.template.Data[unitv4.ConfigDataTab]
	if !ok {
		return "", fmt.Errorf(":%s(key:%s): not find content data", p.template.GetName(), unitv4.ConfigDataTab)
	}

	pod, err := p.client.KubeClient.CoreV1().Pods(p.Unit.Namespace).Get(p.ctx, p.Unit.PodName(), metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	err = p.ParseData(content)
	if err != nil {
		return "", err
	}

	m := make(map[string]interface{}, 20)

	port := 6033

	find := false
	for _, container := range p.Unit.Spec.Template.Spec.Containers {
		if container.Name == p.Unit.Spec.MainContainerName {
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
		return "", fmt.Errorf("not find %s container", p.Unit.Spec.MainContainerName)
	}

	oldAddr := p.config.String("mysql::interfaces")
	ni, err := changeInterface(oldAddr, strconv.Itoa(port))
	if err != nil {
		return "", err
	}

	m["mysql::interfaces"] = ni

	options, err := getConfigOptions(pod)
	if err != nil {
		return "", err
	}

	threads, ok := options["mysql::threads"]
	if ok {
		m["mysql::threads"] = threads
	}

	mc, ok := options["mysql::max_connections"]
	if ok {
		m["mysql::max_connections"] = mc
	}

	for key, val := range m {
		err = p.Set(key, val)
	}

	return p.Marshal()
}

func changeInterface(old, new string) (string, error) {
	//0.0.0.0:3306;/tmp/proxysql.sock
	//addr := strings.Split(old, ";")
	host, _, err := net.SplitHostPort(old)
	if err != nil {
		return "", errors.Errorf("net.SplitHostPort: %s ERR: %v", old, err)
	}

	newaddr := net.JoinHostPort(host, new)
	//newinterface := fmt.Sprintf("%s;%s", newaddr, addr[1])
	return newaddr, nil
}

func (p *ProxysqlParser) Marshal() (string, error) {
	file, err := ioutil.TempFile("", "serviceConfig")
	if err != nil {
		return "", err
	}
	file.Close()
	defer os.Remove(file.Name())

	err = p.config.SaveConfigFile(file.Name())
	if err != nil {
		return "", err
	}

	data, err := ioutil.ReadFile(file.Name())
	return string(data), err
}

func (p *ProxysqlParser) Set(key string, val interface{}) error {
	if p.config == nil {
		return fmt.Errorf("ProxysqlParser cofnig is null")
	}

	return p.config.Set(strings.ToLower(key), fmt.Sprintf("%v", val))
}

func (p *ProxysqlParser) Get(key string) (string, bool) {
	if p.config == nil {
		return "", false
	}

	if val, ok := beegoConfigString(p.config, key); ok {
		return val, ok
	}

	return "", false
}

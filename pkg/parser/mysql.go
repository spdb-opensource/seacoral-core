package parser

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"

	// "strconv"
	"strings"

	"github.com/astaxie/beego/config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	unitv4 "github.com/upmio/dbscale-kube/pkg/apis/unit/v1alpha4"
)

type MysqlParser struct {
	ctx    context.Context
	config config.Configer

	template *corev1.ConfigMap
	client   ParserClient
	Unit     *unitv4.Unit
}

func (p *MysqlParser) ParseData(data string) error {
	configer, err := config.NewConfigData("ini", []byte(data))
	if err != nil {
		return err
	}

	p.config = configer

	return nil
}

func (p *MysqlParser) GenerateConfig() (string, error) {

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

	// m["mysqld::bind_address"] = pod.Status.PodIP
	port := 3306

	memory := int64(0)
	cpu := int64(0)

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

			cpu = container.Resources.Requests.Cpu().Value()
			memory = container.Resources.Requests.Memory().Value()
			memory = memory >> 20 // MB
		}
	}

	if !find {
		return "", fmt.Errorf("not find %s container", p.Unit.Spec.MainContainerName)
	}

	options, err := getConfigOptions(pod)
	if err != nil {
		return "", err
	}

	character, ok := options["mysqld::character_set_server"]
	if !ok {
		return "", fmt.Errorf("not find mysqld::character_set_server from the options")
	}
	m["mysqld::character_set_server"] = character

	m["mysqld::server-id"] = options["service_id"]
	m["mysqld::port"] = port //strconv.Itoa(port)
	m["mysqld::innodb_thread_concurrency"] = cpu

	temp := ""
	if memory <= (4 * 1024) {
		temp = fmt.Sprintf("%v%s", int(float64(memory)*(50.00/100.00)), "M")
	} else if memory > (4*1024) && memory <= (16*1024) {
		temp = fmt.Sprintf("%v%s", int(float64(memory)*(60.00/100.00)), "M")
	} else {
		temp = fmt.Sprintf("%v%s", int(float64(memory)*(70.00/100.00)), "M")
	}
	m["mysqld::innodb_buffer_pool_size"] = temp

	//if cpu > 2 {
	//	_ = memory
	// m["mysqld::innodb_purge_threads"]=
	// m["mysqld::innodb_read_io_threads="]=
	// m["mysqld::innodb_write_io_threads="]=
	//}

	for key, val := range m {
		err = p.Set(key, val)
	}

	return p.Marshal()
}

func (p *MysqlParser) Marshal() (string, error) {
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

func (p *MysqlParser) Set(key string, val interface{}) error {
	if p.config == nil {
		return fmt.Errorf("MysqlParser cofnig is null")
	}

	return p.config.Set(strings.ToLower(key), fmt.Sprintf("%v", val))
}

func (p *MysqlParser) Get(key string) (string, bool) {
	if p.config == nil {
		return "", false
	}

	if val, ok := beegoConfigString(p.config, key); ok {
		return val, ok
	}

	return "", false
}

func beegoConfigString(config config.Configer, key string) (string, bool) {
	if val := config.String(key); val != "" {
		return val, true
	}

	section := "default"
	parts := strings.SplitN(key, "::", 2)
	if len(parts) == 2 {
		section = parts[0]
		key = parts[1]
	}

	m, err := config.GetSection(section)
	if err != nil || len(m) == 0 {
		return "", false
	}

	val, ok := m[key]

	return val, ok
}

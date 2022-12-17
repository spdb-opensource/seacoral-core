package bankend

import (
	"github.com/beego/beego/config"
	"io/ioutil"
	"os"
	"strings"
)

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

func marshal(config config.Configer) ([]byte, error) {
	file, err := ioutil.TempFile("", "serviceConfig")
	if err != nil {
		return nil, err
	}
	file.Close()
	defer os.Remove(file.Name())

	err = config.SaveConfigFile(file.Name())
	if err != nil {
		return nil, err
	}

	return ioutil.ReadFile(file.Name())
}

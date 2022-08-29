package utils

import (
	"os"
	"path/filepath"
)

func GetBaseKubeScriptDir() string {
	dir := os.Getenv("KUBE_SCRIPT_BASE_DIR")
	if dir == "" {
		return "/opt/kube/scripts/"
	}
	return dir
}

func GetNodeInitDir() string {
	bdir := GetBaseKubeScriptDir()
	return filepath.Join(bdir, "host-init")
}

// func GetNodeInitDir() string {
// 	dir := os.Getenv("NODE_INIT_DIR")
// 	if dir == "" {
// 		return "/opt/kube/scripts/host-init"
// 	}
// 	return dir
// }

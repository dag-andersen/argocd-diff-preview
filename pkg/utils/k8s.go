package utils

import (
	"k8s.io/client-go/util/homedir"
	"path/filepath"
)

func GetKubeConfigPath() string {
	if home := homedir.HomeDir(); home != "" {
		return filepath.Join(home, ".kube", "config")
	}
	return ""
}

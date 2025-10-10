package utils

import (
	"os"
	"path/filepath"

	"k8s.io/client-go/util/homedir"
)

// GetKubeConfigPath returns the path to the kubeconfig file and a boolean indicating if the file exists
func GetKubeConfigPath() (string, bool) {
	// Check KUBECONFIG environment variable first
	if kubeconfigPath := os.Getenv("KUBECONFIG"); kubeconfigPath != "" {
		_, err := os.Stat(kubeconfigPath)
		return kubeconfigPath, err == nil
	}

	// Fall back to default kubeconfig location
	if home := homedir.HomeDir(); home != "" {
		path := filepath.Join(home, ".kube", "config")
		_, err := os.Stat(path)
		return path, err == nil
	}
	return "", false
}

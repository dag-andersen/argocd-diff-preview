package utils

import (
	"os"

	"k8s.io/client-go/tools/clientcmd"
)

// GetKubeConfigPath returns the path to the kubeconfig file and a boolean indicating if the file exists
func GetKubeConfigPath() (string, bool) {
	// Check KUBECONFIG environment variable first
	if kubeconfigPath := os.Getenv("KUBECONFIG"); kubeconfigPath != "" {
		_, err := os.Stat(kubeconfigPath)
		return kubeconfigPath, err == nil
	}

	// Fall back to default kubeconfig location
	path := clientcmd.RecommendedHomeFile
	_, err := os.Stat(path)
	return path, err == nil
}

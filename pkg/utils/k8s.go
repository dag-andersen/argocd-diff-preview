package utils

import (
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/client-go/util/homedir"
	"sigs.k8s.io/yaml"
)

func GetKubeConfigPath() string {
	if home := homedir.HomeDir(); home != "" {
		return filepath.Join(home, ".kube", "config")
	}
	return ""
}

// Should set current context to the namespace
// current-context: context_1
// contexts:
// - context:
//     cluster: cluster_1
//     namespace: <namespace>
//     user: user_1
//   name: context_1

// Set namespace by editing the kubeconfig file
func SetNamespaceInKubeConfig(inCluster bool, namespace string) error {

	if inCluster {
		namespacePath := "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

		// override KUBERNETES_NAMESPACE
		err := os.Setenv("KUBERNETES_NAMESPACE", namespace)
		if err != nil {
			return fmt.Errorf("failed to set KUBERNETES_NAMESPACE: %w", err)
		}

		// override file in namespacePath
		err = os.WriteFile(namespacePath, []byte(namespace), 0644)
		if err != nil {
			return fmt.Errorf("failed to write namespace to file %s: %w", namespacePath, err)
		}

		return nil
	}

	path := GetKubeConfigPath()

	// read kubeconfig file
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read kubeconfig file: %w", err)
	}

	// parse yaml
	var kubeconfig map[string]interface{}
	if err := yaml.Unmarshal(content, &kubeconfig); err != nil {
		return fmt.Errorf("failed to unmarshal kubeconfig file: %w", err)
	}

	// get current context
	currentContext := kubeconfig["current-context"].(string)

	// loop over contexts and set namespace for the current context
	for _, context := range kubeconfig["contexts"].([]interface{}) {
		if context.(map[string]interface{})["name"].(string) == currentContext {
			context.(map[string]interface{})["context"].(map[string]interface{})["namespace"] = namespace
		}
	}

	// write kubeconfig file
	yamlContent, err := yaml.Marshal(kubeconfig)
	if err != nil {
		return fmt.Errorf("failed to marshal kubeconfig file: %w", err)
	}

	// write kubeconfig file
	if err := os.WriteFile(path, yamlContent, 0644); err != nil {
		return fmt.Errorf("failed to write kubeconfig file: %w", err)
	}

	return nil
}

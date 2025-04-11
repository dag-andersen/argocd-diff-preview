package utils

import (
	"context"
	"encoding/json"
	"path/filepath"

	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	//
	// Uncomment to load all auth plugins
	//
	// Uncomment to load all auth plugins
	// _ "k8s.io/client-go/plugin/pkg/client/auth"
	//
	// Or uncomment to load specific auth plugins
	// _ "k8s.io/client-go/plugin/pkg/client/auth/azure"
	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	// _ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

type K8sClient struct {
	clientset *dynamic.DynamicClient
}

func NewK8sClient() (*K8sClient, error) {

	var kubeconfig string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = filepath.Join(home, ".kube", "config")
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}

	clientset, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &K8sClient{clientset: clientset}, nil
}

func (c *K8sClient) GetArgoCDApplications(namespace string) (string, error) {
	applicationRes := schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: "applications"}

	// application := &unstructured.Unstructured{
	// 	Object: map[string]interface{}{
	// 		"apiVersion": "argoproj.io/v1alpha1",
	// 		"kind":       "Application",
	// 	},
	// }

	result, err := c.clientset.Resource(applicationRes).Namespace(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return "", err
	}

	// convert result to string
	resultString, err := json.Marshal(result)
	if err != nil {
		return "", err
	}

	WriteFile("applications.yaml", string(resultString))

	return string(resultString), nil
}

func (c *K8sClient) DeleteArgoCDApplications(namespace string) error {

	log.Info().Msg("🧼 Removing applications")

	applicationRes := schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: "applications"}

	apps, err := c.clientset.Resource(applicationRes).Namespace(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, app := range apps.Items {
		err := c.clientset.Resource(applicationRes).Namespace(namespace).Delete(context.Background(), app.GetName(), metav1.DeleteOptions{})
		if err != nil {
			log.Error().Err(err).Msgf("❌ Failed to delete application %s", app.GetName())
		}
	}
	log.Info().Msg("🧼 Deleted applications")
	return nil
}

func (c *K8sClient) ApplyManifest(namespace string, unstructured *unstructured.Unstructured) error {
	applicationRes := schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: "applications"}
	_, err := c.clientset.Resource(applicationRes).Namespace(unstructured.GetNamespace()).Apply(context.Background(), unstructured.GetName(), unstructured, metav1.ApplyOptions{})
	return err
}

package k8s

import "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

// Resource represents a Kubernetes resource from a YAML file
type Resource struct {
	FileName string
	Yaml     unstructured.Unstructured
}

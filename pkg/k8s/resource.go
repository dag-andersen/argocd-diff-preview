package k8s

import "gopkg.in/yaml.v3"

// Resource represents a Kubernetes resource from a YAML file
type Resource struct {
	FileName string
	Yaml     yaml.Node
}

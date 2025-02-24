package types

import "gopkg.in/yaml.v3"

// K8sResource represents a Kubernetes resource from a YAML file
type K8sResource struct {
	FileName string
	Yaml     yaml.Node
}

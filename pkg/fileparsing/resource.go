package fileparsing

import (
	"github.com/dag-andersen/argocd-diff-preview/pkg/git"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Resource represents a Kubernetes resource from a YAML file
type Resource struct {
	FileName string
	Yaml     unstructured.Unstructured
	Branch   git.BranchType
}

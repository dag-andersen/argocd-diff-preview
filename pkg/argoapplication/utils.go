package argoapplication

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

func yamlEqual(a, b *unstructured.Unstructured) bool {

	aStr, err := yaml.Marshal(a)
	if err != nil {
		return false
	}
	bStr, err := yaml.Marshal(b)
	if err != nil {
		return false
	}

	return string(aStr) == string(bStr)
}

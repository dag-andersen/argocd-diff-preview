package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestResource(t *testing.T) {
	tests := []struct {
		name     string
		fileName string
		yaml     unstructured.Unstructured
	}{
		{
			name:     "simple resource",
			fileName: "test.yaml",
			yaml:     unstructured.Unstructured{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resource := Resource{
				FileName: tt.fileName,
				Yaml:     tt.yaml,
			}

			assert.Equal(t, tt.fileName, resource.FileName)
			assert.Equal(t, tt.yaml, resource.Yaml)
		})
	}
}

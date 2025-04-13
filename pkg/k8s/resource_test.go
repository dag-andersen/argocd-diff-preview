package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestResource(t *testing.T) {
	tests := []struct {
		name     string
		fileName string
		yaml     yaml.Node
	}{
		{
			name:     "simple resource",
			fileName: "test.yaml",
			yaml: yaml.Node{
				Kind: yaml.DocumentNode,
			},
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

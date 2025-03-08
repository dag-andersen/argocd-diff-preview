package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestProcessYamlChunk(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		chunk    string
		want     []K8sResource
		wantLog  string
	}{
		{
			name:     "valid application yaml",
			filename: "test.yaml",
			chunk: `apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
spec:
  destination:
    namespace: default
    server: https://kubernetes.default.svc`,
			want: []K8sResource{
				{
					FileName: "test.yaml",
					Yaml: func() yaml.Node {
						var node yaml.Node
						yaml.Unmarshal([]byte(`apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
spec:
  destination:
    namespace: default
    server: https://kubernetes.default.svc`), &node)
						return node
					}(),
				},
			},
		},
		{
			name:     "invalid yaml",
			filename: "invalid.yaml",
			chunk:    "invalid: :",
			want:     nil,
			wantLog:  "⚠️ Failed to parse YAML in file 'invalid.yaml'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var resources []K8sResource
			processYamlChunk(tt.filename, tt.chunk, &resources)

			if tt.want == nil {
				assert.Empty(t, resources)
			} else {
				assert.Equal(t, tt.want[0].FileName, resources[0].FileName)
				assert.Equal(t, tt.want[0].Yaml, resources[0].Yaml)
			}
		})
	}
}

package argoapplicaiton

import (
	"testing"

	"github.com/dag-andersen/argocd-diff-preview/pkg/types"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestFromK8sResource(t *testing.T) {
	tests := []struct {
		name     string
		resource *types.K8sResource
		want     *ArgoResource
		wantErr  bool
	}{
		{
			name: "valid application",
			resource: &types.K8sResource{
				FileName: "test.yaml",
				Yaml: func() yaml.Node {
					var node yaml.Node
					yaml.Unmarshal([]byte(`apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  namespace: default
spec:
  destination:
    namespace: default`), &node)
					return node
				}(),
			},
			want: &ArgoResource{
				Kind:     Application,
				Name:     "test-app",
				FileName: "test.yaml",
			},
			wantErr: false,
		},
		{
			name: "valid application set",
			resource: &types.K8sResource{
				FileName: "test-set.yaml",
				Yaml: func() yaml.Node {
					var node yaml.Node
					yaml.Unmarshal([]byte(`apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: test-set
  namespace: default
spec:
  generators:
    - git:
        repoURL: https://github.com/org/repo.git`), &node)
					return node
				}(),
			},
			want: &ArgoResource{
				Kind:     ApplicationSet,
				Name:     "test-set",
				FileName: "test-set.yaml",
			},
			wantErr: false,
		},
		{
			name: "invalid kind",
			resource: &types.K8sResource{
				FileName: "test.yaml",
				Yaml: func() yaml.Node {
					var node yaml.Node
					yaml.Unmarshal([]byte(`apiVersion: argoproj.io/v1alpha1
kind: InvalidKind
metadata:
  name: test-app`), &node)
					return node
				}(),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "missing metadata",
			resource: &types.K8sResource{
				FileName: "test.yaml",
				Yaml: func() yaml.Node {
					var node yaml.Node
					yaml.Unmarshal([]byte(`apiVersion: argoproj.io/v1alpha1
kind: Application
spec:
  destination:
    namespace: default`), &node)
					return node
				}(),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "missing name",
			resource: &types.K8sResource{
				FileName: "test.yaml",
				Yaml: func() yaml.Node {
					var node yaml.Node
					yaml.Unmarshal([]byte(`apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  namespace: default
spec:
  destination:
    namespace: default`), &node)
					return node
				}(),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "nil yaml",
			resource: &types.K8sResource{
				FileName: "test.yaml",
				Yaml:     yaml.Node{},
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FromK8sResource(tt.resource)

			if tt.wantErr {
				assert.Nil(t, got)
				return
			}

			assert.NotNil(t, got)
			assert.Equal(t, tt.want.Kind, got.Kind)
			assert.Equal(t, tt.want.Name, got.Name)
			assert.Equal(t, tt.want.FileName, got.FileName)

			// Verify YAML structure
			assert.NotNil(t, got.Yaml)

			// Get the root node - either the first document or the root itself
			rootNode := &tt.resource.Yaml
			if len(tt.resource.Yaml.Content) > 0 {
				rootNode = tt.resource.Yaml.Content[0]
			}

			// Verify the YAML content matches
			assert.Equal(t, rootNode.Content[0].Value, got.Yaml.Content[0].Value) // apiVersion
			assert.Equal(t, rootNode.Content[1].Value, got.Yaml.Content[1].Value) // kind
		})
	}
}

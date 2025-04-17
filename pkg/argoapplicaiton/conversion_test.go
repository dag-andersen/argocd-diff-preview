package argoapplicaiton

import (
	"testing"

	"github.com/dag-andersen/argocd-diff-preview/pkg/k8s"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

func TestFromK8sResource(t *testing.T) {
	tests := []struct {
		name     string
		resource *k8s.Resource
		want     *ArgoResource
		wantErr  bool
	}{
		{
			name: "valid application",
			resource: &k8s.Resource{
				FileName: "test.yaml",
				Yaml: func() unstructured.Unstructured {
					var obj unstructured.Unstructured
					err := yaml.Unmarshal([]byte(`apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  namespace: default
spec:
  destination:
    namespace: default`), &obj)
					if err != nil {
						t.Fatalf("Failed to unmarshal YAML: %v", err)
					}
					return obj
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
			resource: &k8s.Resource{
				FileName: "test-set.yaml",
				Yaml: func() unstructured.Unstructured {
					var obj unstructured.Unstructured
					err := yaml.Unmarshal([]byte(`apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: test-set
  namespace: default
spec:
  generators:
    - git:
        repoURL: https://github.com/org/repo.git`), &obj)
					if err != nil {
						t.Fatalf("Failed to unmarshal YAML: %v", err)
					}
					return obj
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
			resource: &k8s.Resource{
				FileName: "test.yaml",
				Yaml: func() unstructured.Unstructured {
					var obj unstructured.Unstructured
					err := yaml.Unmarshal([]byte(`apiVersion: argoproj.io/v1alpha1
kind: InvalidKind
metadata:
  name: test-app`), &obj)
					if err != nil {
						t.Fatalf("Failed to unmarshal YAML: %v", err)
					}
					return obj
				}(),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "missing metadata",
			resource: &k8s.Resource{
				FileName: "test.yaml",
				Yaml: func() unstructured.Unstructured {
					var obj unstructured.Unstructured
					err := yaml.Unmarshal([]byte(`apiVersion: argoproj.io/v1alpha1
kind: Application
spec:
  destination:
    namespace: default`), &obj)
					if err != nil {
						t.Fatalf("Failed to unmarshal YAML: %v", err)
					}
					return obj
				}(),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "missing name",
			resource: &k8s.Resource{
				FileName: "test.yaml",
				Yaml: func() unstructured.Unstructured {
					var obj unstructured.Unstructured
					err := yaml.Unmarshal([]byte(`apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  namespace: default
spec:
  destination:
    namespace: default`), &obj)
					if err != nil {
						t.Fatalf("Failed to unmarshal YAML: %v", err)
					}
					return obj
				}(),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "nil yaml",
			resource: &k8s.Resource{
				FileName: "test.yaml",
				Yaml:     unstructured.Unstructured{},
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FromK8sResource(*tt.resource)

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

			// Verify the yaml is equal
			assert.True(t, yamlEqual(&tt.resource.Yaml, got.Yaml))
		})
	}
}

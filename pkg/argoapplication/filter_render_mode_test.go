package argoapplication

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestGetRenderMode(t *testing.T) {
	tests := []struct {
		name        string
		resource    *ArgoResource
		expectedMode RenderMode
	}{
		{
			name:        "nil yaml defaults to changed",
			resource:    &ArgoResource{Yaml: nil},
			expectedMode: RenderChanged,
		},
		{
			name: "no annotations defaults to changed",
			resource: &ArgoResource{Yaml: &unstructured.Unstructured{Object: map[string]any{
				"metadata": map[string]any{},
			}}},
			expectedMode: RenderChanged,
		},
		{
			name: "render always",
			resource: &ArgoResource{Yaml: &unstructured.Unstructured{Object: map[string]any{
				"metadata": map[string]any{
					"annotations": map[string]any{
						"argocd-diff-preview/render": "always",
					},
				},
			}}},
			expectedMode: RenderAlways,
		},
		{
			name: "render never",
			resource: &ArgoResource{Yaml: &unstructured.Unstructured{Object: map[string]any{
				"metadata": map[string]any{
					"annotations": map[string]any{
						"argocd-diff-preview/render": "never",
					},
				},
			}}},
			expectedMode: RenderNever,
		},
		{
			name: "render changed",
			resource: &ArgoResource{Yaml: &unstructured.Unstructured{Object: map[string]any{
				"metadata": map[string]any{
					"annotations": map[string]any{
						"argocd-diff-preview/render": "changed",
					},
				},
			}}},
			expectedMode: RenderChanged,
		},
		{
			name: "render always with whitespace and casing",
			resource: &ArgoResource{Yaml: &unstructured.Unstructured{Object: map[string]any{
				"metadata": map[string]any{
					"annotations": map[string]any{
						"argocd-diff-preview/render": "  AlWaYs  ",
					},
				},
			}}},
			expectedMode: RenderAlways,
		},
		{
			name: "invalid render value defaults to changed",
			resource: &ArgoResource{Yaml: &unstructured.Unstructured{Object: map[string]any{
				"metadata": map[string]any{
					"annotations": map[string]any{
						"argocd-diff-preview/render": "true",
					},
				},
			}}},
			expectedMode: RenderChanged,
		},
		{
			name: "legacy ignore true maps to render never",
			resource: &ArgoResource{Yaml: &unstructured.Unstructured{Object: map[string]any{
				"metadata": map[string]any{
					"annotations": map[string]any{
						"argocd-diff-preview/ignore": "true",
					},
				},
			}}},
			expectedMode: RenderNever,
		},
		{
			name: "render annotation takes precedence over legacy ignore",
			resource: &ArgoResource{Yaml: &unstructured.Unstructured{Object: map[string]any{
				"metadata": map[string]any{
					"annotations": map[string]any{
						"argocd-diff-preview/render": "always",
						"argocd-diff-preview/ignore": "true",
					},
				},
			}}},
			expectedMode: RenderAlways,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := tt.resource.GetRenderMode()
			assert.Equal(t, tt.expectedMode, actual)
		})
	}
}

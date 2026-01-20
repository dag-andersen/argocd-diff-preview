package extract

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/dag-andersen/argocd-diff-preview/pkg/argoapplication"
	"github.com/dag-andersen/argocd-diff-preview/pkg/git"
)

func TestNormalizeManifests_RemovesTrackingIDAndIgnoresFields(t *testing.T) {
	appYaml := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata": map[string]any{
				"name": "app1",
			},
			"spec": map[string]any{
				"ignoreDifferences": []any{
					map[string]any{
						"group":        "",
						"kind":         "ConfigMap",
						"jsonPointers": []any{"/data/ignored"},
					},
				},
			},
		},
	}

	app := argoapplication.ArgoResource{
		Yaml:   appYaml,
		Kind:   argoapplication.Application,
		Id:     "app1",
		Name:   "app1",
		Branch: git.Target,
	}

	manifest := unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name": "app1",
				"annotations": map[string]any{
					"argocd.argoproj.io/tracking-id": "tracking",
				},
			},
			"data": map[string]any{
				"ignored": "remove-me",
				"keep":    "ok",
			},
		},
	}

	normalized, err := NormalizeManifests([]unstructured.Unstructured{manifest}, app)
	require.NoError(t, err)
	require.Len(t, normalized, 1)

	annotations := normalized[0].GetAnnotations()
	_, trackingExists := annotations["argocd.argoproj.io/tracking-id"]
	assert.False(t, trackingExists)

	data, found, err := unstructured.NestedMap(normalized[0].Object, "data")
	require.NoError(t, err)
	require.True(t, found)
	_, ignoredExists := data["ignored"]
	assert.False(t, ignoredExists)
	assert.Equal(t, "ok", data["keep"])
}

func TestNormalizeManifests_NoAppYamlStillRemovesTracking(t *testing.T) {
	manifest := unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name": "app1",
				"annotations": map[string]any{
					"argocd.argoproj.io/tracking-id": "tracking",
				},
			},
		},
	}

	normalized, err := NormalizeManifests([]unstructured.Unstructured{manifest}, argoapplication.ArgoResource{})
	require.NoError(t, err)
	require.Len(t, normalized, 1)

	annotations := normalized[0].GetAnnotations()
	_, trackingExists := annotations["argocd.argoproj.io/tracking-id"]
	assert.False(t, trackingExists)
}

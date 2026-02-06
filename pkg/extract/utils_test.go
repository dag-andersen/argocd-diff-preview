package extract

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestReplaceStringInObject(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		oldStr   string
		newStr   string
		expected any
	}{
		{
			name:     "simple string replacement",
			input:    "hello-app-id-123-world",
			oldStr:   "app-id-123",
			newStr:   "my-app",
			expected: "hello-my-app-world",
		},
		{
			name:     "string with no match",
			input:    "hello world",
			oldStr:   "app-id-123",
			newStr:   "my-app",
			expected: "hello world",
		},
		{
			name:     "empty string",
			input:    "",
			oldStr:   "app-id-123",
			newStr:   "my-app",
			expected: "",
		},
		{
			name:     "multiple occurrences in string",
			input:    "app-id-123 and app-id-123 again",
			oldStr:   "app-id-123",
			newStr:   "my-app",
			expected: "my-app and my-app again",
		},
		{
			name:     "integer unchanged",
			input:    42,
			oldStr:   "42",
			newStr:   "99",
			expected: 42,
		},
		{
			name:     "float64 unchanged",
			input:    3.14,
			oldStr:   "3.14",
			newStr:   "2.71",
			expected: 3.14,
		},
		{
			name:     "boolean unchanged",
			input:    true,
			oldStr:   "true",
			newStr:   "false",
			expected: true,
		},
		{
			name:     "nil unchanged",
			input:    nil,
			oldStr:   "nil",
			newStr:   "null",
			expected: nil,
		},
		{
			name: "simple map",
			input: map[string]any{
				"name":  "app-id-123",
				"value": "test-app-id-123-value",
			},
			oldStr: "app-id-123",
			newStr: "my-app",
			expected: map[string]any{
				"name":  "my-app",
				"value": "test-my-app-value",
			},
		},
		{
			name: "map with mixed types",
			input: map[string]any{
				"name":    "app-id-123",
				"count":   42,
				"enabled": true,
				"ratio":   3.14,
			},
			oldStr: "app-id-123",
			newStr: "my-app",
			expected: map[string]any{
				"name":    "my-app",
				"count":   42,
				"enabled": true,
				"ratio":   3.14,
			},
		},
		{
			name: "simple array",
			input: []any{
				"app-id-123",
				"other-app-id-123-value",
				"no-match",
			},
			oldStr: "app-id-123",
			newStr: "my-app",
			expected: []any{
				"my-app",
				"other-my-app-value",
				"no-match",
			},
		},
		{
			name: "array with mixed types",
			input: []any{
				"app-id-123",
				42,
				true,
				3.14,
			},
			oldStr: "app-id-123",
			newStr: "my-app",
			expected: []any{
				"my-app",
				42,
				true,
				3.14,
			},
		},
		{
			name: "nested map",
			input: map[string]any{
				"metadata": map[string]any{
					"name": "app-id-123",
					"labels": map[string]any{
						"app": "app-id-123",
					},
				},
			},
			oldStr: "app-id-123",
			newStr: "my-app",
			expected: map[string]any{
				"metadata": map[string]any{
					"name": "my-app",
					"labels": map[string]any{
						"app": "my-app",
					},
				},
			},
		},
		{
			name: "nested array in map",
			input: map[string]any{
				"items": []any{
					"app-id-123",
					map[string]any{
						"name": "app-id-123",
					},
				},
			},
			oldStr: "app-id-123",
			newStr: "my-app",
			expected: map[string]any{
				"items": []any{
					"my-app",
					map[string]any{
						"name": "my-app",
					},
				},
			},
		},
		{
			name: "deeply nested structure",
			input: map[string]any{
				"level1": map[string]any{
					"level2": map[string]any{
						"level3": map[string]any{
							"value": "app-id-123",
						},
					},
				},
			},
			oldStr: "app-id-123",
			newStr: "my-app",
			expected: map[string]any{
				"level1": map[string]any{
					"level2": map[string]any{
						"level3": map[string]any{
							"value": "my-app",
						},
					},
				},
			},
		},
		{
			name:     "empty map",
			input:    map[string]any{},
			oldStr:   "app-id-123",
			newStr:   "my-app",
			expected: map[string]any{},
		},
		{
			name:     "empty array",
			input:    []any{},
			oldStr:   "app-id-123",
			newStr:   "my-app",
			expected: []any{},
		},
		{
			name: "map keys are not replaced",
			input: map[string]any{
				"app-id-123": "value",
			},
			oldStr: "app-id-123",
			newStr: "my-app",
			expected: map[string]any{
				"app-id-123": "value", // key unchanged
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := replaceStringInObject(tt.input, tt.oldStr, tt.newStr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestReplaceAppIdInManifests(t *testing.T) {
	tests := []struct {
		name      string
		manifests []unstructured.Unstructured
		oldId     string
		newName   string
		expected  []unstructured.Unstructured
	}{
		{
			name:      "empty manifests",
			manifests: []unstructured.Unstructured{},
			oldId:     "app-id-123",
			newName:   "my-app",
			expected:  []unstructured.Unstructured{},
		},
		{
			name: "same oldId and newName does nothing",
			manifests: []unstructured.Unstructured{
				{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]any{
							"name": "my-app",
						},
					},
				},
			},
			oldId:   "my-app",
			newName: "my-app",
			expected: []unstructured.Unstructured{
				{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]any{
							"name": "my-app",
						},
					},
				},
			},
		},
		{
			name: "single manifest with replacement in name",
			manifests: []unstructured.Unstructured{
				{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]any{
							"name": "app-id-123-config",
						},
					},
				},
			},
			oldId:   "app-id-123",
			newName: "my-app",
			expected: []unstructured.Unstructured{
				{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]any{
							"name": "my-app-config",
						},
					},
				},
			},
		},
		{
			name: "manifest with replacement in annotations",
			manifests: []unstructured.Unstructured{
				{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]any{
							"name": "config",
							"annotations": map[string]any{
								"argocd.argoproj.io/tracking-id": "app-id-123:ConfigMap:default/config",
							},
						},
					},
				},
			},
			oldId:   "app-id-123",
			newName: "my-app",
			expected: []unstructured.Unstructured{
				{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]any{
							"name": "config",
							"annotations": map[string]any{
								"argocd.argoproj.io/tracking-id": "my-app:ConfigMap:default/config",
							},
						},
					},
				},
			},
		},
		{
			name: "manifest with replacement in labels",
			manifests: []unstructured.Unstructured{
				{
					Object: map[string]any{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]any{
							"name": "deployment",
							"labels": map[string]any{
								"app.kubernetes.io/instance": "app-id-123",
							},
						},
					},
				},
			},
			oldId:   "app-id-123",
			newName: "my-app",
			expected: []unstructured.Unstructured{
				{
					Object: map[string]any{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]any{
							"name": "deployment",
							"labels": map[string]any{
								"app.kubernetes.io/instance": "my-app",
							},
						},
					},
				},
			},
		},
		{
			name: "multiple manifests",
			manifests: []unstructured.Unstructured{
				{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]any{
							"name": "app-id-123-config",
						},
					},
				},
				{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "Secret",
						"metadata": map[string]any{
							"name": "app-id-123-secret",
						},
					},
				},
			},
			oldId:   "app-id-123",
			newName: "my-app",
			expected: []unstructured.Unstructured{
				{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]any{
							"name": "my-app-config",
						},
					},
				},
				{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "Secret",
						"metadata": map[string]any{
							"name": "my-app-secret",
						},
					},
				},
			},
		},
		{
			name: "replacement in spec fields",
			manifests: []unstructured.Unstructured{
				{
					Object: map[string]any{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]any{
							"name": "deployment",
						},
						"spec": map[string]any{
							"selector": map[string]any{
								"matchLabels": map[string]any{
									"app": "app-id-123",
								},
							},
							"template": map[string]any{
								"metadata": map[string]any{
									"labels": map[string]any{
										"app": "app-id-123",
									},
								},
							},
						},
					},
				},
			},
			oldId:   "app-id-123",
			newName: "my-app",
			expected: []unstructured.Unstructured{
				{
					Object: map[string]any{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]any{
							"name": "deployment",
						},
						"spec": map[string]any{
							"selector": map[string]any{
								"matchLabels": map[string]any{
									"app": "my-app",
								},
							},
							"template": map[string]any{
								"metadata": map[string]any{
									"labels": map[string]any{
										"app": "my-app",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "replacement in array values",
			manifests: []unstructured.Unstructured{
				{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]any{
							"name": "config",
						},
						"data": map[string]any{
							"hosts": []any{
								"app-id-123.example.com",
								"api.app-id-123.example.com",
							},
						},
					},
				},
			},
			oldId:   "app-id-123",
			newName: "my-app",
			expected: []unstructured.Unstructured{
				{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]any{
							"name": "config",
						},
						"data": map[string]any{
							"hosts": []any{
								"my-app.example.com",
								"api.my-app.example.com",
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy to verify in-place modification
			manifests := make([]unstructured.Unstructured, len(tt.manifests))
			for i, m := range tt.manifests {
				manifests[i] = *m.DeepCopy()
			}

			replaceAppIdInManifests(manifests, tt.oldId, tt.newName)

			require.Len(t, manifests, len(tt.expected))
			for i := range manifests {
				assert.Equal(t, tt.expected[i].Object, manifests[i].Object)
			}
		})
	}
}

func TestReplaceAppIdInManifests_ModifiesInPlace(t *testing.T) {
	manifests := []unstructured.Unstructured{
		{
			Object: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]any{
					"name": "app-id-123-config",
				},
			},
		},
	}

	// Keep a reference to the original slice
	originalSlice := manifests

	replaceAppIdInManifests(manifests, "app-id-123", "my-app")

	// Verify the same slice was modified (not a new one created)
	assert.Same(t, &originalSlice[0], &manifests[0])

	// Verify the modification happened
	name, _, _ := unstructured.NestedString(manifests[0].Object, "metadata", "name")
	assert.Equal(t, "my-app-config", name)
}

func TestReplaceStringInObject_RealisticKubernetesManifest(t *testing.T) {
	// Use float64 and int64 for numeric types, as that's what YAML unmarshaling produces
	input := map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]any{
			"name":      "app-id-123",
			"namespace": "default",
			"labels": map[string]any{
				"app.kubernetes.io/name":     "app-id-123",
				"app.kubernetes.io/instance": "app-id-123",
			},
			"annotations": map[string]any{
				"argocd.argoproj.io/tracking-id": "app-id-123:apps/Deployment:default/app-id-123",
			},
		},
		"spec": map[string]any{
			"replicas": int64(3),
			"selector": map[string]any{
				"matchLabels": map[string]any{
					"app": "app-id-123",
				},
			},
			"template": map[string]any{
				"metadata": map[string]any{
					"labels": map[string]any{
						"app": "app-id-123",
					},
				},
				"spec": map[string]any{
					"containers": []any{
						map[string]any{
							"name":  "app-id-123",
							"image": "nginx:latest",
							"ports": []any{
								map[string]any{
									"containerPort": int64(80),
								},
							},
						},
					},
				},
			},
		},
	}

	result := replaceStringInObject(input, "app-id-123", "my-app")
	resultMap := result.(map[string]any)

	// Verify various nested replacements
	name, _, _ := unstructured.NestedString(resultMap, "metadata", "name")
	assert.Equal(t, "my-app", name)

	instanceLabel, _, _ := unstructured.NestedString(resultMap, "metadata", "labels", "app.kubernetes.io/instance")
	assert.Equal(t, "my-app", instanceLabel)

	trackingId, _, _ := unstructured.NestedString(resultMap, "metadata", "annotations", "argocd.argoproj.io/tracking-id")
	assert.Equal(t, "my-app:apps/Deployment:default/my-app", trackingId)

	selectorLabel, _, _ := unstructured.NestedString(resultMap, "spec", "selector", "matchLabels", "app")
	assert.Equal(t, "my-app", selectorLabel)

	// Verify non-string fields unchanged
	replicas, _, _ := unstructured.NestedInt64(resultMap, "spec", "replicas")
	assert.Equal(t, int64(3), replicas)

	containers, _, _ := unstructured.NestedSlice(resultMap, "spec", "template", "spec", "containers")
	require.Len(t, containers, 1)
	containerName, _, _ := unstructured.NestedString(containers[0].(map[string]any), "name")
	assert.Equal(t, "my-app", containerName)
}

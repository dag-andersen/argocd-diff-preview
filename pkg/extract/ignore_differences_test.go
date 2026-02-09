package extract

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/dag-andersen/argocd-diff-preview/pkg/argoapplication"
	"github.com/dag-andersen/argocd-diff-preview/pkg/git"
)

func TestParseIgnoreDifferencesFromApp(t *testing.T) {
	tests := []struct {
		name          string
		appObj        map[string]any
		expectedRules []ignoreDifferenceRule
		expectedCount int
	}{
		{
			name: "Application with spec.ignoreDifferences",
			appObj: map[string]any{
				"apiVersion": "argoproj.io/v1alpha1",
				"kind":       "Application",
				"metadata": map[string]any{
					"name":      "test-controller",
					"namespace": "argocd",
				},
				"spec": map[string]any{
					"ignoreDifferences": []any{
						map[string]any{
							"group":        "admissionregistration.k8s.io",
							"kind":         "ValidatingWebhookConfiguration",
							"name":         "example-webhook-validations",
							"jsonPointers": []any{"/webhooks/0/clientConfig/caBundle"},
						},
						map[string]any{
							"group":        "",
							"kind":         "Secret",
							"name":         "example-webhook-ca-keypair",
							"namespace":    "example-system",
							"jsonPointers": []any{"/data"},
						},
					},
				},
			},
			expectedRules: []ignoreDifferenceRule{
				{
					Group:        "admissionregistration.k8s.io",
					Kind:         "ValidatingWebhookConfiguration",
					Name:         "example-webhook-validations",
					JSONPointers: []string{"/webhooks/0/clientConfig/caBundle"},
				},
				{
					Group:        "",
					Kind:         "Secret",
					Name:         "example-webhook-ca-keypair",
					Namespace:    "example-system",
					JSONPointers: []string{"/data"},
				},
			},
			expectedCount: 2,
		},
		{
			name: "ApplicationSet with template.spec.ignoreDifferences",
			appObj: map[string]any{
				"apiVersion": "argoproj.io/v1alpha1",
				"kind":       "ApplicationSet",
				"metadata": map[string]any{
					"name":      "test-appset",
					"namespace": "argocd",
				},
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"ignoreDifferences": []any{
								map[string]any{
									"kind":         "Deployment",
									"jsonPointers": []any{"/metadata/annotations"},
								},
							},
						},
					},
				},
			},
			expectedRules: []ignoreDifferenceRule{
				{
					Kind:         "Deployment",
					JSONPointers: []string{"/metadata/annotations"},
				},
			},
			expectedCount: 1,
		},
		{
			name: "Application with jqPathExpressions",
			appObj: map[string]any{
				"apiVersion": "argoproj.io/v1alpha1",
				"kind":       "Application",
				"spec": map[string]any{
					"ignoreDifferences": []any{
						map[string]any{
							"kind":              "Deployment",
							"jqPathExpressions": []any{".spec.template.spec.containers[].image"},
						},
					},
				},
			},
			expectedRules: []ignoreDifferenceRule{
				{
					Kind:              "Deployment",
					JQPathExpressions: []string{".spec.template.spec.containers[].image"},
				},
			},
			expectedCount: 1,
		},
		{
			name: "Mixed jsonPointers and jqPathExpressions",
			appObj: map[string]any{
				"apiVersion": "argoproj.io/v1alpha1",
				"kind":       "Application",
				"spec": map[string]any{
					"ignoreDifferences": []any{
						map[string]any{
							"kind":              "Service",
							"jsonPointers":      []any{"/metadata/labels"},
							"jqPathExpressions": []any{".spec.ports[].nodePort"},
						},
					},
				},
			},
			expectedRules: []ignoreDifferenceRule{
				{
					Kind:              "Service",
					JSONPointers:      []string{"/metadata/labels"},
					JQPathExpressions: []string{".spec.ports[].nodePort"},
				},
			},
			expectedCount: 1,
		},
		{
			name: "Empty ignoreDifferences",
			appObj: map[string]any{
				"apiVersion": "argoproj.io/v1alpha1",
				"kind":       "Application",
				"spec": map[string]any{
					"ignoreDifferences": []any{},
				},
			},
			expectedRules: []ignoreDifferenceRule{},
			expectedCount: 0,
		},
		{
			name: "No ignoreDifferences field",
			appObj: map[string]any{
				"apiVersion": "argoproj.io/v1alpha1",
				"kind":       "Application",
				"spec":       map[string]any{},
			},
			expectedRules: []ignoreDifferenceRule{},
			expectedCount: 0,
		},
		{
			name: "Invalid rule - missing kind",
			appObj: map[string]any{
				"apiVersion": "argoproj.io/v1alpha1",
				"kind":       "Application",
				"spec": map[string]any{
					"ignoreDifferences": []any{
						map[string]any{
							"group":        "apps",
							"jsonPointers": []any{"/metadata/labels"},
						},
					},
				},
			},
			expectedRules: []ignoreDifferenceRule{},
			expectedCount: 0,
		},
		{
			name: "Invalid rule - missing jsonPointers and jqPathExpressions",
			appObj: map[string]any{
				"apiVersion": "argoproj.io/v1alpha1",
				"kind":       "Application",
				"spec": map[string]any{
					"ignoreDifferences": []any{
						map[string]any{
							"kind": "Deployment",
						},
					},
				},
			},
			expectedRules: []ignoreDifferenceRule{},
			expectedCount: 0,
		},
		{
			name:          "Nil yaml object",
			appObj:        nil,
			expectedRules: []ignoreDifferenceRule{},
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ar *argoapplication.ArgoResource
			if tt.appObj != nil {
				// Determine the resource type based on the kind in the object
				resourceType := argoapplication.Application
				if kind, ok := tt.appObj["kind"].(string); ok && kind == "ApplicationSet" {
					resourceType = argoapplication.ApplicationSet
				}
				ar = argoapplication.NewArgoResource(&unstructured.Unstructured{Object: tt.appObj}, resourceType, "test-app", "test-app", "app.yaml", git.Target)
			} else {
				ar = argoapplication.NewArgoResource(nil, argoapplication.Application, "test-app", "test-app", "app.yaml", git.Target)
			}

			rules := parseIgnoreDifferencesFromApp(*ar)

			require.Len(t, rules, tt.expectedCount)

			for i, expectedRule := range tt.expectedRules {
				actualRule := rules[i]
				assert.Equal(t, expectedRule.Group, actualRule.Group, "Group mismatch for rule %d", i)
				assert.Equal(t, expectedRule.Kind, actualRule.Kind, "Kind mismatch for rule %d", i)
				assert.Equal(t, expectedRule.Name, actualRule.Name, "Name mismatch for rule %d", i)
				assert.Equal(t, expectedRule.Namespace, actualRule.Namespace, "Namespace mismatch for rule %d", i)
				assert.Equal(t, expectedRule.JSONPointers, actualRule.JSONPointers, "JSONPointers mismatch for rule %d", i)
				assert.Equal(t, expectedRule.JQPathExpressions, actualRule.JQPathExpressions, "JQPathExpressions mismatch for rule %d", i)
			}
		})
	}
}

func TestApplyIgnoreDifferencesToManifests(t *testing.T) {
	tests := []struct {
		name      string
		manifests []unstructured.Unstructured
		rules     []ignoreDifferenceRule
		validate  func(t *testing.T, manifests []unstructured.Unstructured)
	}{
		{
			name: "JSON pointer deletion from webhook and secret",
			manifests: []unstructured.Unstructured{
				{Object: map[string]any{
					"apiVersion": "admissionregistration.k8s.io/v1",
					"kind":       "ValidatingWebhookConfiguration",
					"metadata": map[string]any{
						"name": "example-webhook-validations",
					},
					"webhooks": []any{
						map[string]any{
							"clientConfig": map[string]any{
								"caBundle": "SOMEBASE64CERT",
								"service": map[string]any{
									"name": "webhook-service",
								},
							},
						},
					},
				}},
				{Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "Secret",
					"metadata": map[string]any{
						"name":      "example-webhook-ca-keypair",
						"namespace": "example-system",
					},
					"data": map[string]any{
						"tls.crt": "BASE64DATA",
						"tls.key": "BASE64KEY",
					},
				}},
				{Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "Secret",
					"metadata": map[string]any{
						"name":      "other-secret",
						"namespace": "example-system",
					},
					"data": map[string]any{
						"password": "abc",
					},
				}},
			},
			rules: []ignoreDifferenceRule{
				{
					Group:        "admissionregistration.k8s.io",
					Kind:         "ValidatingWebhookConfiguration",
					Name:         "example-webhook-validations",
					JSONPointers: []string{"/webhooks/0/clientConfig/caBundle"},
				},
				{
					Group:        "",
					Kind:         "Secret",
					Name:         "example-webhook-ca-keypair",
					Namespace:    "example-system",
					JSONPointers: []string{"/data"},
				},
			},
			validate: func(t *testing.T, manifests []unstructured.Unstructured) {
				// Assert webhook caBundle removed but service remains
				webhooks, foundSlice, err := unstructured.NestedSlice(manifests[0].Object, "webhooks")
				require.NoError(t, err)
				require.True(t, foundSlice)
				require.GreaterOrEqual(t, len(webhooks), 1)
				firstWebhook, ok := webhooks[0].(map[string]any)
				require.True(t, ok)

				// caBundle should be gone
				got, found, err := unstructured.NestedString(firstWebhook, "clientConfig", "caBundle")
				require.NoError(t, err)
				assert.False(t, found)
				assert.Empty(t, got)

				// service should remain
				serviceName, found, err := unstructured.NestedString(firstWebhook, "clientConfig", "service", "name")
				require.NoError(t, err)
				assert.True(t, found)
				assert.Equal(t, "webhook-service", serviceName)

				// Assert secret1 data removed
				_, foundMap, err := unstructured.NestedMap(manifests[1].Object, "data")
				require.NoError(t, err)
				assert.False(t, foundMap)

				// Assert secret2 remains intact
				val, found, err := unstructured.NestedString(manifests[2].Object, "data", "password")
				require.NoError(t, err)
				assert.True(t, found)
				assert.Equal(t, "abc", val)
			},
		},
		{
			name: "Array element masking with JSON pointer",
			manifests: []unstructured.Unstructured{
				{Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]any{
						"name": "test-deployment",
					},
					"spec": map[string]any{
						"template": map[string]any{
							"spec": map[string]any{
								"containers": []any{
									map[string]any{
										"name":  "app",
										"image": "nginx:1.20",
									},
									map[string]any{
										"name":  "sidecar",
										"image": "busybox:latest",
									},
								},
							},
						},
					},
				}},
			},
			rules: []ignoreDifferenceRule{
				{
					Kind:         "Deployment",
					JSONPointers: []string{"/spec/template/spec/containers/1"},
				},
			},
			validate: func(t *testing.T, manifests []unstructured.Unstructured) {
				containers, found, err := unstructured.NestedSlice(manifests[0].Object, "spec", "template", "spec", "containers")
				require.NoError(t, err)
				require.True(t, found)
				require.Len(t, containers, 2)

				// First container should be unchanged
				firstContainer, ok := containers[0].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "app", firstContainer["name"])
				assert.Equal(t, "nginx:1.20", firstContainer["image"])

				// Second container should be masked
				assert.Equal(t, maskedValue, containers[1])
			},
		},
		{
			name: "No matching rules",
			manifests: []unstructured.Unstructured{
				{Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"metadata": map[string]any{
						"name": "test-config",
					},
					"data": map[string]any{
						"key": "value",
					},
				}},
			},
			rules: []ignoreDifferenceRule{
				{
					Kind:         "Secret",
					JSONPointers: []string{"/data"},
				},
			},
			validate: func(t *testing.T, manifests []unstructured.Unstructured) {
				// ConfigMap should remain unchanged
				val, found, err := unstructured.NestedString(manifests[0].Object, "data", "key")
				require.NoError(t, err)
				assert.True(t, found)
				assert.Equal(t, "value", val)
			},
		},
		{
			name: "Empty rules",
			manifests: []unstructured.Unstructured{
				{Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "Secret",
					"data": map[string]any{
						"key": "value",
					},
				}},
			},
			rules: []ignoreDifferenceRule{},
			validate: func(t *testing.T, manifests []unstructured.Unstructured) {
				// Secret should remain unchanged
				val, found, err := unstructured.NestedString(manifests[0].Object, "data", "key")
				require.NoError(t, err)
				assert.True(t, found)
				assert.Equal(t, "value", val)
			},
		},
		{
			name: "Multiple JSON pointers on same resource",
			manifests: []unstructured.Unstructured{
				{Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "Service",
					"metadata": map[string]any{
						"name": "test-service",
						"labels": map[string]any{
							"app": "test",
						},
						"annotations": map[string]any{
							"key": "value",
						},
					},
					"spec": map[string]any{
						"ports": []any{
							map[string]any{
								"port":     80,
								"nodePort": 30080,
							},
						},
					},
				}},
			},
			rules: []ignoreDifferenceRule{
				{
					Kind:         "Service",
					JSONPointers: []string{"/metadata/labels", "/spec/ports/0/nodePort"},
				},
			},
			validate: func(t *testing.T, manifests []unstructured.Unstructured) {
				// Labels should be removed
				_, found, err := unstructured.NestedMap(manifests[0].Object, "metadata", "labels")
				require.NoError(t, err)
				assert.False(t, found)

				// Annotations should remain
				val, found, err := unstructured.NestedString(manifests[0].Object, "metadata", "annotations", "key")
				require.NoError(t, err)
				assert.True(t, found)
				assert.Equal(t, "value", val)

				// Port should remain but nodePort should be deleted
				ports, found, err := unstructured.NestedSlice(manifests[0].Object, "spec", "ports")
				require.NoError(t, err)
				require.True(t, found)
				require.Len(t, ports, 1)

				firstPort, ok := ports[0].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, int64(80), firstPort["port"])
				_, exists := firstPort["nodePort"]
				assert.False(t, exists)
			},
		},
		{
			name: "Group matching with core group",
			manifests: []unstructured.Unstructured{
				{Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]any{
						"name": "test-pod",
					},
					"spec": map[string]any{
						"containers": []any{
							map[string]any{
								"name": "app",
							},
						},
					},
				}},
				{Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]any{
						"name": "test-deployment",
					},
					"spec": map[string]any{
						"replicas": 3,
					},
				}},
			},
			rules: []ignoreDifferenceRule{
				{
					Group:        "", // Core group
					Kind:         "Pod",
					JSONPointers: []string{"/spec/containers"},
				},
				{
					Group:        "apps",
					Kind:         "Deployment",
					JSONPointers: []string{"/spec/replicas"},
				},
			},
			validate: func(t *testing.T, manifests []unstructured.Unstructured) {
				// Pod containers should be removed
				_, found, err := unstructured.NestedSlice(manifests[0].Object, "spec", "containers")
				require.NoError(t, err)
				assert.False(t, found)

				// Deployment replicas should be removed
				_, found, err = unstructured.NestedInt64(manifests[1].Object, "spec", "replicas")
				require.NoError(t, err)
				assert.False(t, found)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make deep copies to avoid test interference
			manifestsCopy := make([]unstructured.Unstructured, len(tt.manifests))
			for i, m := range tt.manifests {
				manifestsCopy[i] = unstructured.Unstructured{
					Object: deepCopyValue(m.Object).(map[string]any),
				}
			}

			applyIgnoreDifferencesToManifests(manifestsCopy, tt.rules)

			tt.validate(t, manifestsCopy)
		})
	}
}

func TestDeleteOrMaskAtJSONPointer(t *testing.T) {
	tests := []struct {
		name     string
		obj      map[string]any
		pointer  string
		validate func(t *testing.T, obj map[string]any)
	}{
		{
			name: "Delete simple key",
			obj: map[string]any{
				"metadata": map[string]any{
					"name": "test",
					"labels": map[string]any{
						"app": "myapp",
					},
				},
			},
			pointer: "/metadata/labels",
			validate: func(t *testing.T, obj map[string]any) {
				metadata, ok := obj["metadata"].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "test", metadata["name"])
				_, exists := metadata["labels"]
				assert.False(t, exists)
			},
		},
		{
			name: "Delete nested key",
			obj: map[string]any{
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"containers": []any{
								map[string]any{
									"name":  "app",
									"image": "nginx",
								},
							},
						},
					},
				},
			},
			pointer: "/spec/template/spec",
			validate: func(t *testing.T, obj map[string]any) {
				template, found, err := unstructured.NestedMap(obj, "spec", "template")
				require.NoError(t, err)
				require.True(t, found)
				_, exists := template["spec"]
				assert.False(t, exists)
			},
		},
		{
			name: "Mask array element",
			obj: map[string]any{
				"items": []any{
					"first",
					"second",
					"third",
				},
			},
			pointer: "/items/1",
			validate: func(t *testing.T, obj map[string]any) {
				items, ok := obj["items"].([]any)
				require.True(t, ok)
				require.Len(t, items, 3)
				assert.Equal(t, "first", items[0])
				assert.Equal(t, maskedValue, items[1])
				assert.Equal(t, "third", items[2])
			},
		},
		{
			name: "Mask nested array element",
			obj: map[string]any{
				"spec": map[string]any{
					"containers": []any{
						map[string]any{
							"name": "app",
							"env": []any{
								map[string]any{
									"name":  "VAR1",
									"value": "val1",
								},
								map[string]any{
									"name":  "VAR2",
									"value": "val2",
								},
							},
						},
					},
				},
			},
			pointer: "/spec/containers/0/env/1",
			validate: func(t *testing.T, obj map[string]any) {
				containers, ok := obj["spec"].(map[string]any)["containers"].([]any)
				require.True(t, ok)
				require.Len(t, containers, 1)

				container, ok := containers[0].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "app", container["name"])

				env, ok := container["env"].([]any)
				require.True(t, ok)
				require.Len(t, env, 2)

				firstEnv, ok := env[0].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "VAR1", firstEnv["name"])
				assert.Equal(t, "val1", firstEnv["value"])

				assert.Equal(t, maskedValue, env[1])
			},
		},
		{
			name: "Invalid pointer - no leading slash",
			obj: map[string]any{
				"key": "value",
			},
			pointer: "key",
			validate: func(t *testing.T, obj map[string]any) {
				// Should remain unchanged
				assert.Equal(t, "value", obj["key"])
			},
		},
		{
			name: "Invalid pointer - empty string",
			obj: map[string]any{
				"key": "value",
			},
			pointer: "",
			validate: func(t *testing.T, obj map[string]any) {
				// Should remain unchanged
				assert.Equal(t, "value", obj["key"])
			},
		},
		{
			name: "Nonexistent path",
			obj: map[string]any{
				"existing": "value",
			},
			pointer: "/nonexistent/path",
			validate: func(t *testing.T, obj map[string]any) {
				// Should remain unchanged
				assert.Equal(t, "value", obj["existing"])
			},
		},
		{
			name: "Array index out of bounds",
			obj: map[string]any{
				"items": []any{"one", "two"},
			},
			pointer: "/items/5",
			validate: func(t *testing.T, obj map[string]any) {
				// Should remain unchanged
				items, ok := obj["items"].([]any)
				require.True(t, ok)
				assert.Len(t, items, 2)
				assert.Equal(t, "one", items[0])
				assert.Equal(t, "two", items[1])
			},
		},
		{
			name: "Array index negative",
			obj: map[string]any{
				"items": []any{"one", "two"},
			},
			pointer: "/items/-1",
			validate: func(t *testing.T, obj map[string]any) {
				// Should remain unchanged
				items, ok := obj["items"].([]any)
				require.True(t, ok)
				assert.Len(t, items, 2)
				assert.Equal(t, "one", items[0])
				assert.Equal(t, "two", items[1])
			},
		},
		{
			name: "JSON pointer escaping",
			obj: map[string]any{
				"path/with~slash": map[string]any{
					"nested~key": "value",
				},
			},
			pointer: "/path~1with~0slash/nested~0key",
			validate: func(t *testing.T, obj map[string]any) {
				parent, ok := obj["path/with~slash"].(map[string]any)
				require.True(t, ok)
				_, exists := parent["nested~key"]
				assert.False(t, exists)
			},
		},
		{
			name: "Root level deletion",
			obj: map[string]any{
				"metadata": map[string]any{
					"name": "test",
				},
				"spec": map[string]any{
					"replicas": 3,
				},
			},
			pointer: "/metadata",
			validate: func(t *testing.T, obj map[string]any) {
				_, exists := obj["metadata"]
				assert.False(t, exists)
				assert.Equal(t, int64(3), obj["spec"].(map[string]any)["replicas"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a deep copy to avoid test interference
			objCopy := make(map[string]any)
			for k, v := range tt.obj {
				objCopy[k] = deepCopyValue(v)
			}

			deleteOrMaskAtJSONPointer(objCopy, tt.pointer)
			tt.validate(t, objCopy)
		})
	}
}

func TestDecodeJSONPointerToken(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "No escaping needed",
			input:    "simple",
			expected: "simple",
		},
		{
			name:     "Tilde escape",
			input:    "key~0name",
			expected: "key~name",
		},
		{
			name:     "Slash escape",
			input:    "key~1name",
			expected: "key/name",
		},
		{
			name:     "Both escapes",
			input:    "key~0and~1name",
			expected: "key~and/name",
		},
		{
			name:     "Multiple tildes",
			input:    "~0~0~1~1",
			expected: "~~//",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := decodeJSONPointerToken(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Helper function for deep copying values
func deepCopyValue(v any) any {
	switch val := v.(type) {
	case map[string]any:
		copy := make(map[string]any)
		for k, v := range val {
			copy[k] = deepCopyValue(v)
		}
		return copy
	case []any:
		copy := make([]any, len(val))
		for i, v := range val {
			copy[i] = deepCopyValue(v)
		}
		return copy
	case int:
		return int64(val) // Convert int to int64 for consistency with JSON unmarshaling
	case int32:
		return int64(val)
	case int64:
		return val
	case float32:
		return float64(val)
	case float64:
		return val
	case bool:
		return val
	case string:
		return val
	case nil:
		return nil
	default:
		return v
	}
}

func TestRuleMatches(t *testing.T) {
	tests := []struct {
		testName  string
		rule      ignoreDifferenceRule
		group     string
		kind      string
		name      string
		namespace string
		expected  bool
	}{
		{
			testName: "Exact match all fields",
			rule: ignoreDifferenceRule{
				Group:     "apps",
				Kind:      "Deployment",
				Name:      "test-app",
				Namespace: "default",
			},
			group:     "apps",
			kind:      "Deployment",
			name:      "test-app",
			namespace: "default",
			expected:  true,
		},
		{
			testName: "Kind only match",
			rule: ignoreDifferenceRule{
				Kind: "Secret",
			},
			group:     "",
			kind:      "Secret",
			name:      "any-secret",
			namespace: "any-namespace",
			expected:  true,
		},
		{
			testName: "Case insensitive kind match",
			rule: ignoreDifferenceRule{
				Kind: "deployment",
			},
			group:     "apps",
			kind:      "Deployment",
			name:      "test-app",
			namespace: "default",
			expected:  true,
		},
		{
			testName: "Group mismatch",
			rule: ignoreDifferenceRule{
				Group: "networking.k8s.io",
				Kind:  "Ingress",
			},
			group:     "extensions",
			kind:      "Ingress",
			name:      "test-ingress",
			namespace: "default",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			result := ruleMatches(tt.rule, tt.group, tt.kind, tt.name, tt.namespace)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGroupFromAPIVersion(t *testing.T) {
	tests := []struct {
		testName   string
		apiVersion string
		expected   string
	}{
		{
			testName:   "Core group v1",
			apiVersion: "v1",
			expected:   "",
		},
		{
			testName:   "Apps group",
			apiVersion: "apps/v1",
			expected:   "apps",
		},
		{
			testName:   "Networking group",
			apiVersion: "networking.k8s.io/v1",
			expected:   "networking.k8s.io",
		},
		{
			testName:   "Empty string",
			apiVersion: "",
			expected:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			result := groupFromAPIVersion(tt.apiVersion)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestApplyJQPathExpression(t *testing.T) {
	tests := []struct {
		name     string
		obj      map[string]any
		expr     string
		validate func(t *testing.T, obj map[string]any)
	}{
		{
			name: "Simple field selection",
			obj: map[string]any{
				"metadata": map[string]any{
					"name": "test",
					"labels": map[string]any{
						"app": "myapp",
					},
				},
			},
			expr: ".metadata.labels",
			validate: func(t *testing.T, obj map[string]any) {
				metadata, ok := obj["metadata"].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "test", metadata["name"])
				_, exists := metadata["labels"]
				assert.False(t, exists)
			},
		},
		{
			name: "Array element selection",
			obj: map[string]any{
				"spec": map[string]any{
					"containers": []any{
						map[string]any{
							"name":  "app",
							"image": "nginx:1.20",
						},
						map[string]any{
							"name":  "sidecar",
							"image": "busybox:latest",
						},
					},
				},
			},
			expr: ".spec.containers[1]",
			validate: func(t *testing.T, obj map[string]any) {
				containers, found, err := unstructured.NestedSlice(obj, "spec", "containers")
				require.NoError(t, err)
				require.True(t, found)
				require.Len(t, containers, 2)

				firstContainer, ok := containers[0].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "app", firstContainer["name"])

				assert.Equal(t, maskedValue, containers[1])
			},
		},
		{
			name: "Array slice with all elements",
			obj: map[string]any{
				"spec": map[string]any{
					"containers": []any{
						map[string]any{
							"name":  "app",
							"image": "nginx:1.20",
						},
						map[string]any{
							"name":  "sidecar",
							"image": "busybox:latest",
						},
					},
				},
			},
			expr: ".spec.containers[].image",
			validate: func(t *testing.T, obj map[string]any) {
				containers, found, err := unstructured.NestedSlice(obj, "spec", "containers")
				require.NoError(t, err)
				require.True(t, found)
				require.Len(t, containers, 2)

				// Both containers should have their image fields removed
				firstContainer, ok := containers[0].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "app", firstContainer["name"])
				_, exists := firstContainer["image"]
				assert.False(t, exists)

				secondContainer, ok := containers[1].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "sidecar", secondContainer["name"])
				_, exists = secondContainer["image"]
				assert.False(t, exists)
			},
		},
		{
			name: "Nested field selection",
			obj: map[string]any{
				"spec": map[string]any{
					"template": map[string]any{
						"metadata": map[string]any{
							"labels": map[string]any{
								"app": "test",
							},
						},
						"spec": map[string]any{
							"containers": []any{
								map[string]any{
									"name": "app",
								},
							},
						},
					},
				},
			},
			expr: ".spec.template.metadata.labels",
			validate: func(t *testing.T, obj map[string]any) {
				metadata, found, err := unstructured.NestedMap(obj, "spec", "template", "metadata")
				require.NoError(t, err)
				require.True(t, found)
				_, exists := metadata["labels"]
				assert.False(t, exists)

				// Other fields should remain
				containers, found, err := unstructured.NestedSlice(obj, "spec", "template", "spec", "containers")
				require.NoError(t, err)
				require.True(t, found)
				require.Len(t, containers, 1)
			},
		},
		{
			name: "Invalid jq expression",
			obj: map[string]any{
				"key": "value",
			},
			expr: ".invalid[syntax",
			validate: func(t *testing.T, obj map[string]any) {
				// Should remain unchanged due to invalid expression
				assert.Equal(t, "value", obj["key"])
			},
		},
		{
			name: "Empty expression",
			obj: map[string]any{
				"key": "value",
			},
			expr: "",
			validate: func(t *testing.T, obj map[string]any) {
				// Should remain unchanged
				assert.Equal(t, "value", obj["key"])
			},
		},
		{
			name: "Expression with no matches",
			obj: map[string]any{
				"metadata": map[string]any{
					"name": "test",
				},
			},
			expr: ".spec.containers",
			validate: func(t *testing.T, obj map[string]any) {
				// Should remain unchanged since path doesn't exist
				assert.Equal(t, "test", obj["metadata"].(map[string]any)["name"])
			},
		},
		{
			name: "Complex jq expression with select",
			obj: map[string]any{
				"spec": map[string]any{
					"containers": []any{
						map[string]any{
							"name":  "app",
							"image": "nginx:1.20",
							"ports": []any{
								map[string]any{
									"containerPort": 80,
									"name":          "http",
								},
								map[string]any{
									"containerPort": 443,
									"name":          "https",
								},
							},
						},
					},
				},
			},
			expr: ".spec.containers[0].ports[].containerPort",
			validate: func(t *testing.T, obj map[string]any) {
				containers, ok := obj["spec"].(map[string]any)["containers"].([]any)
				require.True(t, ok)
				require.Len(t, containers, 1)

				container, ok := containers[0].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "app", container["name"])
				assert.Equal(t, "nginx:1.20", container["image"])

				ports, ok := container["ports"].([]any)
				require.True(t, ok)
				require.Len(t, ports, 2)

				// Both ports should have containerPort removed but name should remain
				firstPort, ok := ports[0].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "http", firstPort["name"])
				_, exists := firstPort["containerPort"]
				assert.False(t, exists)

				secondPort, ok := ports[1].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "https", secondPort["name"])
				_, exists = secondPort["containerPort"]
				assert.False(t, exists)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a deep copy to avoid test interference
			objCopy := make(map[string]any)
			for k, v := range tt.obj {
				objCopy[k] = deepCopyValue(v)
			}

			applyJQPathExpression(objCopy, tt.expr)
			tt.validate(t, objCopy)
		})
	}
}

func TestApplyIgnoreDifferencesWithJQPathExpressions(t *testing.T) {
	tests := []struct {
		name      string
		manifests []unstructured.Unstructured
		rules     []ignoreDifferenceRule
		validate  func(t *testing.T, manifests []unstructured.Unstructured)
	}{
		{
			name: "JQ path expression on deployment containers",
			manifests: []unstructured.Unstructured{
				{Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]any{
						"name": "test-deployment",
					},
					"spec": map[string]any{
						"template": map[string]any{
							"spec": map[string]any{
								"containers": []any{
									map[string]any{
										"name":  "app",
										"image": "nginx:1.20",
									},
									map[string]any{
										"name":  "sidecar",
										"image": "busybox:latest",
									},
								},
							},
						},
					},
				}},
			},
			rules: []ignoreDifferenceRule{
				{
					Kind:              "Deployment",
					JQPathExpressions: []string{".spec.template.spec.containers[].image"},
				},
			},
			validate: func(t *testing.T, manifests []unstructured.Unstructured) {
				containers, found, err := unstructured.NestedSlice(manifests[0].Object, "spec", "template", "spec", "containers")
				require.NoError(t, err)
				require.True(t, found)
				require.Len(t, containers, 2)

				// Both containers should have image removed but names should remain
				firstContainer, ok := containers[0].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "app", firstContainer["name"])
				_, exists := firstContainer["image"]
				assert.False(t, exists)

				secondContainer, ok := containers[1].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "sidecar", secondContainer["name"])
				_, exists = secondContainer["image"]
				assert.False(t, exists)
			},
		},
		{
			name: "Mixed JSON pointers and JQ path expressions",
			manifests: []unstructured.Unstructured{
				{Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "Service",
					"metadata": map[string]any{
						"name": "test-service",
						"labels": map[string]any{
							"app": "test",
						},
					},
					"spec": map[string]any{
						"ports": []any{
							map[string]any{
								"port":     80,
								"nodePort": 30080,
							},
							map[string]any{
								"port":     443,
								"nodePort": 30443,
							},
						},
					},
				}},
			},
			rules: []ignoreDifferenceRule{
				{
					Kind:              "Service",
					JSONPointers:      []string{"/metadata/labels"},
					JQPathExpressions: []string{".spec.ports[].nodePort"},
				},
			},
			validate: func(t *testing.T, manifests []unstructured.Unstructured) {
				// Labels should be removed by JSON pointer
				_, found, err := unstructured.NestedMap(manifests[0].Object, "metadata", "labels")
				require.NoError(t, err)
				assert.False(t, found)

				// NodePorts should be removed by JQ expression but ports should remain
				spec, ok := manifests[0].Object["spec"].(map[string]any)
				require.True(t, ok)
				ports, ok := spec["ports"].([]any)
				require.True(t, ok)
				require.Len(t, ports, 2)

				firstPort, ok := ports[0].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, int64(80), firstPort["port"])
				_, exists := firstPort["nodePort"]
				assert.False(t, exists)

				secondPort, ok := ports[1].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, int64(443), secondPort["port"])
				_, exists = secondPort["nodePort"]
				assert.False(t, exists)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make deep copies to avoid test interference
			manifestsCopy := make([]unstructured.Unstructured, len(tt.manifests))
			for i, m := range tt.manifests {
				manifestsCopy[i] = unstructured.Unstructured{
					Object: deepCopyValue(m.Object).(map[string]any),
				}
			}

			applyIgnoreDifferencesToManifests(manifestsCopy, tt.rules)

			tt.validate(t, manifestsCopy)
		})
	}
}

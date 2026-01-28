package extract

import (
	"testing"

	"github.com/dag-andersen/argocd-diff-preview/pkg/argoapplication"
	"github.com/dag-andersen/argocd-diff-preview/pkg/vars"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestNormalizeNamespaces(t *testing.T) {
	tests := []struct {
		name                     string
		manifests                []unstructured.Unstructured
		destNamespace            string
		namespacedResources      map[schema.GroupKind]bool
		appName                  string
		expectedNamespaces       []string          // expected namespace for each manifest (for ordered tests)
		expectedNamespacesByName map[string]string // expected namespace by resource name (for unordered tests)
		expectError              bool
	}{
		{
			name:          "empty destination namespace returns manifests unchanged",
			destNamespace: "",
			manifests: []unstructured.Unstructured{
				{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]any{
							"name": "test-cm",
						},
					},
				},
			},
			namespacedResources: map[schema.GroupKind]bool{
				{Group: "", Kind: "ConfigMap"}: true,
			},
			appName:            "test-app",
			expectedNamespaces: []string{""}, // unchanged
			expectError:        false,
		},
		{
			name:          "adds namespace to namespaced resource without namespace",
			destNamespace: "target-ns",
			manifests: []unstructured.Unstructured{
				{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]any{
							"name": "test-cm",
						},
					},
				},
			},
			namespacedResources: map[schema.GroupKind]bool{
				{Group: "", Kind: "ConfigMap"}: true,
			},
			appName:            "test-app",
			expectedNamespaces: []string{"target-ns"},
			expectError:        false,
		},
		{
			name:          "preserves existing namespace on namespaced resource",
			destNamespace: "target-ns",
			manifests: []unstructured.Unstructured{
				{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]any{
							"name":      "test-cm",
							"namespace": "existing-ns",
						},
					},
				},
			},
			namespacedResources: map[schema.GroupKind]bool{
				{Group: "", Kind: "ConfigMap"}: true,
			},
			appName:            "test-app",
			expectedNamespaces: []string{"existing-ns"},
			expectError:        false,
		},
		{
			name:          "clears namespace from cluster-scoped resource",
			destNamespace: "target-ns",
			manifests: []unstructured.Unstructured{
				{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "Namespace",
						"metadata": map[string]any{
							"name":      "my-namespace",
							"namespace": "should-be-cleared",
						},
					},
				},
			},
			namespacedResources: map[schema.GroupKind]bool{
				{Group: "", Kind: "Namespace"}: false, // cluster-scoped
			},
			appName:            "test-app",
			expectedNamespaces: []string{""}, // cleared
			expectError:        false,
		},
		{
			name:          "handles multiple manifests with mixed namespace scenarios",
			destNamespace: "target-ns",
			manifests: []unstructured.Unstructured{
				{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "ConfigMap",
						"metadata": map[string]any{
							"name": "cm-without-ns",
						},
					},
				},
				{
					Object: map[string]any{
						"apiVersion": "v1",
						"kind":       "Secret",
						"metadata": map[string]any{
							"name":      "secret-with-ns",
							"namespace": "other-ns",
						},
					},
				},
				{
					Object: map[string]any{
						"apiVersion": "rbac.authorization.k8s.io/v1",
						"kind":       "ClusterRole",
						"metadata": map[string]any{
							"name": "my-cluster-role",
						},
					},
				},
			},
			namespacedResources: map[schema.GroupKind]bool{
				{Group: "", Kind: "ConfigMap"}:                            true,
				{Group: "", Kind: "Secret"}:                               true,
				{Group: "rbac.authorization.k8s.io", Kind: "ClusterRole"}: false, // cluster-scoped
			},
			appName: "test-app",
			// Note: DeduplicateTargetObjects may reorder manifests, so we check by name->namespace map
			expectedNamespacesByName: map[string]string{
				"cm-without-ns":   "target-ns", // gets destination namespace added
				"secret-with-ns":  "other-ns",  // preserves existing namespace
				"my-cluster-role": "",          // cluster-scoped, namespace cleared
			},
			expectError: false,
		},
		{
			name:          "empty manifests slice returns empty slice",
			destNamespace: "target-ns",
			manifests:     []unstructured.Unstructured{},
			namespacedResources: map[schema.GroupKind]bool{
				{Group: "", Kind: "ConfigMap"}: true,
			},
			appName:            "test-app",
			expectedNamespaces: []string{},
			expectError:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := normalizeNamespaces(tt.manifests, tt.destNamespace, tt.namespacedResources, tt.appName)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)

			// If we have expectedNamespacesByName, check by name (order-independent)
			if tt.expectedNamespacesByName != nil {
				require.Len(t, result, len(tt.expectedNamespacesByName))
				for _, manifest := range result {
					name := manifest.GetName()
					expectedNs, ok := tt.expectedNamespacesByName[name]
					require.True(t, ok, "unexpected manifest name: %s", name)
					assert.Equal(t, expectedNs, manifest.GetNamespace(), "manifest %s has wrong namespace", name)
				}
			} else {
				// Check by position (order-dependent)
				require.Len(t, result, len(tt.expectedNamespaces))
				for i, expectedNs := range tt.expectedNamespaces {
					actualNs := result[i].GetNamespace()
					assert.Equal(t, expectedNs, actualNs, "manifest %d has wrong namespace", i)
				}
			}
		})
	}
}

func TestVerifyNoDuplicateAppIds(t *testing.T) {
	tests := []struct {
		name        string
		apps        []argoapplication.ArgoResource
		expectError bool
		errorMsg    string
	}{
		{
			name:        "empty slice returns no error",
			apps:        []argoapplication.ArgoResource{},
			expectError: false,
		},
		{
			name: "single app returns no error",
			apps: []argoapplication.ArgoResource{
				{Id: "app-1"},
			},
			expectError: false,
		},
		{
			name: "multiple unique apps returns no error",
			apps: []argoapplication.ArgoResource{
				{Id: "app-1"},
				{Id: "app-2"},
				{Id: "app-3"},
			},
			expectError: false,
		},
		{
			name: "duplicate app ids returns error",
			apps: []argoapplication.ArgoResource{
				{Id: "app-1"},
				{Id: "app-2"},
				{Id: "app-1"}, // duplicate
			},
			expectError: true,
			errorMsg:    "duplicate app name: app-1",
		},
		{
			name: "first two apps are duplicates",
			apps: []argoapplication.ArgoResource{
				{Id: "duplicate"},
				{Id: "duplicate"},
			},
			expectError: true,
			errorMsg:    "duplicate app name: duplicate",
		},
		{
			name: "multiple duplicates returns error for first duplicate found",
			apps: []argoapplication.ArgoResource{
				{Id: "app-a"},
				{Id: "app-b"},
				{Id: "app-a"}, // first duplicate
				{Id: "app-b"}, // second duplicate (won't be reached)
			},
			expectError: true,
			errorMsg:    "duplicate app name: app-a",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := verifyNoDuplicateAppIds(tt.apps)

			if tt.expectError {
				require.Error(t, err)
				assert.Equal(t, tt.errorMsg, err.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestLabelApplicationWithRunID(t *testing.T) {
	tests := []struct {
		name           string
		app            *argoapplication.ArgoResource
		runID          string
		existingLabels map[string]string
		expectedLabels map[string]string
	}{
		{
			name: "adds label to app with no existing labels",
			app: &argoapplication.ArgoResource{
				Yaml: &unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "argoproj.io/v1alpha1",
						"kind":       "Application",
						"metadata": map[string]any{
							"name": "test-app",
						},
					},
				},
			},
			runID: "run-123",
			expectedLabels: map[string]string{
				vars.ArgoCDApplicationLabelKey: "run-123",
			},
		},
		{
			name: "adds label to app with existing labels",
			app: &argoapplication.ArgoResource{
				Yaml: &unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "argoproj.io/v1alpha1",
						"kind":       "Application",
						"metadata": map[string]any{
							"name": "test-app",
							"labels": map[string]any{
								"existing-label": "existing-value",
								"another-label":  "another-value",
							},
						},
					},
				},
			},
			runID: "run-456",
			expectedLabels: map[string]string{
				"existing-label":               "existing-value",
				"another-label":                "another-value",
				vars.ArgoCDApplicationLabelKey: "run-456",
			},
		},
		{
			name: "overwrites existing run ID label",
			app: &argoapplication.ArgoResource{
				Yaml: &unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "argoproj.io/v1alpha1",
						"kind":       "Application",
						"metadata": map[string]any{
							"name": "test-app",
							"labels": map[string]any{
								vars.ArgoCDApplicationLabelKey: "old-run-id",
							},
						},
					},
				},
			},
			runID: "new-run-id",
			expectedLabels: map[string]string{
				vars.ArgoCDApplicationLabelKey: "new-run-id",
			},
		},
		{
			name: "handles empty run ID",
			app: &argoapplication.ArgoResource{
				Yaml: &unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "argoproj.io/v1alpha1",
						"kind":       "Application",
						"metadata": map[string]any{
							"name": "test-app",
						},
					},
				},
			},
			runID: "",
			expectedLabels: map[string]string{
				vars.ArgoCDApplicationLabelKey: "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := labelApplicationWithRunID(tt.app, tt.runID)

			require.NoError(t, err)

			actualLabels := tt.app.Yaml.GetLabels()
			assert.Equal(t, tt.expectedLabels, actualLabels)
		})
	}
}

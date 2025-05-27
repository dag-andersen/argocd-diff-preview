package extract

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestProcessYamlOutput(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedCount int
		expectedError bool
		expectedKinds []string
	}{
		{
			name: "single valid kubernetes manifest",
			input: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment
  namespace: default
spec:
  replicas: 1`,
			expectedCount: 1,
			expectedError: false,
			expectedKinds: []string{"Deployment"},
		},
		{
			name: "multiple valid kubernetes manifests",
			input: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment
  namespace: default
spec:
  replicas: 1
---
apiVersion: v1
kind: Service
metadata:
  name: test-service
  namespace: default
spec:
  ports:
  - port: 80`,
			expectedCount: 2,
			expectedError: false,
			expectedKinds: []string{"Deployment", "Service"},
		},
		{
			name:          "empty input",
			input:         "",
			expectedCount: 0,
			expectedError: false,
			expectedKinds: []string{},
		},
		{
			name:          "only whitespace",
			input:         "   \n\t  \r\n   \t\t  \n  ",
			expectedCount: 0,
			expectedError: false,
			expectedKinds: []string{},
		},
		{
			name:          "only separators",
			input:         "---\n---\n---",
			expectedCount: 0,
			expectedError: false,
			expectedKinds: []string{},
		},
		{
			name: "manifest without apiVersion",
			input: `kind: Deployment
metadata:
  name: test-deployment
spec:
  replicas: 1`,
			expectedCount: 0,
			expectedError: false,
			expectedKinds: []string{},
		},
		{
			name: "manifest without kind",
			input: `apiVersion: apps/v1
metadata:
  name: test-deployment
spec:
  replicas: 1`,
			expectedCount: 0,
			expectedError: false,
			expectedKinds: []string{},
		},
		{
			name: "manifest with empty apiVersion",
			input: `apiVersion: ""
kind: Deployment
metadata:
  name: test-deployment`,
			expectedCount: 0,
			expectedError: false,
			expectedKinds: []string{},
		},
		{
			name: "manifest with empty kind",
			input: `apiVersion: apps/v1
kind: ""
metadata:
  name: test-deployment`,
			expectedCount: 0,
			expectedError: false,
			expectedKinds: []string{},
		},
		{
			name: "invalid YAML syntax",
			input: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment
  invalid: [unclosed bracket`,
			expectedCount: 0,
			expectedError: true,
			expectedKinds: []string{},
		},
		{
			name:          "scalar string value",
			input:         `just a plain string`,
			expectedCount: 0,
			expectedError: true,
			expectedKinds: []string{},
		},
		{
			name:          "scalar number value",
			input:         `42`,
			expectedCount: 0,
			expectedError: true,
			expectedKinds: []string{},
		},
		{
			name:          "scalar boolean value",
			input:         `true`,
			expectedCount: 0,
			expectedError: true,
			expectedKinds: []string{},
		},
		{
			name: "mixed scalar and valid manifest",
			input: `just a string
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config`,
			expectedCount: 1,
			expectedError: true,
			expectedKinds: []string{},
		},
		{
			name: "scalar array value",
			input: `- item1
- item2
- item3`,
			expectedCount: 0,
			expectedError: true,
			expectedKinds: []string{},
		},
		{
			name: "mixed valid and invalid manifests",
			input: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment
---
kind: Service
metadata:
  name: test-service
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config`,
			expectedCount: 2,
			expectedError: false,
			expectedKinds: []string{"Deployment", "ConfigMap"},
		},
		{
			name: "manifest with extra whitespace and empty documents",
			input: `

---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment

---

---
apiVersion: v1
kind: Service
metadata:
  name: test-service
---

`,
			expectedCount: 2,
			expectedError: false,
			expectedKinds: []string{"Deployment", "Service"},
		},
		{
			name: "complex kubernetes manifest",
			input: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
  namespace: default
  labels:
    app: nginx
spec:
  replicas: 3
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:1.14.2
        ports:
        - containerPort: 80`,
			expectedCount: 1,
			expectedError: false,
			expectedKinds: []string{"Deployment"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := processYamlOutput(tt.input)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Len(t, result, tt.expectedCount)

			// Verify the kinds of the returned manifests
			for i, expectedKind := range tt.expectedKinds {
				if i < len(result) {
					kind, found, err := unstructured.NestedString(result[i].Object, "kind")
					require.NoError(t, err)
					require.True(t, found)
					assert.Equal(t, expectedKind, kind)
				}
			}

			// Verify all returned manifests have required fields
			for _, manifest := range result {
				apiVersion, found, err := unstructured.NestedString(manifest.Object, "apiVersion")
				require.NoError(t, err)
				require.True(t, found)
				assert.NotEmpty(t, apiVersion)

				kind, found, err := unstructured.NestedString(manifest.Object, "kind")
				require.NoError(t, err)
				require.True(t, found)
				assert.NotEmpty(t, kind)
			}
		})
	}
}

func TestProcessYamlOutput_EdgeCases(t *testing.T) {
	t.Run("very large manifest", func(t *testing.T) {
		// Create a manifest with many labels
		input := `apiVersion: v1
kind: ConfigMap
metadata:
  name: large-config
  labels:`

		// Add many labels to test performance
		for i := 0; i < 100; i++ {
			input += "\n    label" + fmt.Sprintf("%d", i) + ": value" + fmt.Sprintf("%d", i)
		}

		input += "\ndata:\n  key: value"

		result, err := processYamlOutput(input)
		require.NoError(t, err)
		assert.Len(t, result, 1)

		kind, found, err := unstructured.NestedString(result[0].Object, "kind")
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, "ConfigMap", kind)
	})

	t.Run("manifest with special characters", func(t *testing.T) {
		input := `apiVersion: v1
kind: Secret
metadata:
  name: special-chars-secret
  annotations:
    description: "This contains special chars: !@#$%^&*()_+-={}[]|\\:;\"'<>?,./"
data:
  key: dmFsdWU=`

		result, err := processYamlOutput(input)
		require.NoError(t, err)
		assert.Len(t, result, 1)

		kind, found, err := unstructured.NestedString(result[0].Object, "kind")
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, "Secret", kind)
	})

	t.Run("manifest with unicode characters", func(t *testing.T) {
		input := `apiVersion: v1
kind: ConfigMap
metadata:
  name: unicode-config
  annotations:
    description: "Unicode test: 你好世界 🌍 café naïve résumé"
data:
  greeting: "Hello 世界"`

		result, err := processYamlOutput(input)
		require.NoError(t, err)
		assert.Len(t, result, 1)

		kind, found, err := unstructured.NestedString(result[0].Object, "kind")
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, "ConfigMap", kind)
	})
}

func TestProcessYamlOutput_YamlStructure(t *testing.T) {
	t.Run("verify unstructured object structure", func(t *testing.T) {
		input := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment
  namespace: test-namespace
  labels:
    app: test
spec:
  replicas: 2
  selector:
    matchLabels:
      app: test`

		result, err := processYamlOutput(input)
		require.NoError(t, err)
		require.Len(t, result, 1)

		manifest := result[0]

		// Test nested field access
		name, found, err := unstructured.NestedString(manifest.Object, "metadata", "name")
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, "test-deployment", name)

		namespace, found, err := unstructured.NestedString(manifest.Object, "metadata", "namespace")
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, "test-namespace", namespace)

		replicas, found, err := unstructured.NestedFloat64(manifest.Object, "spec", "replicas")
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, float64(2), replicas)

		// Test nested map access
		labels, found, err := unstructured.NestedStringMap(manifest.Object, "metadata", "labels")
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, "test", labels["app"])
	})
}

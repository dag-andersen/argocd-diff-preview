package extract

import (
	"testing"

	"github.com/dag-andersen/argocd-diff-preview/pkg/argoapplication"
	"github.com/dag-andersen/argocd-diff-preview/pkg/git"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

// Helper function to create a test ArgoResource
func createTestArgoResource(id, name, fileName string, branch git.BranchType) *argoapplication.ArgoResource {
	yamlStr := `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: ` + name + `
spec:
  destination:
    namespace: default`

	var y unstructured.Unstructured
	if err := yaml.Unmarshal([]byte(yamlStr), &y); err != nil {
		panic("failed to unmarshal yaml in test")
	}

	return argoapplication.NewArgoResource(&y, argoapplication.Application, id, name, fileName, branch)
}

func TestAddApplicationPrefix(t *testing.T) {
	tests := []struct {
		name           string
		app            *argoapplication.ArgoResource
		prefix         string
		expectedId     string
		expectedPrefix string // The prefix part we expect in the result
		expectError    bool
	}{
		{
			name:           "short name with base branch",
			app:            createTestArgoResource("test-app", "test-app", "test.yaml", git.Base),
			prefix:         "pr123",
			expectedPrefix: "pr123-b-",
			expectError:    false,
		},
		{
			name:           "short name with target branch",
			app:            createTestArgoResource("test-app", "test-app", "test.yaml", git.Target),
			prefix:         "pr456",
			expectedPrefix: "pr456-t-",
			expectError:    false,
		},
		{
			name:        "empty branch should not modify name",
			app:         createTestArgoResource("test-app", "test-app", "test.yaml", ""),
			prefix:      "pr123",
			expectedId:  "test-app", // Should remain unchanged
			expectError: false,
		},
		{
			name: "long name requiring unique ID",
			app: createTestArgoResource(
				"very-long-application-name-that-exceeds-kubernetes-limit",
				"very-long-application-name-that-exceeds-kubernetes-limit",
				"test.yaml",
				git.Base,
			),
			prefix:         "pr123",
			expectedPrefix: "pr123-b-",
			expectError:    false,
		},
		{
			name: "name at exact limit boundary",
			app: createTestArgoResource(
				"app-name-exactly-at-the-boundary-point-test",
				"app-name-exactly-at-the-boundary-point-test",
				"test.yaml",
				git.Target,
			),
			prefix:         "pr123",
			expectedPrefix: "pr123-t-",
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalYamlName := tt.app.Yaml.GetName()

			err := addApplicationPrefix(tt.app, tt.prefix)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)

			// For empty branch case, name should remain unchanged
			if tt.app.Branch == "" {
				assert.Equal(t, tt.expectedId, tt.app.Id)
				assert.Equal(t, originalYamlName, tt.app.Yaml.GetName())
				return
			}

			// Check that the ID starts with expected prefix
			assert.True(t, len(tt.app.Id) > 0, "ID should not be empty")
			assert.Contains(t, tt.app.Id, tt.expectedPrefix, "ID should contain expected prefix")

			// Check that YAML name matches the struct ID
			assert.Equal(t, tt.app.Id, tt.app.Yaml.GetName(), "YAML name should match struct ID")

			// Check that the result doesn't exceed Kubernetes name length limit
			assert.LessOrEqual(t, len(tt.app.Id), 53, "ID should not exceed Kubernetes name length limit")
		})
	}
}

// Helper to create an ArgoResource with a plugin source
func createTestArgoResourceWithPlugin(name string, branch git.BranchType) *argoapplication.ArgoResource {
	yamlStr := `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: ` + name + `
spec:
  destination:
    namespace: default
  source:
    repoURL: https://github.com/example/repo
    path: chart/
    plugin:
      name: my-plugin`

	var y unstructured.Unstructured
	if err := yaml.Unmarshal([]byte(yamlStr), &y); err != nil {
		panic("failed to unmarshal yaml in test")
	}

	return argoapplication.NewArgoResource(&y, argoapplication.Application, name, name, "test.yaml", branch)
}

// Helper to create an ArgoResource with multi-source including a plugin
func createTestArgoResourceWithMultiSourcePlugin(name string, branch git.BranchType) *argoapplication.ArgoResource {
	yamlStr := `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: ` + name + `
spec:
  destination:
    namespace: default
  sources:
    - repoURL: https://github.com/example/repo
      path: manifests/
    - repoURL: https://github.com/example/repo
      path: chart/
      plugin:
        name: my-plugin`

	var y unstructured.Unstructured
	if err := yaml.Unmarshal([]byte(yamlStr), &y); err != nil {
		panic("failed to unmarshal yaml in test")
	}

	return argoapplication.NewArgoResource(&y, argoapplication.Application, name, name, "test.yaml", branch)
}

// Helper to create an ArgoResource with a plugin that already has env vars
func createTestArgoResourceWithPluginEnv(name string, branch git.BranchType) *argoapplication.ArgoResource {
	yamlStr := `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: ` + name + `
spec:
  destination:
    namespace: default
  source:
    repoURL: https://github.com/example/repo
    path: chart/
    plugin:
      name: my-plugin
      env:
        - name: MY_VAR
          value: my-value`

	var y unstructured.Unstructured
	if err := yaml.Unmarshal([]byte(yamlStr), &y); err != nil {
		panic("failed to unmarshal yaml in test")
	}

	return argoapplication.NewArgoResource(&y, argoapplication.Application, name, name, "test.yaml", branch)
}

func TestAddApplicationPrefix_PluginEnvOverride(t *testing.T) {
	t.Run("injects DIFF_NAME into single-source plugin", func(t *testing.T) {
		app := createTestArgoResourceWithPlugin("my-app", git.Target)
		err := addApplicationPrefix(app, "pr123")
		assert.NoError(t, err)

		assert.Equal(t, "pr123-t-my-app", app.Id)

		env, found, _ := unstructured.NestedSlice(app.Yaml.Object, "spec", "source", "plugin", "env")
		assert.True(t, found, "plugin.env should exist")
		assert.Len(t, env, 1)
		entry := env[0].(map[string]any)
		assert.Equal(t, "DIFF_NAME", entry["name"])
		assert.Equal(t, "my-app", entry["value"])
	})

	t.Run("does not inject for non-plugin sources", func(t *testing.T) {
		app := createTestArgoResource("my-app", "my-app", "test.yaml", git.Target)
		err := addApplicationPrefix(app, "pr123")
		assert.NoError(t, err)

		_, found, _ := unstructured.NestedSlice(app.Yaml.Object, "spec", "source", "plugin", "env")
		assert.False(t, found, "plugin.env should not exist for non-plugin sources")
	})

	t.Run("injects DIFF_NAME into multi-source plugin", func(t *testing.T) {
		app := createTestArgoResourceWithMultiSourcePlugin("my-app", git.Target)
		err := addApplicationPrefix(app, "pr123")
		assert.NoError(t, err)

		sources, found, _ := unstructured.NestedSlice(app.Yaml.Object, "spec", "sources")
		assert.True(t, found)
		assert.Len(t, sources, 2)

		// First source should NOT have plugin.env
		src0 := sources[0].(map[string]any)
		_, hasPlugin := src0["plugin"]
		assert.False(t, hasPlugin, "first source should not have a plugin")

		// Second source should have DIFF_NAME injected
		src1 := sources[1].(map[string]any)
		pluginMap := src1["plugin"].(map[string]any)
		envSlice := pluginMap["env"].([]any)
		assert.Len(t, envSlice, 1)
		entry := envSlice[0].(map[string]any)
		assert.Equal(t, "DIFF_NAME", entry["name"])
		assert.Equal(t, "my-app", entry["value"])
	})

	t.Run("preserves existing plugin env vars", func(t *testing.T) {
		app := createTestArgoResourceWithPluginEnv("my-app", git.Target)
		err := addApplicationPrefix(app, "pr123")
		assert.NoError(t, err)

		env, found, _ := unstructured.NestedSlice(app.Yaml.Object, "spec", "source", "plugin", "env")
		assert.True(t, found)
		assert.Len(t, env, 2, "should have both original and injected env vars")

		names := make(map[string]string)
		for _, e := range env {
			entry := e.(map[string]any)
			names[entry["name"].(string)] = entry["value"].(string)
		}
		assert.Equal(t, "my-value", names["MY_VAR"])
		assert.Equal(t, "my-app", names["DIFF_NAME"])
	})

	t.Run("does not inject for empty branch", func(t *testing.T) {
		app := createTestArgoResourceWithPlugin("my-app", "")
		err := addApplicationPrefix(app, "pr123")
		assert.NoError(t, err)

		assert.Equal(t, "my-app", app.Id)

		_, found, _ := unstructured.NestedSlice(app.Yaml.Object, "spec", "source", "plugin", "env")
		assert.False(t, found, "plugin.env should not be added when branch is empty")
	})
}

func TestBranchShortNames(t *testing.T) {
	tests := []struct {
		name              string
		branch            git.BranchType
		expectedShortName string
	}{
		{
			name:              "base branch should use 'b'",
			branch:            git.Base,
			expectedShortName: "b",
		},
		{
			name:              "target branch should use 't'",
			branch:            git.Target,
			expectedShortName: "t",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := createTestArgoResource("test-app", "test-app", "test.yaml", tt.branch)
			prefix := "pr123"

			err := addApplicationPrefix(app, prefix)
			assert.NoError(t, err)

			expectedPrefix := prefix + "-" + tt.expectedShortName + "-"
			assert.Contains(t, app.Id, expectedPrefix, "ID should contain expected branch short name")
		})
	}
}

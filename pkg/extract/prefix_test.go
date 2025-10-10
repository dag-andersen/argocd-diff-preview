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

func TestRemoveApplicationPrefix(t *testing.T) {
	tests := []struct {
		name        string
		app         *argoapplication.ArgoResource
		prefix      string
		expectedId  string
		expectError bool
	}{
		{
			name: "remove prefix from base branch",
			app: func() *argoapplication.ArgoResource {
				app := createTestArgoResource("pr123-b-test-app", "pr123-b-test-app", "test.yaml", git.Base)
				return app
			}(),
			prefix:      "pr123",
			expectedId:  "test-app",
			expectError: false,
		},
		{
			name: "remove prefix from target branch",
			app: func() *argoapplication.ArgoResource {
				app := createTestArgoResource("pr456-t-my-service", "pr456-t-my-service", "service.yaml", git.Target)
				return app
			}(),
			prefix:      "pr456",
			expectedId:  "my-service",
			expectError: false,
		},
		{
			name: "remove prefix with unique ID",
			app: func() *argoapplication.ArgoResource {
				app := createTestArgoResource("pr789-b-uid-12345", "pr789-b-uid-12345", "app.yaml", git.Base)
				return app
			}(),
			prefix:      "pr789",
			expectedId:  "uid-12345",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := setName(tt.app, tt.prefix)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedId, tt.app.Id, "ID should match expected after removing prefix")
			assert.Equal(t, tt.app.Id, tt.app.Yaml.GetName(), "YAML name should match struct ID")
		})
	}
}

func TestAddAndRemoveApplicationPrefix_RoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		app    *argoapplication.ArgoResource
		prefix string
	}{
		{
			name:   "round trip with base branch",
			app:    createTestArgoResource("original-app", "original-app", "app.yaml", git.Base),
			prefix: "pr123",
		},
		{
			name:   "round trip with target branch",
			app:    createTestArgoResource("another-app", "another-app", "app.yaml", git.Target),
			prefix: "pr456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalId := tt.app.Id
			originalYamlName := tt.app.Yaml.GetName()

			// Add prefix
			err := addApplicationPrefix(tt.app, tt.prefix)
			assert.NoError(t, err)

			// Verify it was changed
			assert.NotEqual(t, originalId, tt.app.Id, "ID should be different after adding prefix")
			assert.Equal(t, tt.app.Id, tt.app.Yaml.GetName(), "YAML name should match struct ID")

			// Remove prefix
			_, err = setName(tt.app, tt.prefix)
			assert.NoError(t, err)

			// Verify it's back to original (for short names)
			if len(originalId) <= (53 - len(tt.prefix) - 2 - 2) {
				assert.Equal(t, originalId, tt.app.Id, "ID should be back to original after round trip")
				assert.Equal(t, originalYamlName, tt.app.Yaml.GetName(), "YAML name should be back to original")
			}
		})
	}
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

package argoapplicaiton

import (
	"testing"

	"github.com/dag-andersen/argocd-diff-preview/pkg/types"
	yamlutil "github.com/dag-andersen/argocd-diff-preview/pkg/yaml"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestUniqueNames(t *testing.T) {
	tests := []struct {
		name    string
		apps    []ArgoResource
		branch  *types.Branch
		want    []ArgoResource
		wantErr bool
	}{
		{
			name: "no duplicates",
			apps: []ArgoResource{
				createTestApp("app1", "app1.yaml"),
				createTestApp("app2", "app2.yaml"),
				createTestApp("app3", "app3.yaml"),
			},
			branch: &types.Branch{Name: "test-branch"},
			want: []ArgoResource{
				createTestApp("app1", "app1.yaml"),
				createTestApp("app2", "app2.yaml"),
				createTestApp("app3", "app3.yaml"),
			},
			wantErr: false,
		},
		{
			name: "with duplicates",
			apps: []ArgoResource{
				createTestApp("app1", "app1.yaml"),
				createTestApp("app1", "app1-2.yaml"),
				createTestApp("app2", "app2.yaml"),
			},
			branch: &types.Branch{Name: "test-branch"},
			want: []ArgoResource{
				createTestApp("app1-1", "app1.yaml"),
				createTestApp("app1-2", "app1-2.yaml"),
				createTestApp("app2", "app2.yaml"),
			},
			wantErr: false,
		},
		{
			name: "multiple duplicates",
			apps: []ArgoResource{
				createTestApp("app1", "app1.yaml"),
				createTestApp("app1", "app1-2.yaml"),
				createTestApp("app1", "app1-3.yaml"),
				createTestApp("app2", "app2.yaml"),
				createTestApp("app2", "app2-2.yaml"),
			},
			branch: &types.Branch{Name: "test-branch"},
			want: []ArgoResource{
				createTestApp("app1-1", "app1.yaml"),
				createTestApp("app1-2", "app1-2.yaml"),
				createTestApp("app1-3", "app1-3.yaml"),
				createTestApp("app2-1", "app2.yaml"),
				createTestApp("app2-2", "app2-2.yaml"),
			},
			wantErr: false,
		},
		{
			name:    "empty slice",
			apps:    []ArgoResource{},
			branch:  &types.Branch{Name: "test-branch"},
			want:    []ArgoResource{},
			wantErr: false,
		},
		{
			name: "single app",
			apps: []ArgoResource{
				createTestApp("app1", "app1.yaml"),
			},
			branch: &types.Branch{Name: "test-branch"},
			want: []ArgoResource{
				createTestApp("app1", "app1.yaml"),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Run UniqueNames
			got := UniqueNames(tt.apps, tt.branch)

			// Check length
			assert.Equal(t, len(tt.want), len(got), "Expected %d apps, got %d", len(tt.want), len(got))

			// Check each app
			for i := range got {
				assert.Equal(t, tt.want[i].Name, got[i].Name, "App %d: Expected name %s, got %s", i, tt.want[i].Name, got[i].Name)

				// Check YAML name matches
				gotName := yamlutil.GetYamlValue(got[i].Yaml, []string{"metadata", "name"}).Value
				assert.Equal(t, got[i].Name, gotName, "App %d: YAML name %s doesn't match struct name %s", i, gotName, got[i].Name)
			}

			// Check uniqueness
			names := make(map[string]bool)
			for _, app := range got {
				assert.False(t, names[app.Name], "Duplicate name found: %s", app.Name)
				names[app.Name] = true
			}
		})
	}
}

// Helper function to create a test ArgoResource with basic YAML structure
func createTestApp(name, fileName string) ArgoResource {
	yamlStr := `apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: ` + name + `
spec:
  destination:
    namespace: default`

	var node yaml.Node
	yaml.Unmarshal([]byte(yamlStr), &node)

	return ArgoResource{
		Yaml:     &node,
		Kind:     Application,
		Name:     name,
		FileName: fileName,
	}
}

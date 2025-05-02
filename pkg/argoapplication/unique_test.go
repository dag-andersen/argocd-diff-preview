package argoapplication

import (
	"fmt"
	"testing"

	"github.com/dag-andersen/argocd-diff-preview/pkg/git"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

func TestUniqueNames(t *testing.T) {
	tests := []struct {
		name    string
		apps    []ArgoResource
		branch  *git.Branch
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
			branch: &git.Branch{Name: "test-branch"},
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
			branch: &git.Branch{Name: "test-branch"},
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
			branch: &git.Branch{Name: "test-branch"},
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
			branch:  &git.Branch{Name: "test-branch"},
			want:    []ArgoResource{},
			wantErr: false,
		},
		{
			name: "single app",
			apps: []ArgoResource{
				createTestApp("app1", "app1.yaml"),
			},
			branch: &git.Branch{Name: "test-branch"},
			want: []ArgoResource{
				createTestApp("app1", "app1.yaml"),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Run UniqueNames
			got := UniqueIds(tt.apps, tt.branch)

			// Check length
			assert.Equal(t, len(tt.want), len(got), "Expected %d apps, got %d", len(tt.want), len(got))

			// Check each app
			for i := range got {
				assert.Equal(t, tt.want[i].Id, got[i].Id, "App %d: Expected name %s, got %s", i, tt.want[i].Id, got[i].Id)

				// Check YAML name matches
				gotName := got[i].Yaml.GetName()
				assert.Equal(t, got[i].Id, gotName, "App %d: YAML name %s doesn't match struct name %s", i, gotName, got[i].Id)
			}

			// Check uniqueness
			names := make(map[string]bool)
			for _, app := range got {
				assert.False(t, names[app.Id], "Duplicate name found: %s", app.Id)
				names[app.Id] = true
			}
		})
	}
}

// Helper function to create a test ArgoResource with basic YAML structure
func createTestApp(name, fileName string) ArgoResource {
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
		panic(fmt.Sprintf("failed to unmarshal yaml in test: %v", err))
	}

	return ArgoResource{
		Yaml:     &y,
		Kind:     Application,
		Id:       name,
		FileName: fileName,
	}
}

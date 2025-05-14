package fileparsing

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dag-andersen/argocd-diff-preview/pkg/git"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

func TestParser(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "test")
	assert.NoError(t, err)
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Failed to remove temporary directory: %v", err)
		}
	}()

	// Create test YAML files
	testFiles := []struct {
		name    string
		content string
	}{
		{
			name: "test1.yaml",
			content: `apiVersion: v1
kind: Pod
metadata:
  name: test-pod`,
		},
		{
			name: "test2.yaml",
			content: `apiVersion: v1
kind: Service
metadata:
  name: test-service`,
		},
	}

	for _, file := range testFiles {
		err := os.WriteFile(filepath.Join(tempDir, file.name), []byte(file.content), 0644)
		assert.NoError(t, err)
	}

	t.Run("GetYamlFiles", func(t *testing.T) {
		files := GetYamlFiles(tempDir, nil)
		assert.Len(t, files, 2)
		assert.Contains(t, files, "test1.yaml")
		assert.Contains(t, files, "test2.yaml")
	})

	t.Run("ParseYaml", func(t *testing.T) {
		files := []string{"test1.yaml", "test2.yaml"}
		resources := ParseYaml(tempDir, files, git.BranchType(""))
		assert.Len(t, resources, 2)
	})

	t.Run("WithFileRegex", func(t *testing.T) {
		regex := "test1.yaml"
		files := GetYamlFiles(tempDir, &regex)
		assert.Len(t, files, 1)
		assert.Contains(t, files, "test1.yaml")
	})
}

func TestProcessYamlChunk(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		chunk    string
		want     []Resource
		wantLog  string
	}{
		{
			name:     "valid application yaml",
			filename: "test.yaml",
			chunk: `apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
spec:
  destination:
    namespace: default
    server: https://kubernetes.default.svc`,
			want: []Resource{
				{
					FileName: "test.yaml",
					Yaml: func() unstructured.Unstructured {
						var y unstructured.Unstructured
						err := yaml.Unmarshal([]byte(`apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
spec:
  destination:
    namespace: default
    server: https://kubernetes.default.svc`), &y)
						assert.NoError(t, err)
						return y
					}(),
				},
			},
		},
		{
			name:     "invalid yaml",
			filename: "invalid.yaml",
			chunk:    "invalid: :",
			want:     nil,
			wantLog:  "⚠️ Failed to parse YAML in file 'invalid.yaml'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var resources []Resource
			processYamlChunk(tt.filename, tt.chunk, &resources, git.BranchType(""))

			if tt.want == nil {
				assert.Empty(t, resources)
			} else {
				assert.Equal(t, tt.want[0].FileName, resources[0].FileName)
				assert.Equal(t, tt.want[0].Yaml, resources[0].Yaml)
			}
		})
	}
}

package k8s

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestParser(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

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
		resources := ParseYaml(tempDir, files)
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
					Yaml: func() yaml.Node {
						var node yaml.Node
						yaml.Unmarshal([]byte(`apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
spec:
  destination:
    namespace: default
    server: https://kubernetes.default.svc`), &node)
						return node
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
			processYamlChunk(tt.filename, tt.chunk, &resources)

			if tt.want == nil {
				assert.Empty(t, resources)
			} else {
				assert.Equal(t, tt.want[0].FileName, resources[0].FileName)
				assert.Equal(t, tt.want[0].Yaml, resources[0].Yaml)
			}
		})
	}
}

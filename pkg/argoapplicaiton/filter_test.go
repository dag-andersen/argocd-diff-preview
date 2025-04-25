package argoapplicaiton

import (
	"log"
	"testing"

	"github.com/dag-andersen/argocd-diff-preview/pkg/selector"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

func TestFilterBySelectors(t *testing.T) {

	zerolog.SetGlobalLevel(zerolog.FatalLevel)

	tests := []struct {
		name      string
		yaml      string
		selectors []selector.Selector
		want      bool
	}{
		{
			name: "no selectors",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  labels:
    app: test`,
			selectors: []selector.Selector{},
			want:      true,
		},
		{
			name: "matching label selector",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  labels:
    app: test`,
			selectors: []selector.Selector{
				{Key: "app", Value: "test", Operator: selector.Eq},
			},
			want: true,
		},
		{
			name: "non-matching label selector",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  labels:
    app: test`,
			selectors: []selector.Selector{
				{Key: "app", Value: "other", Operator: selector.Eq},
			},
			want: false,
		},
		{
			name: "missing label",
			yaml: `apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  labels:
    other: value`,
			selectors: []selector.Selector{
				{Key: "app", Value: "test", Operator: selector.Eq},
			},
			want: false,
		},
		{
			name: "no labels",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app`,
			selectors: []selector.Selector{
				{Key: "app", Value: "test", Operator: selector.Eq},
			},
			want: false,
		},
		{
			name: "no metadata",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
spec:
  destination:
    namespace: default`,
			selectors: []selector.Selector{
				{Key: "app", Value: "test", Operator: selector.Eq},
			},
			want: false,
		},
		{
			name: "not equals operator",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  labels:
    app: test`,
			selectors: []selector.Selector{
				{Key: "app", Value: "other", Operator: selector.Ne},
			},
			want: true,
		},
		{
			name: "not equals operator with matching value",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  labels:
    app: test`,
			selectors: []selector.Selector{
				{Key: "app", Value: "test", Operator: selector.Ne},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse YAML
			var node unstructured.Unstructured
			err := yaml.Unmarshal([]byte(tt.yaml), &node)
			assert.NoError(t, err)

			// Create ArgoResource
			app := &ArgoResource{
				Yaml:     &node,
				Kind:     Application,
				Id:       "test-app",
				FileName: "test.yaml",
			}

			// Run filter
			got := app.filterBySelectors(tt.selectors)

			// Check result
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFilterByFilesChanged(t *testing.T) {

	zerolog.SetGlobalLevel(zerolog.FatalLevel)

	tests := []struct {
		name                      string
		yaml                      string
		filesChanged              []string
		ignoreInvalidWatchPattern bool
		want                      bool
	}{
		{
			name: "no files changed",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  annotations:
    argocd-diff-preview/watch-pattern: ".*\\.yaml$"`,
			filesChanged:              []string{},
			ignoreInvalidWatchPattern: false,
			want:                      false,
		},
		{
			name: "matching file watch pattern",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  annotations:
    argocd-diff-preview/watch-pattern: ".*\\.yaml$"`,
			filesChanged:              []string{"test.yaml"},
			ignoreInvalidWatchPattern: false,
			want:                      true,
		},
		{
			name: "non-matching file watch pattern",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  annotations:
    argocd-diff-preview/watch-pattern: ".*\\.yaml$"`,
			filesChanged:              []string{"test.txt"},
			ignoreInvalidWatchPattern: false,
			want:                      false,
		},
		{
			name: "multiple watch patterns",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  annotations:
    argocd-diff-preview/watch-pattern: ".*\\.yaml$,.*\\.txt$"`,
			filesChanged:              []string{"test.txt"},
			ignoreInvalidWatchPattern: false,
			want:                      true,
		},
		{
			name: "invalid watch pattern with ignore",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  annotations:
    argocd-diff-preview/watch-pattern: "[invalid, .*\\.yaml$"`,
			filesChanged:              []string{"some-file.yaml"},
			ignoreInvalidWatchPattern: true,
			want:                      true,
		},
		{
			name: "invalid watch pattern without ignore",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  annotations:
    argocd-diff-preview/watch-pattern: "[invalid, .*\\.yaml$"`,
			filesChanged:              []string{"some-file.yaml"},
			ignoreInvalidWatchPattern: false,
			want:                      false,
		},
		{
			name: "empty watch pattern",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  annotations:
    argocd-diff-preview/watch-pattern: ""`,
			filesChanged:              []string{"some-file.yaml"},
			ignoreInvalidWatchPattern: false,
			want:                      false,
		},
		{
			name: "no watch pattern annotation",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app`,
			filesChanged:              []string{"some-file.yaml"},
			ignoreInvalidWatchPattern: false,
			want:                      false,
		},
		{
			name: "no metadata",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
spec:
  destination:
    namespace: default`,
			filesChanged:              []string{"some-file.yaml"},
			ignoreInvalidWatchPattern: false,
			want:                      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse YAML
			var node unstructured.Unstructured
			err := yaml.Unmarshal([]byte(tt.yaml), &node)
			assert.NoError(t, err)

			// Create ArgoResource
			app := &ArgoResource{
				Yaml:     &node,
				Kind:     Application,
				Name:     "test-app",
				FileName: "test.yaml",
			}

			// Run filter
			got := app.filterByFilesChanged(tt.filesChanged, tt.ignoreInvalidWatchPattern)

			// Check result
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFilter(t *testing.T) {

	zerolog.SetGlobalLevel(zerolog.FatalLevel)

	tests := []struct {
		name                      string
		yaml                      string
		selectors                 []selector.Selector
		filesChanged              []string
		ignoreInvalidWatchPattern bool
		want                      *ArgoResource
	}{
		{
			name: "no selectors or files changed",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  labels:
    app: test
  annotations:
    argocd-diff-preview/watch-pattern: ".*\\.yaml$"`,
			selectors:                 []selector.Selector{},
			filesChanged:              []string{},
			ignoreInvalidWatchPattern: false,
			want:                      &ArgoResource{Id: "test-app"},
		},
		{
			name: "matching label selector",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  labels:
    app: test
  annotations:
    argocd-diff-preview/watch-pattern: ".*\\.yaml$"`,
			selectors: []selector.Selector{
				{Key: "app", Value: "test", Operator: selector.Eq},
			},
			filesChanged:              []string{},
			ignoreInvalidWatchPattern: false,
			want:                      &ArgoResource{Id: "test-app"},
		},
		{
			name: "non-matching label selector",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  labels:
    app: test
  annotations:
    argocd-diff-preview/watch-pattern: ".*\\.yaml$"`,
			selectors: []selector.Selector{
				{Key: "app", Value: "other", Operator: selector.Eq},
			},
			filesChanged:              []string{},
			ignoreInvalidWatchPattern: false,
			want:                      nil,
		},
		{
			name: "matching file watch pattern",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  labels:
    app: test
  annotations:
    argocd-diff-preview/watch-pattern: ".*\\.yaml$"`,
			selectors:                 []selector.Selector{},
			filesChanged:              []string{"test.yaml"},
			ignoreInvalidWatchPattern: false,
			want:                      &ArgoResource{Id: "test-app"},
		},
		{
			name: "non-matching file watch pattern",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  labels:
    app: test
  annotations:
    argocd-diff-preview/watch-pattern: ".*\\.yaml$"`,
			selectors:                 []selector.Selector{},
			filesChanged:              []string{"test.txt"},
			ignoreInvalidWatchPattern: false,
			want:                      nil,
		},
		{
			name: "multiple watch patterns",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  labels:
    app: test
  annotations:
    argocd-diff-preview/watch-pattern: ".*\\.yaml$,.*\\.txt$"`,
			selectors:                 []selector.Selector{},
			filesChanged:              []string{"test.txt"},
			ignoreInvalidWatchPattern: false,
			want:                      &ArgoResource{Id: "test-app"},
		},
		{
			name: "invalid watch pattern with ignore",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  labels:
    app: test
  annotations:
    argocd-diff-preview/watch-pattern: "[invalid"`,
			selectors:                 []selector.Selector{},
			filesChanged:              []string{"test.yaml"},
			ignoreInvalidWatchPattern: true,
			want:                      &ArgoResource{Id: "test-app"},
		},
		{
			name: "invalid watch pattern without ignore",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  labels:
    app: test
  annotations:
    argocd-diff-preview/watch-pattern: "[invalid"`,
			selectors:                 []selector.Selector{},
			filesChanged:              []string{"some-file.yaml"},
			ignoreInvalidWatchPattern: false,
			want:                      nil,
		},
		{
			name: "empty watch pattern",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  labels:
    app: test
  annotations:
    argocd-diff-preview/watch-pattern: ""`,
			selectors:                 []selector.Selector{},
			filesChanged:              []string{"some-file.yaml"},
			ignoreInvalidWatchPattern: false,
			want:                      nil,
		},
		{
			name: "no watch pattern annotation",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  labels:
    app: test`,
			selectors:                 []selector.Selector{},
			filesChanged:              []string{"some-file.yaml"},
			ignoreInvalidWatchPattern: false,
			want:                      nil,
		},
		{
			name: "no metadata",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
spec:
  destination:
    namespace: default`,
			selectors:                 []selector.Selector{},
			filesChanged:              []string{"some-file.yaml"},
			ignoreInvalidWatchPattern: false,
			want:                      nil,
		},
		{
			name: "not equals operator",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  labels:
    app: test
  annotations:
    argocd-diff-preview/watch-pattern: ".*\\.yaml$"`,
			selectors: []selector.Selector{
				{Key: "app", Value: "other", Operator: selector.Ne},
			},
			filesChanged:              []string{},
			ignoreInvalidWatchPattern: false,
			want:                      &ArgoResource{Id: "test-app"},
		},
		{
			name: "not equals operator with matching value",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  labels:
    app: test
  annotations:
    argocd-diff-preview/watch-pattern: ".*\\.yaml$"`,
			selectors: []selector.Selector{
				{Key: "app", Value: "test", Operator: selector.Ne},
			},
			filesChanged:              []string{},
			ignoreInvalidWatchPattern: false,
			want:                      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse YAML
			var node unstructured.Unstructured
			err := yaml.Unmarshal([]byte(tt.yaml), &node)
			assert.NoError(t, err)

			// Create ArgoResource
			app := &ArgoResource{
				Yaml:     &node,
				Kind:     Application,
				Id:       "test-app",
				FileName: "test.yaml",
			}

			// Run filter
			got := app.Filter(tt.selectors, tt.filesChanged, tt.ignoreInvalidWatchPattern)

			// Check result
			if tt.want == nil {
				assert.Nil(t, got)
			} else {
				assert.NotNil(t, got)
				assert.Equal(t, tt.want.Id, got.Id)
			}
		})
	}
}

func getYamlApp(annotation string, sourcePath string) *unstructured.Unstructured {
	var node unstructured.Unstructured
	yamlText := `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    argocd.argoproj.io/manifest-generate-paths: "` + annotation + `"
spec:
  source:
    path: "` + sourcePath + `"`
	if err := yaml.Unmarshal([]byte(yamlText), &node); err != nil {
		log.Fatalf("Error unmarshalling YAML: %v", err)
	}
	return &node
}

func getMultiSourceYamlApp(annotation string, paths ...string) *unstructured.Unstructured {
	var node unstructured.Unstructured
	yamlText := `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    argocd.argoproj.io/manifest-generate-paths: "` + annotation + `"
spec:
  sources:`
	for _, path := range paths {
		yamlText += `
    - path: "` + path + `"`
	}
	if err := yaml.Unmarshal([]byte(yamlText), &node); err != nil {
		log.Fatalf("Error unmarshalling YAML: %v", err)
	}
	return &node
}

func TestFilterByAnnotationWatchPattern(t *testing.T) {

	zerolog.SetGlobalLevel(zerolog.FatalLevel)

	tests := []struct {
		name           string
		app            *unstructured.Unstructured
		files          []string
		changeExpected bool
	}{
		{"default no path", &unstructured.Unstructured{}, []string{"README.md"}, false},
		{"no files changed", getYamlApp(".", "source/path"), []string{}, false},
		{"relative path - matching", getYamlApp(".", "source/path"), []string{"source/path/my-deployment.yaml"}, true},
		{"relative path, multi source - matching #1", getMultiSourceYamlApp(".", "source/path", "other/path"), []string{"source/path/my-deployment.yaml"}, true},
		{"relative path, multi source - matching #2", getMultiSourceYamlApp(".", "other/path", "source/path"), []string{"source/path/my-deployment.yaml"}, true},
		{"relative path - not matching", getYamlApp(".", "source/path"), []string{"README.md"}, false},
		{"relative path, multi source - not matching", getMultiSourceYamlApp(".", "other/path", "unrelated/path"), []string{"README.md"}, false},
		{"absolute path - matching", getYamlApp("/source/path", "source/path"), []string{"source/path/my-deployment.yaml"}, true},
		{"absolute path, multi source - matching #1", getMultiSourceYamlApp("/source/path", "source/path", "other/path"), []string{"source/path/my-deployment.yaml"}, true},
		{"absolute path, multi source - matching #2", getMultiSourceYamlApp("/source/path", "other/path", "source/path"), []string{"source/path/my-deployment.yaml"}, true},
		{"absolute path - not matching", getYamlApp("/source/path1", "source/path"), []string{"source/path/my-deployment.yaml"}, false},
		{"absolute path, multi source - not matching", getMultiSourceYamlApp("/source/path1", "other/path", "source/path"), []string{"source/path/my-deployment.yaml"}, false},
		{"glob path * - matching", getYamlApp("/source/**/my-deployment.yaml", "source/path"), []string{"source/path/my-deployment.yaml"}, true},
		{"glob path * - not matching", getYamlApp("/source/**/my-service.yaml", "source/path"), []string{"source/path/my-deployment.yaml"}, false},
		{"glob path ? - matching", getYamlApp("/source/path/my-deployment-?.yaml", "source/path"), []string{"source/path/my-deployment-0.yaml"}, true},
		{"glob path ? - not matching", getYamlApp("/source/path/my-deployment-?.yaml", "source/path"), []string{"source/path/my-deployment.yaml"}, false},
		{"glob path char range - matching", getYamlApp("/source/path[0-9]/my-deployment.yaml", "source/path"), []string{"source/path1/my-deployment.yaml"}, true},
		{"glob path char range - not matching", getYamlApp("/source/path[0-9]/my-deployment.yaml", "source/path"), []string{"source/path/my-deployment.yaml"}, false},
		{"mixed glob path - matching", getYamlApp("/source/path[0-9]/my-*.yaml", "source/path"), []string{"source/path1/my-deployment.yaml"}, true},
		{"mixed glob path - not matching", getYamlApp("/source/path[0-9]/my-*.yaml", "source/path"), []string{"README.md"}, false},
		{"two relative paths - matching", getYamlApp(".;../shared", "my-app"), []string{"shared/my-deployment.yaml"}, true},
		{"two relative paths, multi source - matching #1", getMultiSourceYamlApp(".;../shared", "my-app", "other/path"), []string{"shared/my-deployment.yaml"}, true},
		{"two relative paths, multi source - matching #2", getMultiSourceYamlApp(".;../shared", "my-app", "other/path"), []string{"shared/my-deployment.yaml"}, true},
		{"two relative paths - not matching", getYamlApp(".;../shared", "my-app"), []string{"README.md"}, false},
		{"two relative paths, multi source - not matching", getMultiSourceYamlApp(".;../shared", "my-app", "other/path"), []string{"README.md"}, false},
		{"file relative path - matching", getYamlApp("./my-deployment.yaml", "source/path"), []string{"source/path/my-deployment.yaml"}, true},
		{"file relative path, multi source - matching #1", getMultiSourceYamlApp("./my-deployment.yaml", "source/path", "other/path"), []string{"source/path/my-deployment.yaml"}, true},
		{"file relative path, multi source - matching #2", getMultiSourceYamlApp("./my-deployment.yaml", "other/path", "source/path"), []string{"source/path/my-deployment.yaml"}, true},
		{"file relative path - not matching", getYamlApp("./my-deployment.yaml", "source/path"), []string{"README.md"}, false},
		{"file relative path, multi source - not matching", getMultiSourceYamlApp("./my-deployment.yaml", "source/path", "other/path"), []string{"README.md"}, false},
		{"file absolute path - matching", getYamlApp("/source/path/my-deployment.yaml", "source/path"), []string{"source/path/my-deployment.yaml"}, true},
		{"file absolute path, multi source - matching #1", getMultiSourceYamlApp("/source/path/my-deployment.yaml", "source/path", "other/path"), []string{"source/path/my-deployment.yaml"}, true},
		{"file absolute path, multi source - matching #2", getMultiSourceYamlApp("/source/path/my-deployment.yaml", "other/path", "source/path"), []string{"source/path/my-deployment.yaml"}, true},
		{"file absolute path - not matching", getYamlApp("/source/path1/README.md", "source/path"), []string{"source/path/my-deployment.yaml"}, false},
		{"file absolute path, multi source - not matching", getMultiSourceYamlApp("/source/path1/README.md", "source/path", "other/path"), []string{"source/path/my-deployment.yaml"}, false},
		{"file two relative paths - matching", getYamlApp("./README.md;../shared/my-deployment.yaml", "my-app"), []string{"shared/my-deployment.yaml"}, true},
		{"file two relative paths, multi source - matching", getMultiSourceYamlApp("./README.md;../shared/my-deployment.yaml", "my-app", "other-path"), []string{"shared/my-deployment.yaml"}, true},
		{"file two relative paths - not matching", getYamlApp(".README.md;../shared/my-deployment.yaml", "my-app"), []string{"kustomization.yaml"}, false},
		{"file two relative paths, multi source - not matching", getMultiSourceYamlApp(".README.md;../shared/my-deployment.yaml", "my-app", "other-path"), []string{"kustomization.yaml"}, false},
		{"changed file absolute path - matching", getYamlApp(".", "source/path"), []string{"/source/path/my-deployment.yaml"}, true},
	}
	for _, tt := range tests {
		ttc := tt
		t.Run(ttc.name, func(t *testing.T) {
			t.Parallel()
			app := &ArgoResource{
				Yaml:     ttc.app,
				Kind:     Application,
				Name:     "test-app",
				FileName: "test.yaml",
			}

			annotations, _, err := unstructured.NestedStringMap(ttc.app.Object, "metadata", "annotations")
			assert.NoError(t, err)
			assert.Equal(t, ttc.changeExpected, app.filterByManifestGeneratePaths(annotations, ttc.files))
		})
	}
}

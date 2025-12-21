package argoapplication

import (
	"testing"

	"github.com/dag-andersen/argocd-diff-preview/pkg/git"
	"github.com/dag-andersen/argocd-diff-preview/pkg/selector"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

func TestFilterByIgnoreAnnotation(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]any
		want        bool
	}{
		{
			name:        "no annotations",
			annotations: map[string]any{},
			want:        true,
		},
		{
			name: "ignore annotation not set",
			annotations: map[string]any{
				"other-annotation": "value",
			},
			want: true,
		},
		{
			name: "ignore annotation set to true",
			annotations: map[string]any{
				"argocd-diff-preview/ignore": "true",
			},
			want: false,
		},
		{
			name: "ignore annotation set to false",
			annotations: map[string]any{
				"argocd-diff-preview/ignore": "false",
			},
			want: true,
		},
		{
			name: "ignore annotation set to non-boolean value",
			annotations: map[string]any{
				"argocd-diff-preview/ignore": "some-value",
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resource := &ArgoResource{
				Yaml: &unstructured.Unstructured{
					Object: map[string]any{
						"metadata": map[string]any{
							"annotations": tt.annotations,
						},
					},
				},
			}

			got, _ := resource.filterByIgnoreAnnotation()
			if got != tt.want {
				t.Errorf("filterByIgnoreAnnotation() = %v, want %v", got, tt.want)
			}
		})
	}
}

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
			got, _ := app.filterBySelectors(tt.selectors)

			// Check result
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFilterByFilesChanged(t *testing.T) {

	zerolog.SetGlobalLevel(zerolog.FatalLevel)

	tests := []struct {
		name                       string
		yaml                       string
		filesChanged               []string
		ignoreInvalidWatchPattern  bool
		watchIfNoWatchPatternFound bool
		want                       bool
	}{
		{
			name: "no files changed",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  annotations:
    argocd-diff-preview/watch-pattern: '.*\.yaml$'`,
			filesChanged:               []string{},
			ignoreInvalidWatchPattern:  false,
			watchIfNoWatchPatternFound: false,
			want:                       false,
		},
		{
			name: "matching file watch pattern",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  annotations:
    argocd-diff-preview/watch-pattern: '.*\.yaml$'`,
			filesChanged:               []string{"test.yaml"},
			ignoreInvalidWatchPattern:  false,
			watchIfNoWatchPatternFound: false,
			want:                       true,
		},
		{
			name: "non-matching file watch pattern",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  annotations:
    argocd-diff-preview/watch-pattern: '.*\.yaml$'`,
			filesChanged:               []string{"test.txt"},
			ignoreInvalidWatchPattern:  false,
			watchIfNoWatchPatternFound: false,
			want:                       false,
		},
		{
			name: "multiple watch patterns",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  annotations:
    argocd-diff-preview/watch-pattern: '.*\.yaml$,.*\.txt$'`,
			filesChanged:               []string{"test.txt"},
			ignoreInvalidWatchPattern:  false,
			watchIfNoWatchPatternFound: false,
			want:                       true,
		},
		{
			name: "invalid watch pattern with ignore",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  annotations:
    argocd-diff-preview/watch-pattern: '[invalid, .*\.yaml$'`,
			filesChanged:               []string{"some-file.yaml"},
			ignoreInvalidWatchPattern:  true,
			watchIfNoWatchPatternFound: false,
			want:                       true,
		},
		{
			name: "invalid watch pattern without ignore",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  annotations:
    argocd-diff-preview/watch-pattern: '[invalid, .*\.yaml$'`,
			filesChanged:               []string{"some-file.yaml"},
			ignoreInvalidWatchPattern:  false,
			watchIfNoWatchPatternFound: false,
			want:                       false,
		},
		{
			name: "empty watch pattern",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  annotations:
    argocd-diff-preview/watch-pattern: ''`,
			filesChanged:               []string{"some-file.yaml"},
			ignoreInvalidWatchPattern:  false,
			watchIfNoWatchPatternFound: false,
			want:                       false,
		},
		{
			name: "no watch pattern annotation",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app`,
			filesChanged:               []string{"some-file.yaml"},
			ignoreInvalidWatchPattern:  false,
			watchIfNoWatchPatternFound: false,
			want:                       false,
		},
		{
			name: "no metadata",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
spec:
  destination:
    namespace: default`,
			filesChanged:               []string{"some-file.yaml"},
			ignoreInvalidWatchPattern:  false,
			watchIfNoWatchPatternFound: false,
			want:                       false,
		},
		{
			name: "nested path matching pattern",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  annotations:
    argocd-diff-preview/watch-pattern: 'apps/backend/.*\.yaml$'`,
			filesChanged:               []string{"apps/backend/deployment.yaml"},
			ignoreInvalidWatchPattern:  false,
			watchIfNoWatchPatternFound: false,
			want:                       true,
		},
		{
			name: "nested path non-matching pattern",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  annotations:
    argocd-diff-preview/watch-pattern: 'apps/backend/.*\.yaml$'`,
			filesChanged:               []string{"apps/frontend/deployment.yaml"},
			ignoreInvalidWatchPattern:  false,
			watchIfNoWatchPatternFound: false,
			want:                       false,
		},
		{
			name: "multiple nested patterns",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  annotations:
    argocd-diff-preview/watch-pattern: 'apps/backend/.*\.yaml$,apps/frontend/.*\.js$'`,
			filesChanged:               []string{"apps/frontend/index.js"},
			ignoreInvalidWatchPattern:  false,
			watchIfNoWatchPatternFound: false,
			want:                       true,
		},
		{
			name: "glob pattern with nested paths",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  annotations:
    argocd-diff-preview/watch-pattern: 'services/.*/k8s/.*\.yaml$'`,
			filesChanged:               []string{"services/auth-service/k8s/deployment.yaml"},
			ignoreInvalidWatchPattern:  false,
			watchIfNoWatchPatternFound: false,
			want:                       true,
		},
		{
			name: "glob pattern nested paths non-matching",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  annotations:
    argocd-diff-preview/watch-pattern: 'services/.*/k8s/.*\.yaml$'`,
			filesChanged:               []string{"services/auth-service/src/main.go"},
			ignoreInvalidWatchPattern:  false,
			watchIfNoWatchPatternFound: false,
			want:                       false,
		},
		{
			name: "nested path with multiple files changed",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  annotations:
    argocd-diff-preview/watch-pattern: 'microservices/user-service/.*\.yaml$'`,
			filesChanged:               []string{"microservices/auth-service/deployment.yaml", "microservices/user-service/service.yaml", "README.md"},
			ignoreInvalidWatchPattern:  false,
			watchIfNoWatchPatternFound: false,
			want:                       true,
		},
		{
			name: "nested path exact match",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  annotations:
    argocd-diff-preview/watch-pattern: '^some/path/to/specific/file\.yaml$'`,
			filesChanged:               []string{"some/path/to/specific/file.yaml"},
			ignoreInvalidWatchPattern:  false,
			watchIfNoWatchPatternFound: false,
			want:                       true,
		},
		{
			name: "nested path exact match non-matching",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  annotations:
    argocd-diff-preview/watch-pattern: '^some/path/to/specific/file\.yaml$'`,
			filesChanged:               []string{"some/path/to/different/file.yaml"},
			ignoreInvalidWatchPattern:  false,
			watchIfNoWatchPatternFound: false,
			want:                       false,
		},
		// Test cases for WatchIfNoWatchPatternFound behavior
		{
			name: "no watch pattern annotation with WatchIfNoWatchPatternFound=true",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app`,
			filesChanged:               []string{"some-file.yaml"},
			ignoreInvalidWatchPattern:  false,
			watchIfNoWatchPatternFound: true,
			want:                       true,
		},
		{
			name: "no watch pattern annotation with WatchIfNoWatchPatternFound=false",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app`,
			filesChanged:               []string{"some-file.yaml"},
			ignoreInvalidWatchPattern:  false,
			watchIfNoWatchPatternFound: false,
			want:                       false,
		},
		{
			name: "empty watch pattern with WatchIfNoWatchPatternFound=true",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  annotations:
    argocd-diff-preview/watch-pattern: ''`,
			filesChanged:               []string{"some-file.yaml"},
			ignoreInvalidWatchPattern:  false,
			watchIfNoWatchPatternFound: true,
			want:                       true,
		},
		{
			name: "empty watch pattern with WatchIfNoWatchPatternFound=false",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  annotations:
    argocd-diff-preview/watch-pattern: ''`,
			filesChanged:               []string{"some-file.yaml"},
			ignoreInvalidWatchPattern:  false,
			watchIfNoWatchPatternFound: false,
			want:                       false,
		},
		{
			name: "whitespace-only watch pattern with WatchIfNoWatchPatternFound=true",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  annotations:
    argocd-diff-preview/watch-pattern: '   '`,
			filesChanged:               []string{"some-file.yaml"},
			ignoreInvalidWatchPattern:  false,
			watchIfNoWatchPatternFound: true,
			want:                       true,
		},
		{
			name: "no annotations section with WatchIfNoWatchPatternFound=true",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app`,
			filesChanged:               []string{"some-file.yaml"},
			ignoreInvalidWatchPattern:  false,
			watchIfNoWatchPatternFound: true,
			want:                       true,
		},
		{
			name: "no annotations section with WatchIfNoWatchPatternFound=false",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app`,
			filesChanged:               []string{"some-file.yaml"},
			ignoreInvalidWatchPattern:  false,
			watchIfNoWatchPatternFound: false,
			want:                       false,
		},
		{
			name: "application file itself changed with WatchIfNoWatchPatternFound=false",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app`,
			filesChanged:               []string{"test.yaml"}, // matches FileName
			ignoreInvalidWatchPattern:  false,
			watchIfNoWatchPatternFound: false,
			want:                       true, // Should return true because app file itself changed
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
			got, _ := app.filterByFilesChanged(tt.filesChanged, tt.ignoreInvalidWatchPattern, tt.watchIfNoWatchPatternFound)

			// Check result
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFilter(t *testing.T) {

	zerolog.SetGlobalLevel(zerolog.FatalLevel)

	tests := []struct {
		name                       string
		yaml                       string
		selectors                  []selector.Selector
		filesChanged               []string
		ignoreInvalidWatchPattern  bool
		watchIfNoWatchPatternFound bool
		want                       bool
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
    argocd-diff-preview/watch-pattern: '.*\.yaml$'`,
			selectors:                  []selector.Selector{},
			filesChanged:               []string{},
			ignoreInvalidWatchPattern:  false,
			watchIfNoWatchPatternFound: false,
			want:                       true,
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
    argocd-diff-preview/watch-pattern: '.*\.yaml$'`,
			selectors: []selector.Selector{
				{Key: "app", Value: "test", Operator: selector.Eq},
			},
			filesChanged:               []string{},
			ignoreInvalidWatchPattern:  false,
			watchIfNoWatchPatternFound: false,
			want:                       true,
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
    argocd-diff-preview/watch-pattern: '.*\.yaml$'`,
			selectors: []selector.Selector{
				{Key: "app", Value: "other", Operator: selector.Eq},
			},
			filesChanged:               []string{},
			ignoreInvalidWatchPattern:  false,
			watchIfNoWatchPatternFound: false,
			want:                       false,
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
    argocd-diff-preview/watch-pattern: '.*\.yaml$'`,
			selectors:                  []selector.Selector{},
			filesChanged:               []string{"test.yaml"},
			ignoreInvalidWatchPattern:  false,
			watchIfNoWatchPatternFound: false,
			want:                       true,
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
    argocd-diff-preview/watch-pattern: '.*\.yaml$'`,
			selectors:                  []selector.Selector{},
			filesChanged:               []string{"test.txt"},
			ignoreInvalidWatchPattern:  false,
			watchIfNoWatchPatternFound: false,
			want:                       false,
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
    argocd-diff-preview/watch-pattern: '.*\.yaml$,.*\.txt$'`,
			selectors:                  []selector.Selector{},
			filesChanged:               []string{"test.txt"},
			ignoreInvalidWatchPattern:  false,
			watchIfNoWatchPatternFound: false,
			want:                       true,
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
    argocd-diff-preview/watch-pattern: '[invalid'`,
			selectors:                  []selector.Selector{},
			filesChanged:               []string{"test.yaml"},
			ignoreInvalidWatchPattern:  true,
			watchIfNoWatchPatternFound: false,
			want:                       true,
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
    argocd-diff-preview/watch-pattern: '[invalid'`,
			selectors:                  []selector.Selector{},
			filesChanged:               []string{"some-file.yaml"},
			ignoreInvalidWatchPattern:  false,
			watchIfNoWatchPatternFound: false,
			want:                       false,
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
    argocd-diff-preview/watch-pattern: ''`,
			selectors:                  []selector.Selector{},
			filesChanged:               []string{"some-file.yaml"},
			ignoreInvalidWatchPattern:  false,
			watchIfNoWatchPatternFound: false,
			want:                       false,
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
			selectors:                  []selector.Selector{},
			filesChanged:               []string{"some-file.yaml"},
			ignoreInvalidWatchPattern:  false,
			watchIfNoWatchPatternFound: false,
			want:                       false,
		},
		{
			name: "no metadata",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
spec:
  destination:
    namespace: default`,
			selectors:                  []selector.Selector{},
			filesChanged:               []string{"some-file.yaml"},
			ignoreInvalidWatchPattern:  false,
			watchIfNoWatchPatternFound: false,
			want:                       false,
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
    argocd-diff-preview/watch-pattern: '.*\\.yaml$'`,
			selectors: []selector.Selector{
				{Key: "app", Value: "other", Operator: selector.Ne},
			},
			filesChanged:               []string{},
			ignoreInvalidWatchPattern:  false,
			watchIfNoWatchPatternFound: false,
			want:                       true,
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
    argocd-diff-preview/watch-pattern: '.*\\.yaml$'`,
			selectors: []selector.Selector{
				{Key: "app", Value: "test", Operator: selector.Ne},
			},
			filesChanged:               []string{},
			ignoreInvalidWatchPattern:  false,
			watchIfNoWatchPatternFound: false,
			want:                       false,
		},
		{
			name: "ignore annotation",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  annotations:
    argocd-diff-preview/ignore: 'true'`,
			selectors:                  []selector.Selector{},
			filesChanged:               []string{},
			ignoreInvalidWatchPattern:  false,
			watchIfNoWatchPatternFound: false,
			want:                       false,
		},
		{
			name: "ignore annotation with watch pattern",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  annotations:
    argocd-diff-preview/ignore: "true"
    argocd-diff-preview/watch-pattern: '.*\\.yaml$'`,
			selectors:                  []selector.Selector{},
			filesChanged:               []string{},
			ignoreInvalidWatchPattern:  false,
			watchIfNoWatchPatternFound: false,
			want:                       false,
		},
		{
			name: "ignore annotation = false with matching selectors",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  labels:
    app: test
  annotations:
    argocd-diff-preview/ignore: 'false'`,
			selectors: []selector.Selector{
				{Key: "app", Value: "test", Operator: selector.Eq},
			},
			filesChanged:               []string{},
			ignoreInvalidWatchPattern:  false,
			watchIfNoWatchPatternFound: false,
			want:                       true,
		},
		{
			name: "ignore annotation = false with watch pattern",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  labels:
    app: test
  annotations:
    argocd-diff-preview/ignore: "false"
    argocd-diff-preview/watch-pattern: '.*\\.yaml$'`,
			selectors:                  []selector.Selector{},
			filesChanged:               []string{"test.yaml"},
			ignoreInvalidWatchPattern:  false,
			watchIfNoWatchPatternFound: false,
			want:                       true,
		},
		// Integration tests for WatchIfNoWatchPatternFound behavior
		{
			name: "no watch pattern with WatchIfNoWatchPatternFound=true should include app",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  labels:
    app: test`,
			selectors:                  []selector.Selector{},
			filesChanged:               []string{"some-file.yaml"},
			ignoreInvalidWatchPattern:  false,
			watchIfNoWatchPatternFound: true,
			want:                       true,
		},
		{
			name: "no watch pattern with WatchIfNoWatchPatternFound=false should exclude app",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  labels:
    app: test`,
			selectors:                  []selector.Selector{},
			filesChanged:               []string{"some-file.yaml"},
			ignoreInvalidWatchPattern:  false,
			watchIfNoWatchPatternFound: false,
			want:                       false,
		},
		{
			name: "matching selector but no watch pattern with WatchIfNoWatchPatternFound=true",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  labels:
    env: prod`,
			selectors: []selector.Selector{
				{Key: "env", Value: "prod", Operator: selector.Eq},
			},
			filesChanged:               []string{"some-file.yaml"},
			ignoreInvalidWatchPattern:  false,
			watchIfNoWatchPatternFound: true,
			want:                       true,
		},
		{
			name: "matching selector but no watch pattern with WatchIfNoWatchPatternFound=false",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  labels:
    env: prod`,
			selectors: []selector.Selector{
				{Key: "env", Value: "prod", Operator: selector.Eq},
			},
			filesChanged:               []string{"some-file.yaml"},
			ignoreInvalidWatchPattern:  false,
			watchIfNoWatchPatternFound: false,
			want:                       false,
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
			got := app.Filter(FilterOptions{
				Selector:                   tt.selectors,
				FilesChanged:               tt.filesChanged,
				IgnoreInvalidWatchPattern:  tt.ignoreInvalidWatchPattern,
				WatchIfNoWatchPatternFound: tt.watchIfNoWatchPatternFound,
			})

			// Check result
			assert.Equal(t, tt.want, got)
		})
	}
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
		{"no files changed", getYamlApp(t, ".", "source/path"), []string{}, false},
		{"relative path - matching", getYamlApp(t, ".", "source/path"), []string{"source/path/my-deployment.yaml"}, true},
		{"relative path, multi source - matching #1", getMultiSourceYamlApp(t, ".", "source/path", "other/path"), []string{"source/path/my-deployment.yaml"}, true},
		{"relative path, multi source - matching #2", getMultiSourceYamlApp(t, ".", "other/path", "source/path"), []string{"source/path/my-deployment.yaml"}, true},
		{"relative path - not matching", getYamlApp(t, ".", "source/path"), []string{"README.md"}, false},
		{"relative path, multi source - not matching", getMultiSourceYamlApp(t, ".", "other/path", "unrelated/path"), []string{"README.md"}, false},
		{"absolute path - matching", getYamlApp(t, "/source/path", "source/path"), []string{"source/path/my-deployment.yaml"}, true},
		{"absolute path, multi source - matching #1", getMultiSourceYamlApp(t, "/source/path", "source/path", "other/path"), []string{"source/path/my-deployment.yaml"}, true},
		{"absolute path, multi source - matching #2", getMultiSourceYamlApp(t, "/source/path", "other/path", "source/path"), []string{"source/path/my-deployment.yaml"}, true},
		{"absolute path - not matching", getYamlApp(t, "/source/path1", "source/path"), []string{"source/path/my-deployment.yaml"}, false},
		{"absolute path, multi source - not matching", getMultiSourceYamlApp(t, "/source/path1", "other/path", "source/path"), []string{"source/path/my-deployment.yaml"}, false},
		{"glob path * - matching", getYamlApp(t, "/source/**/my-deployment.yaml", "source/path"), []string{"source/path/my-deployment.yaml"}, true},
		{"glob path * - not matching", getYamlApp(t, "/source/**/my-service.yaml", "source/path"), []string{"source/path/my-deployment.yaml"}, false},
		{"glob path ? - matching", getYamlApp(t, "/source/path/my-deployment-?.yaml", "source/path"), []string{"source/path/my-deployment-0.yaml"}, true},
		{"glob path ? - not matching", getYamlApp(t, "/source/path/my-deployment-?.yaml", "source/path"), []string{"source/path/my-deployment.yaml"}, false},
		{"glob path char range - matching", getYamlApp(t, "/source/path[0-9]/my-deployment.yaml", "source/path"), []string{"source/path1/my-deployment.yaml"}, true},
		{"glob path char range - not matching", getYamlApp(t, "/source/path[0-9]/my-deployment.yaml", "source/path"), []string{"source/path/my-deployment.yaml"}, false},
		{"mixed glob path - matching", getYamlApp(t, "/source/path[0-9]/my-*.yaml", "source/path"), []string{"source/path1/my-deployment.yaml"}, true},
		{"mixed glob path - not matching", getYamlApp(t, "/source/path[0-9]/my-*.yaml", "source/path"), []string{"README.md"}, false},
		{"two relative paths - matching", getYamlApp(t, ".;../shared", "my-app"), []string{"shared/my-deployment.yaml"}, true},
		{"two relative paths, multi source - matching #1", getMultiSourceYamlApp(t, ".;../shared", "my-app", "other/path"), []string{"shared/my-deployment.yaml"}, true},
		{"two relative paths, multi source - matching #2", getMultiSourceYamlApp(t, ".;../shared", "my-app", "other/path"), []string{"shared/my-deployment.yaml"}, true},
		{"two relative paths - not matching", getYamlApp(t, ".;../shared", "my-app"), []string{"README.md"}, false},
		{"two relative paths, multi source - not matching", getMultiSourceYamlApp(t, ".;../shared", "my-app", "other/path"), []string{"README.md"}, false},
		{"file relative path - matching", getYamlApp(t, "./my-deployment.yaml", "source/path"), []string{"source/path/my-deployment.yaml"}, true},
		{"file relative path, multi source - matching #1", getMultiSourceYamlApp(t, "./my-deployment.yaml", "source/path", "other/path"), []string{"source/path/my-deployment.yaml"}, true},
		{"file relative path, multi source - matching #2", getMultiSourceYamlApp(t, "./my-deployment.yaml", "other/path", "source/path"), []string{"source/path/my-deployment.yaml"}, true},
		{"file relative path - not matching", getYamlApp(t, "./my-deployment.yaml", "source/path"), []string{"README.md"}, false},
		{"file relative path, multi source - not matching", getMultiSourceYamlApp(t, "./my-deployment.yaml", "source/path", "other/path"), []string{"README.md"}, false},
		{"file absolute path - matching", getYamlApp(t, "/source/path/my-deployment.yaml", "source/path"), []string{"source/path/my-deployment.yaml"}, true},
		{"file absolute path, multi source - matching #1", getMultiSourceYamlApp(t, "/source/path/my-deployment.yaml", "source/path", "other/path"), []string{"source/path/my-deployment.yaml"}, true},
		{"file absolute path, multi source - matching #2", getMultiSourceYamlApp(t, "/source/path/my-deployment.yaml", "other/path", "source/path"), []string{"source/path/my-deployment.yaml"}, true},
		{"file absolute path - not matching", getYamlApp(t, "/source/path1/README.md", "source/path"), []string{"source/path/my-deployment.yaml"}, false},
		{"file absolute path, multi source - not matching", getMultiSourceYamlApp(t, "/source/path1/README.md", "source/path", "other/path"), []string{"source/path/my-deployment.yaml"}, false},
		{"file two relative paths - matching", getYamlApp(t, "./README.md;../shared/my-deployment.yaml", "my-app"), []string{"shared/my-deployment.yaml"}, true},
		{"file two relative paths, multi source - matching", getMultiSourceYamlApp(t, "./README.md;../shared/my-deployment.yaml", "my-app", "other-path"), []string{"shared/my-deployment.yaml"}, true},
		{"file two relative paths - not matching", getYamlApp(t, ".README.md;../shared/my-deployment.yaml", "my-app"), []string{"kustomization.yaml"}, false},
		{"file two relative paths, multi source - not matching", getMultiSourceYamlApp(t, ".README.md;../shared/my-deployment.yaml", "my-app", "other-path"), []string{"kustomization.yaml"}, false},
		{"changed file absolute path - matching", getYamlApp(t, ".", "source/path"), []string{"/source/path/my-deployment.yaml"}, true},
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
			manifestGeneratePaths := annotations["argocd.argoproj.io/manifest-generate-paths"]
			got, _ := app.filterByManifestGeneratePaths(manifestGeneratePaths, ttc.files)
			assert.Equal(t, ttc.changeExpected, got)
		})
	}
}

func TestFilterApps(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.FatalLevel)

	baseBranch := git.NewBranch("main", git.Base)
	targetBranch := git.NewBranch("feature", git.Target)

	// Helper to create an ArgoResource from YAML
	createApp := func(name, fileName, yamlStr string, branch git.BranchType) ArgoResource {
		var node unstructured.Unstructured
		if err := yaml.Unmarshal([]byte(yamlStr), &node); err != nil {
			t.Fatalf("Error unmarshalling YAML: %v", err)
		}
		return ArgoResource{
			Yaml:     &node,
			Kind:     Application,
			Id:       name,
			Name:     name,
			FileName: fileName,
			Branch:   branch,
		}
	}

	t.Run("no filters - all apps selected", func(t *testing.T) {
		baseApps := []ArgoResource{
			createApp("app1", "app1.yaml", `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: app1`, git.Base),
		}
		targetApps := []ArgoResource{
			createApp("app1", "app1.yaml", `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: app1`, git.Target),
		}

		baseResult, targetResult := FilterApps(baseApps, targetApps, FilterOptions{}, baseBranch, targetBranch)

		assert.Len(t, baseResult, 1)
		assert.Len(t, targetResult, 1)
	})

	t.Run("ignore annotation filters out app", func(t *testing.T) {
		baseApps := []ArgoResource{
			createApp("app1", "app1.yaml", `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: app1
  annotations:
    argocd-diff-preview/ignore: "true"`, git.Base),
			createApp("app2", "app2.yaml", `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: app2`, git.Base),
		}
		targetApps := []ArgoResource{
			createApp("app1", "app1.yaml", `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: app1
  annotations:
    argocd-diff-preview/ignore: "true"`, git.Target),
			createApp("app2", "app2.yaml", `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: app2`, git.Target),
		}

		baseResult, targetResult := FilterApps(baseApps, targetApps, FilterOptions{}, baseBranch, targetBranch)

		assert.Len(t, baseResult, 1)
		assert.Len(t, targetResult, 1)
		assert.Equal(t, "app2", baseResult[0].Name)
		assert.Equal(t, "app2", targetResult[0].Name)
	})

	t.Run("ignore annotation only in one branch - app still excluded", func(t *testing.T) {
		baseApps := []ArgoResource{
			createApp("app1", "app1.yaml", `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: app1
  annotations:
    argocd-diff-preview/ignore: "true"`, git.Base),
		}
		targetApps := []ArgoResource{
			createApp("app1", "app1.yaml", `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: app1`, git.Target), // No ignore annotation in target
		}

		baseResult, targetResult := FilterApps(baseApps, targetApps, FilterOptions{}, baseBranch, targetBranch)

		// app1 is selected in target (no ignore), so both branches should include it
		assert.Len(t, baseResult, 1)
		assert.Len(t, targetResult, 1)
	})

	t.Run("label selector filters apps", func(t *testing.T) {
		baseApps := []ArgoResource{
			createApp("app1", "app1.yaml", `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: app1
  labels:
    env: prod`, git.Base),
			createApp("app2", "app2.yaml", `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: app2
  labels:
    env: dev`, git.Base),
		}
		targetApps := []ArgoResource{
			createApp("app1", "app1.yaml", `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: app1
  labels:
    env: prod`, git.Target),
			createApp("app2", "app2.yaml", `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: app2
  labels:
    env: dev`, git.Target),
		}

		filterOptions := FilterOptions{
			Selector: []selector.Selector{
				{Key: "env", Value: "prod", Operator: selector.Eq},
			},
		}

		baseResult, targetResult := FilterApps(baseApps, targetApps, filterOptions, baseBranch, targetBranch)

		assert.Len(t, baseResult, 1)
		assert.Len(t, targetResult, 1)
		assert.Equal(t, "app1", baseResult[0].Name)
		assert.Equal(t, "app1", targetResult[0].Name)
	})

	t.Run("watch pattern filters apps", func(t *testing.T) {
		baseApps := []ArgoResource{
			createApp("app1", "app1.yaml", `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: app1
  annotations:
    argocd-diff-preview/watch-pattern: 'backend/.*'`, git.Base),
			createApp("app2", "app2.yaml", `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: app2
  annotations:
    argocd-diff-preview/watch-pattern: 'frontend/.*'`, git.Base),
		}
		targetApps := []ArgoResource{
			createApp("app1", "app1.yaml", `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: app1
  annotations:
    argocd-diff-preview/watch-pattern: 'backend/.*'`, git.Target),
			createApp("app2", "app2.yaml", `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: app2
  annotations:
    argocd-diff-preview/watch-pattern: 'frontend/.*'`, git.Target),
		}

		filterOptions := FilterOptions{
			FilesChanged: []string{"backend/deployment.yaml"},
		}

		baseResult, targetResult := FilterApps(baseApps, targetApps, filterOptions, baseBranch, targetBranch)

		assert.Len(t, baseResult, 1)
		assert.Len(t, targetResult, 1)
		assert.Equal(t, "app1", baseResult[0].Name)
		assert.Equal(t, "app1", targetResult[0].Name)
	})

	t.Run("app selected in target but not base - both included", func(t *testing.T) {
		baseApps := []ArgoResource{
			createApp("app1", "app1.yaml", `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: app1
  labels:
    env: dev`, git.Base),
		}
		targetApps := []ArgoResource{
			createApp("app1", "app1.yaml", `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: app1
  labels:
    env: prod`, git.Target), // Changed to prod in target
		}

		filterOptions := FilterOptions{
			Selector: []selector.Selector{
				{Key: "env", Value: "prod", Operator: selector.Eq},
			},
		}

		baseResult, targetResult := FilterApps(baseApps, targetApps, filterOptions, baseBranch, targetBranch)

		// app1 is selected in target (has env=prod), so both should be included
		assert.Len(t, baseResult, 1)
		assert.Len(t, targetResult, 1)
	})

	t.Run("app selected in base but not target - both included", func(t *testing.T) {
		baseApps := []ArgoResource{
			createApp("app1", "app1.yaml", `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: app1
  labels:
    env: prod`, git.Base),
		}
		targetApps := []ArgoResource{
			createApp("app1", "app1.yaml", `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: app1
  labels:
    env: dev`, git.Target), // Changed to dev in target
		}

		filterOptions := FilterOptions{
			Selector: []selector.Selector{
				{Key: "env", Value: "prod", Operator: selector.Eq},
			},
		}

		baseResult, targetResult := FilterApps(baseApps, targetApps, filterOptions, baseBranch, targetBranch)

		// app1 is selected in base (has env=prod), so both should be included
		assert.Len(t, baseResult, 1)
		assert.Len(t, targetResult, 1)
	})

	t.Run("app not selected in either branch - excluded", func(t *testing.T) {
		baseApps := []ArgoResource{
			createApp("app1", "app1.yaml", `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: app1
  labels:
    env: dev`, git.Base),
		}
		targetApps := []ArgoResource{
			createApp("app1", "app1.yaml", `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: app1
  labels:
    env: staging`, git.Target),
		}

		filterOptions := FilterOptions{
			Selector: []selector.Selector{
				{Key: "env", Value: "prod", Operator: selector.Eq},
			},
		}

		baseResult, targetResult := FilterApps(baseApps, targetApps, filterOptions, baseBranch, targetBranch)

		// Neither has env=prod, so both should be empty
		assert.Len(t, baseResult, 0)
		assert.Len(t, targetResult, 0)
	})

	t.Run("new app only in target - included", func(t *testing.T) {
		baseApps := []ArgoResource{}
		targetApps := []ArgoResource{
			createApp("app1", "app1.yaml", `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: app1`, git.Target),
		}

		baseResult, targetResult := FilterApps(baseApps, targetApps, FilterOptions{}, baseBranch, targetBranch)

		assert.Len(t, baseResult, 0)
		assert.Len(t, targetResult, 1)
	})

	t.Run("deleted app only in base - included", func(t *testing.T) {
		baseApps := []ArgoResource{
			createApp("app1", "app1.yaml", `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: app1`, git.Base),
		}
		targetApps := []ArgoResource{}

		baseResult, targetResult := FilterApps(baseApps, targetApps, FilterOptions{}, baseBranch, targetBranch)

		assert.Len(t, baseResult, 1)
		assert.Len(t, targetResult, 0)
	})

	t.Run("duplicate apps with same key - all included when one is selected", func(t *testing.T) {
		baseApps := []ArgoResource{
			createApp("app1", "app1.yaml", `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: app1
  labels:
    env: prod`, git.Base),
			createApp("app1", "app1.yaml", `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: app1
  labels:
    env: dev`, git.Base), // Same key but different labels
		}
		targetApps := []ArgoResource{
			createApp("app1", "app1.yaml", `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: app1
  labels:
    env: prod`, git.Target),
			createApp("app1", "app1.yaml", `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: app1
  labels:
    env: dev`, git.Target),
		}

		filterOptions := FilterOptions{
			Selector: []selector.Selector{
				{Key: "env", Value: "prod", Operator: selector.Eq},
			},
		}

		baseResult, targetResult := FilterApps(baseApps, targetApps, filterOptions, baseBranch, targetBranch)

		// Both apps share the same key, and one matches env=prod, so both should be included
		assert.Len(t, baseResult, 2)
		assert.Len(t, targetResult, 2)
	})

	t.Run("combined ignore annotation and selector", func(t *testing.T) {
		baseApps := []ArgoResource{
			createApp("app1", "app1.yaml", `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: app1
  labels:
    env: prod
  annotations:
    argocd-diff-preview/ignore: "true"`, git.Base),
			createApp("app2", "app2.yaml", `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: app2
  labels:
    env: prod`, git.Base),
		}
		targetApps := []ArgoResource{
			createApp("app1", "app1.yaml", `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: app1
  labels:
    env: prod
  annotations:
    argocd-diff-preview/ignore: "true"`, git.Target),
			createApp("app2", "app2.yaml", `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: app2
  labels:
    env: prod`, git.Target),
		}

		filterOptions := FilterOptions{
			Selector: []selector.Selector{
				{Key: "env", Value: "prod", Operator: selector.Eq},
			},
		}

		baseResult, targetResult := FilterApps(baseApps, targetApps, filterOptions, baseBranch, targetBranch)

		// app1 is ignored despite matching selector, app2 should be included
		assert.Len(t, baseResult, 1)
		assert.Len(t, targetResult, 1)
		assert.Equal(t, "app2", baseResult[0].Name)
	})

	t.Run("combined selector and watch pattern", func(t *testing.T) {
		baseApps := []ArgoResource{
			createApp("app1", "app1.yaml", `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: app1
  labels:
    env: prod
  annotations:
    argocd-diff-preview/watch-pattern: 'backend/.*'`, git.Base),
			createApp("app2", "app2.yaml", `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: app2
  labels:
    env: prod
  annotations:
    argocd-diff-preview/watch-pattern: 'frontend/.*'`, git.Base),
		}
		targetApps := []ArgoResource{
			createApp("app1", "app1.yaml", `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: app1
  labels:
    env: prod
  annotations:
    argocd-diff-preview/watch-pattern: 'backend/.*'`, git.Target),
			createApp("app2", "app2.yaml", `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: app2
  labels:
    env: prod
  annotations:
    argocd-diff-preview/watch-pattern: 'frontend/.*'`, git.Target),
		}

		filterOptions := FilterOptions{
			Selector: []selector.Selector{
				{Key: "env", Value: "prod", Operator: selector.Eq},
			},
			FilesChanged: []string{"backend/deployment.yaml"},
		}

		baseResult, targetResult := FilterApps(baseApps, targetApps, filterOptions, baseBranch, targetBranch)

		// Both match selector, but only app1 matches watch pattern
		assert.Len(t, baseResult, 1)
		assert.Len(t, targetResult, 1)
		assert.Equal(t, "app1", baseResult[0].Name)
	})

	t.Run("manifest-generate-paths annotation", func(t *testing.T) {
		baseApps := []ArgoResource{
			createApp("app1", "app1.yaml", `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: app1
  annotations:
    argocd.argoproj.io/manifest-generate-paths: .
spec:
  source:
    path: apps/backend`, git.Base),
		}
		targetApps := []ArgoResource{
			createApp("app1", "app1.yaml", `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: app1
  annotations:
    argocd.argoproj.io/manifest-generate-paths: .
spec:
  source:
    path: apps/backend`, git.Target),
		}

		filterOptions := FilterOptions{
			FilesChanged: []string{"apps/backend/deployment.yaml"},
		}

		baseResult, targetResult := FilterApps(baseApps, targetApps, filterOptions, baseBranch, targetBranch)

		assert.Len(t, baseResult, 1)
		assert.Len(t, targetResult, 1)
	})

	t.Run("app file itself changed", func(t *testing.T) {
		baseApps := []ArgoResource{
			createApp("app1", "apps/app1.yaml", `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: app1`, git.Base),
		}
		targetApps := []ArgoResource{
			createApp("app1", "apps/app1.yaml", `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: app1`, git.Target),
		}

		filterOptions := FilterOptions{
			FilesChanged: []string{"apps/app1.yaml"},
		}

		baseResult, targetResult := FilterApps(baseApps, targetApps, filterOptions, baseBranch, targetBranch)

		// App file itself is in files changed, so it should be selected
		assert.Len(t, baseResult, 1)
		assert.Len(t, targetResult, 1)
	})

	t.Run("watchIfNoWatchPatternFound true", func(t *testing.T) {
		baseApps := []ArgoResource{
			createApp("app1", "app1.yaml", `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: app1`, git.Base), // No watch pattern
		}
		targetApps := []ArgoResource{
			createApp("app1", "app1.yaml", `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: app1`, git.Target),
		}

		filterOptions := FilterOptions{
			FilesChanged:               []string{"some/file.yaml"},
			WatchIfNoWatchPatternFound: true,
		}

		baseResult, targetResult := FilterApps(baseApps, targetApps, filterOptions, baseBranch, targetBranch)

		assert.Len(t, baseResult, 1)
		assert.Len(t, targetResult, 1)
	})

	t.Run("watchIfNoWatchPatternFound false", func(t *testing.T) {
		baseApps := []ArgoResource{
			createApp("app1", "app1.yaml", `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: app1`, git.Base), // No watch pattern
		}
		targetApps := []ArgoResource{
			createApp("app1", "app1.yaml", `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: app1`, git.Target),
		}

		filterOptions := FilterOptions{
			FilesChanged:               []string{"some/file.yaml"},
			WatchIfNoWatchPatternFound: false,
		}

		baseResult, targetResult := FilterApps(baseApps, targetApps, filterOptions, baseBranch, targetBranch)

		assert.Len(t, baseResult, 0)
		assert.Len(t, targetResult, 0)
	})
}

func getYamlApp(t *testing.T, annotation string, sourcePath string) *unstructured.Unstructured {
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
		t.Fatalf("Error unmarshalling YAML: %v", err)
	}
	return &node
}

func getMultiSourceYamlApp(t *testing.T, annotation string, paths ...string) *unstructured.Unstructured {
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
		t.Fatalf("Error unmarshalling YAML: %v", err)
	}
	return &node
}

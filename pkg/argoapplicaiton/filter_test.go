package argoapplicaiton

import (
	"testing"

	"github.com/dag-andersen/argocd-diff-preview/pkg/selector"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
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
			var node yaml.Node
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
			var node yaml.Node
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
			want:                      &ArgoResource{Name: "test-app"},
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
			want:                      &ArgoResource{Name: "test-app"},
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
			want:                      &ArgoResource{Name: "test-app"},
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
			want:                      &ArgoResource{Name: "test-app"},
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
			want:                      &ArgoResource{Name: "test-app"},
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
			want:                      &ArgoResource{Name: "test-app"},
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
			var node yaml.Node
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
			got := app.Filter(tt.selectors, tt.filesChanged, tt.ignoreInvalidWatchPattern)

			// Check result
			if tt.want == nil {
				assert.Nil(t, got)
			} else {
				assert.NotNil(t, got)
				assert.Equal(t, tt.want.Name, got.Name)
			}
		})
	}
}

package argoapplication

import (
	"fmt"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

func TestRedirectGenerators(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.FatalLevel)

	const (
		repo   = "https://github.com/org/repo.git"
		branch = "target"
	)

	tests := []struct {
		name              string
		yaml              string
		want              string
		redirectRevisions []string
		expectErr         error
	}{
		{
			name: "application set with git generator and redirect all revisions",
			yaml: applicationSetSpec(`
    generators:
        - git:
            repoURL: https://github.com/org/repo.git
            revision: HEAD
`),
			want: applicationSetSpec(`
    generators:
        - git:
            repoURL: https://github.com/org/repo.git
            revision: target
`),
			redirectRevisions: []string{},
			expectErr:         nil,
		},
		{
			name: "application set with multiple git generators and redirect all revisions",
			yaml: applicationSetSpec(`
    generators:
        - git:
            repoURL: https://github.com/org/repo.git
            revision: HEAD
        - git:
            repoURL: https://github.com/org/repo.git
            revision: main
`),
			want: applicationSetSpec(`
    generators:
        - git:
            repoURL: https://github.com/org/repo.git
            revision: target
        - git:
            repoURL: https://github.com/org/repo.git
            revision: target
`),
			redirectRevisions: []string{},
			expectErr:         nil,
		},
		{
			name: "application set with multiple git generators and redirect only specific revisions",
			yaml: applicationSetSpec(`
    generators:
        - git:
            repoURL: https://github.com/org/repo.git
            revision: HEAD
        - git:
            repoURL: https://github.com/org/repo.git
            revision: main
        - git:
            repoURL: https://github.com/org/repo.git
            revision: 0.9.9
`),
			want: applicationSetSpec(`
    generators:
        - git:
            repoURL: https://github.com/org/repo.git
            revision: target
        - git:
            repoURL: https://github.com/org/repo.git
            revision: target
        - git:
            repoURL: https://github.com/org/repo.git
            revision: 0.9.9
`),
			redirectRevisions: []string{"HEAD", "main"},
			expectErr:         nil,
		},
		{
			name: "application set with matrix generator and redirect all revisions",
			yaml: applicationSetSpec(`
    generators:
        - matrix:
            generators:
                - git:
                    repoURL: https://github.com/org/repo.git
                    revision: HEAD
                - clusters:
                    selector:
                        matchLabels:
                            argocd.argoproj.io/secret-type: cluster
        - git:
            repoURL: https://github.com/org/repo.git
            revision: HEAD
`),
			want: applicationSetSpec(`
    generators:
        - matrix:
            generators:
                - git:
                    repoURL: https://github.com/org/repo.git
                    revision: target
                - clusters:
                    selector:
                        matchLabels:
                            argocd.argoproj.io/secret-type: cluster
        - git:
            repoURL: https://github.com/org/repo.git
            revision: target
`),
			redirectRevisions: []string{},
			expectErr:         nil,
		},
		{
			name: "application set with matrix generator and redirect only specific revisions",
			yaml: applicationSetSpec(`
    generators:
        - matrix:
            generators:
                - git:
                    repoURL: https://github.com/org/repo.git
                    revision: HEAD
                - git:
                    repoURL: https://github.com/org/repo.git
                    revision: 0.9.9
        - git:
            repoURL: https://github.com/org/repo.git
            revision: HEAD
`),
			want: applicationSetSpec(`
    generators:
        - matrix:
            generators:
                - git:
                    repoURL: https://github.com/org/repo.git
                    revision: target
                - git:
                    repoURL: https://github.com/org/repo.git
                    revision: 0.9.9
        - git:
            repoURL: https://github.com/org/repo.git
            revision: target
`),
			redirectRevisions: []string{"HEAD", "main"},
			expectErr:         nil,
		},
		{
			name: "application set with nested matrix generators and redirect only specific revisions",
			yaml: applicationSetSpec(`
    generators:
        - matrix:
            generators:
                - matrix:
                    generators:
                        - git:
                            repoURL: https://github.com/org/repo.git
                            revision: HEAD
                        - git:
                            repoURL: https://github.com/org/repo.git
                            revision: 0.9.9
                - git:
                    repoURL: https://github.com/org/repo.git
                    revision: HEAD
        - git:
            repoURL: https://github.com/org/repo.git
            revision: HEAD
`),
			want: applicationSetSpec(`
    generators:
        - matrix:
            generators:
                - matrix:
                    generators:
                        - git:
                            repoURL: https://github.com/org/repo.git
                            revision: target
                        - git:
                            repoURL: https://github.com/org/repo.git
                            revision: 0.9.9
                - git:
                    repoURL: https://github.com/org/repo.git
                    revision: target
        - git:
            repoURL: https://github.com/org/repo.git
            revision: target
`),
			redirectRevisions: []string{"HEAD", "main"},
			expectErr:         nil,
		},
		{
			name: "application set with too many levels of nested matrix generators",
			yaml: applicationSetSpec(`
    generators:
        - matrix:
            generators:
                - matrix:
                    generators:
                        - matrix:
                            generators:
                                - git:
                                    repoURL: https://github.com/org/repo.git
                                    revision: HEAD
`),
			want:              "",
			redirectRevisions: []string{},
			expectErr:         fmt.Errorf("too many levels of nested matrix generators in ApplicationSet: %s", "test-set"),
		},
		{
			name: "application set with too many child generators in matrix generators",
			yaml: applicationSetSpec(`
    generators:
        - matrix:
            generators:
                - git:
                    repoURL: https://github.com/org/repo.git
                    revision: HEAD
                - git:
                    repoURL: https://github.com/org/repo.git
                    revision: main
                - git:
                    repoURL: https://github.com/org/repo.git
                    revision: 0.9.9
`),
			want:              "",
			redirectRevisions: []string{},
			expectErr:         fmt.Errorf("only 2 child generators are allowed for matrix generator '%s' in ApplicationSet: %s", "spec.generators[0].matrix", "test-set"),
		},
		{
			name: "application set with complex nested structure - matrix with clusters and merge generators",
			yaml: applicationSetSpec(`
    generators:
        - matrix:
            generators:
                - clusters:
                    selector:
                        matchLabels:
                            fleet: test
                - merge:
                    mergeKeys:
                        - app
                    generators:
                        - list:
                            elements:
                                - app: test-app
                                  repoURL: https://github.com/org/test-app
                                  namespace: system
                        - git:
                            repoURL: https://github.com/org/repo.git
                            files:
                                - path: development.yaml
                            revision: main
`),
			want: applicationSetSpec(`
    generators:
        - matrix:
            generators:
                - clusters:
                    selector:
                        matchLabels:
                            fleet: test
                - merge:
                    mergeKeys:
                        - app
                    generators:
                        - list:
                            elements:
                                - app: test-app
                                  repoURL: https://github.com/org/test-app
                                  namespace: system
                        - git:
                            repoURL: https://github.com/org/repo.git
                            files:
                                - path: development.yaml
                            revision: target
`),
			redirectRevisions: []string{},
			expectErr:         nil,
		},
		{
			name: "application set with basic merge generator and redirect all revisions",
			yaml: applicationSetSpec(`
    generators:
        - merge:
            mergeKeys:
                - app
            generators:
                - list:
                    elements:
                        - app: app1
                          namespace: default
                - git:
                    repoURL: https://github.com/org/repo.git
                    revision: HEAD
                    files:
                        - path: config.yaml
`),
			want: applicationSetSpec(`
    generators:
        - merge:
            mergeKeys:
                - app
            generators:
                - list:
                    elements:
                        - app: app1
                          namespace: default
                - git:
                    repoURL: https://github.com/org/repo.git
                    revision: target
                    files:
                        - path: config.yaml
`),
			redirectRevisions: []string{},
			expectErr:         nil,
		},
		{
			name: "application set with basic merge generator and redirect only specific revisions",
			yaml: applicationSetSpec(`
    generators:
        - merge:
            mergeKeys:
                - app
            generators:
                - list:
                    elements:
                        - app: app1
                          namespace: default
                - git:
                    repoURL: https://github.com/org/repo.git
                    revision: main
                    files:
                        - path: config.yaml
                - git:
                    repoURL: https://github.com/org/repo.git
                    revision: v1.0.0
                    files:
                        - path: versions.yaml
`),
			want: applicationSetSpec(`
    generators:
        - merge:
            mergeKeys:
                - app
            generators:
                - list:
                    elements:
                        - app: app1
                          namespace: default
                - git:
                    repoURL: https://github.com/org/repo.git
                    revision: target
                    files:
                        - path: config.yaml
                - git:
                    repoURL: https://github.com/org/repo.git
                    revision: v1.0.0
                    files:
                        - path: versions.yaml
`),
			redirectRevisions: []string{"main", "HEAD"},
			expectErr:         nil,
		},
		{
			name: "application set with merge generator containing multiple git generators",
			yaml: applicationSetSpec(`
    generators:
        - merge:
            mergeKeys:
                - app
            generators:
                - git:
                    repoURL: https://github.com/org/repo.git
                    revision: HEAD
                    files:
                        - path: apps.yaml
                - git:
                    repoURL: https://github.com/org/repo.git
                    revision: main
                    files:
                        - path: config.yaml
`),
			want: applicationSetSpec(`
    generators:
        - merge:
            mergeKeys:
                - app
            generators:
                - git:
                    repoURL: https://github.com/org/repo.git
                    revision: target
                    files:
                        - path: apps.yaml
                - git:
                    repoURL: https://github.com/org/repo.git
                    revision: target
                    files:
                        - path: config.yaml
`),
			redirectRevisions: []string{},
			expectErr:         nil,
		},
		{
			name: "application set with nested merge generators",
			yaml: applicationSetSpec(`
    generators:
        - merge:
            mergeKeys:
                - app
            generators:
                - list:
                    elements:
                        - app: app1
                          namespace: default
                - merge:
                    mergeKeys:
                        - version
                    generators:
                        - git:
                            repoURL: https://github.com/org/repo.git
                            revision: HEAD
                            files:
                                - path: versions.yaml
                        - git:
                            repoURL: https://github.com/org/repo.git
                            revision: main
                            files:
                                - path: config.yaml
`),
			want: applicationSetSpec(`
    generators:
        - merge:
            mergeKeys:
                - app
            generators:
                - list:
                    elements:
                        - app: app1
                          namespace: default
                - merge:
                    mergeKeys:
                        - version
                    generators:
                        - git:
                            repoURL: https://github.com/org/repo.git
                            revision: target
                            files:
                                - path: versions.yaml
                        - git:
                            repoURL: https://github.com/org/repo.git
                            revision: target
                            files:
                                - path: config.yaml
`),
			redirectRevisions: []string{},
			expectErr:         nil,
		},
		{
			name: "application set with merge generator and non-matching repoURL",
			yaml: applicationSetSpec(`
    generators:
        - merge:
            mergeKeys:
                - app
            generators:
                - list:
                    elements:
                        - app: app1
                          namespace: default
                - git:
                    repoURL: https://github.com/other/repo.git
                    revision: HEAD
                    files:
                        - path: config.yaml
`),
			want: applicationSetSpec(`
    generators:
        - merge:
            mergeKeys:
                - app
            generators:
                - list:
                    elements:
                        - app: app1
                          namespace: default
                - git:
                    repoURL: https://github.com/other/repo.git
                    revision: HEAD
                    files:
                        - path: config.yaml
`),
			redirectRevisions: []string{},
			expectErr:         nil,
		},
		{
			name: "application set with merge generator containing only non-git generators",
			yaml: applicationSetSpec(`
    generators:
        - merge:
            mergeKeys:
                - app
            generators:
                - list:
                    elements:
                        - app: app1
                          namespace: default
                - clusters:
                    selector:
                        matchLabels:
                            env: production
`),
			want: applicationSetSpec(`
    generators:
        - merge:
            mergeKeys:
                - app
            generators:
                - list:
                    elements:
                        - app: app1
                          namespace: default
                - clusters:
                    selector:
                        matchLabels:
                            env: production
`),
			redirectRevisions: []string{},
			expectErr:         nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse YAML
			var node map[string]interface{}
			err := yaml.Unmarshal([]byte(tt.yaml), &node)
			assert.NoError(t, err)

			// Create ArgoResource
			app := &ArgoResource{
				Yaml:     &unstructured.Unstructured{Object: node},
				Kind:     ApplicationSet,
				Id:       "test-set",
				Name:     "test-set",
				FileName: "test-set.yaml",
			}

			// Run redirect generators
			err = app.RedirectGenerators(repo, branch, tt.redirectRevisions)

			// Check result
			if tt.expectErr == nil {
				assert.Nil(t, err)
				got, err := app.AsString()
				assert.Nil(t, err)
				assert.Equal(t, tt.want, got)
			} else {
				assert.Equal(t, tt.expectErr.Error(), err.Error())
			}
		})
	}
}

func applicationSetSpec(spec string) string {
	metadata := `
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: test-set
  namespace: default
spec:`

	yamlString := fmt.Sprintf("%s%s", metadata, spec)

	// convert to yaml unstructured
	unstructured := &unstructured.Unstructured{}
	err := yaml.Unmarshal([]byte(yamlString), unstructured)
	if err != nil {
		panic(err)
	}

	// convert back to yaml string
	yamlBytes, err := yaml.Marshal(unstructured)
	if err != nil {
		panic(err)
	}

	return string(yamlBytes)
}

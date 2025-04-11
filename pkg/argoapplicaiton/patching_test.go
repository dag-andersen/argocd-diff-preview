package argoapplicaiton

import (
	"fmt"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
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
				Kind:     ApplicationSet,
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
	metadata := `apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
    name: test-set
    namespace: default
spec:`

	return fmt.Sprintf("%s%s", metadata, spec)
}

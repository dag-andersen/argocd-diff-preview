package argoapplication

import (
	"fmt"
	"testing"

	"github.com/dag-andersen/argocd-diff-preview/pkg/git"
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
			var node map[string]any
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

func TestPointDestinationToInCluster(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.FatalLevel)

	tests := []struct {
		name      string
		kind      ApplicationKind
		yaml      string
		want      string
		expectErr error
	}{
		{
			name: "application with destination should modify destination",
			kind: Application,
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  namespace: default
spec:
  destination:
    name: my-cluster
    namespace: default
`,
			want: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  namespace: default
spec:
  destination:
    server: https://kubernetes.default.svc
    namespace: default
`,
			expectErr: nil,
		},
		{
			name: "application set with destination should not modify destination",
			kind: ApplicationSet,
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: test-set
  namespace: default
spec:
  template:
    spec:
      destination:
        server: https://kubernetes.default.svc
        namespace: default
`,
			want: `
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: test-set
  namespace: default
spec:
  template:
    spec:
      destination:
        server: https://kubernetes.default.svc
        namespace: default
`,
			expectErr: nil,
		},
		{
			name: "application without destination should do nothing",
			kind: Application,
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  namespace: default
spec:
  source:
    repoURL: https://github.com/org/repo.git
`,
			want: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  namespace: default
spec:
  source:
    repoURL: https://github.com/org/repo.git
`,
			expectErr: nil,
		},
		{
			name: "application set without destination should do nothing",
			kind: ApplicationSet,
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: test-set
  namespace: default
spec:
  generators:
    - list: {}
`,
			want: `
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: test-set
  namespace: default
spec:
  generators:
    - list: {}
`,
			expectErr: nil,
		},
		{
			name: "application with destination containing name should delete name and set server",
			kind: Application,
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  namespace: default
spec:
  destination:
    name: my-cluster
    server: https://other-server.example.com
    namespace: default
`,
			want: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  namespace: default
spec:
  destination:
    server: https://kubernetes.default.svc
    namespace: default
`,
			expectErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse YAML
			var node map[string]any
			err := yaml.Unmarshal([]byte(tt.yaml), &node)
			assert.NoError(t, err)

			// Create ArgoResource
			app := &ArgoResource{
				Yaml:     &unstructured.Unstructured{Object: node},
				Kind:     tt.kind,
				Id:       "test-resource",
				Name:     "test-resource",
				FileName: "test-resource.yaml",
			}

			// Run PointDestinationToInCluster
			err = app.SetDestinationServerToLocal()

			// Check result
			if tt.expectErr == nil {
				assert.Nil(t, err)
				got, err := app.AsString()
				assert.Nil(t, err)

				// Normalize both expected and actual YAML for comparison
				expectedNormalized := normalizeYAML(tt.want)
				gotNormalized := normalizeYAML(got)
				assert.Equal(t, expectedNormalized, gotNormalized)
			} else {
				assert.Equal(t, tt.expectErr.Error(), err.Error())
			}
		})
	}
}

// normalizeYAML normalizes YAML strings by parsing and re-marshaling
func normalizeYAML(yamlStr string) string {
	var node map[string]any
	err := yaml.Unmarshal([]byte(yamlStr), &node)
	if err != nil {
		return yamlStr
	}

	yamlBytes, err := yaml.Marshal(node)
	if err != nil {
		return yamlStr
	}

	return string(yamlBytes)
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

// TestPatchApplication verifies the full PatchApplication pipeline on a single
// ArgoResource. Each sub-test exercises one or more of the patches that
// PatchApplication chains together (namespace, project, destination, sync
// policy, finalizers, source redirect).
func TestPatchApplication(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.FatalLevel)

	const (
		argocdNamespace = "argocd"
		prRepo          = "https://github.com/org/repo.git"
		branchName      = "my-feature"
	)

	branch := git.NewBranch(branchName, git.Target)

	tests := []struct {
		name              string
		kind              ApplicationKind
		inputYAML         string
		wantYAML          string
		redirectRevisions []string
	}{
		{
			name: "namespace is set to argocd",
			kind: Application,
			inputYAML: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-app
  namespace: some-other-namespace
spec:
  project: my-project
  source:
    repoURL: https://github.com/org/repo.git
    targetRevision: HEAD
    path: apps/my-app
  destination:
    server: https://kubernetes.default.svc
    namespace: default
`,
			wantYAML: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-app
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/org/repo.git
    targetRevision: my-feature
    path: apps/my-app
  destination:
    server: https://kubernetes.default.svc
    namespace: default
`,
		},
		{
			name: "sync policy is removed",
			kind: Application,
			inputYAML: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: sync-app
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/org/repo.git
    targetRevision: HEAD
    path: apps/sync-app
  destination:
    server: https://kubernetes.default.svc
    namespace: default
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
      - CreateNamespace=true
`,
			wantYAML: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: sync-app
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/org/repo.git
    targetRevision: my-feature
    path: apps/sync-app
  destination:
    server: https://kubernetes.default.svc
    namespace: default
`,
		},
		{
			name: "project is reset to default",
			kind: Application,
			inputYAML: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: proj-app
  namespace: argocd
spec:
  project: production
  source:
    repoURL: https://github.com/org/repo.git
    targetRevision: HEAD
    path: apps/proj-app
  destination:
    server: https://kubernetes.default.svc
    namespace: default
`,
			wantYAML: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: proj-app
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/org/repo.git
    targetRevision: my-feature
    path: apps/proj-app
  destination:
    server: https://kubernetes.default.svc
    namespace: default
`,
		},
		{
			name: "destination server is redirected to in-cluster",
			kind: Application,
			inputYAML: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: dest-app
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/org/repo.git
    targetRevision: HEAD
    path: apps/dest-app
  destination:
    name: remote-cluster
    namespace: production
`,
			wantYAML: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: dest-app
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/org/repo.git
    targetRevision: my-feature
    path: apps/dest-app
  destination:
    server: https://kubernetes.default.svc
    namespace: production
`,
		},
		{
			name: "argocd finalizer is removed",
			kind: Application,
			inputYAML: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: final-app
  namespace: argocd
  finalizers:
    - resources-finalizer.argocd.argoproj.io
    - some-other-finalizer
spec:
  project: default
  source:
    repoURL: https://github.com/org/repo.git
    targetRevision: HEAD
    path: apps/final-app
  destination:
    server: https://kubernetes.default.svc
    namespace: default
`,
			wantYAML: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: final-app
  namespace: argocd
  finalizers:
    - some-other-finalizer
spec:
  project: default
  source:
    repoURL: https://github.com/org/repo.git
    targetRevision: my-feature
    path: apps/final-app
  destination:
    server: https://kubernetes.default.svc
    namespace: default
`,
		},
		{
			name: "source targetRevision is redirected to branch",
			kind: Application,
			inputYAML: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: src-app
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/org/repo.git
    targetRevision: main
    path: apps/src-app
  destination:
    server: https://kubernetes.default.svc
    namespace: default
`,
			wantYAML: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: src-app
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/org/repo.git
    targetRevision: my-feature
    path: apps/src-app
  destination:
    server: https://kubernetes.default.svc
    namespace: default
`,
		},
		{
			name: "source revision not redirected when repoURL does not match",
			kind: Application,
			inputYAML: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: external-app
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/other/unrelated.git
    targetRevision: HEAD
    path: apps/external-app
  destination:
    server: https://kubernetes.default.svc
    namespace: default
`,
			wantYAML: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: external-app
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/other/unrelated.git
    targetRevision: HEAD
    path: apps/external-app
  destination:
    server: https://kubernetes.default.svc
    namespace: default
`,
		},
		{
			name: "specific redirectRevisions: only matching revision is redirected",
			kind: Application,
			inputYAML: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: selective-app
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/org/repo.git
    targetRevision: HEAD
    path: apps/selective-app
  destination:
    server: https://kubernetes.default.svc
    namespace: default
`,
			wantYAML: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: selective-app
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/org/repo.git
    targetRevision: my-feature
    path: apps/selective-app
  destination:
    server: https://kubernetes.default.svc
    namespace: default
`,
			redirectRevisions: []string{"HEAD", "main"},
		},
		{
			name: "specific redirectRevisions: non-matching revision is left unchanged",
			kind: Application,
			inputYAML: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: pinned-app
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/org/repo.git
    targetRevision: v1.2.3
    path: apps/pinned-app
  destination:
    server: https://kubernetes.default.svc
    namespace: default
`,
			wantYAML: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: pinned-app
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/org/repo.git
    targetRevision: v1.2.3
    path: apps/pinned-app
  destination:
    server: https://kubernetes.default.svc
    namespace: default
`,
			redirectRevisions: []string{"HEAD", "main"},
		},
		{
			name: "ApplicationSet namespace and project patched",
			kind: ApplicationSet,
			inputYAML: `
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: my-appset
  namespace: some-namespace
spec:
  generators:
    - git:
        repoURL: https://github.com/org/repo.git
        revision: HEAD
  template:
    metadata:
      namespace: argocd
    spec:
      project: platform
      destination:
        server: https://kubernetes.default.svc
        namespace: default
`,
			wantYAML: `
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: my-appset
  namespace: argocd
spec:
  generators:
    - git:
        repoURL: https://github.com/org/repo.git
        revision: my-feature
  template:
    metadata:
      namespace: argocd
    spec:
      project: default
      destination:
        server: https://kubernetes.default.svc
        namespace: default
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var obj map[string]any
			err := yaml.Unmarshal([]byte(tt.inputYAML), &obj)
			assert.NoError(t, err)

			resource := &ArgoResource{
				Yaml:     &unstructured.Unstructured{Object: obj},
				Kind:     tt.kind,
				Id:       "test-id",
				Name:     "test-name",
				FileName: "parent: root-app",
			}

			redirectRevisions := tt.redirectRevisions

			// ── Normal cases ────────────────────────────────────────────────
			patched, err := PatchApplication(argocdNamespace, *resource, branch, prRepo, redirectRevisions)
			assert.NoError(t, err)
			assert.NotNil(t, patched)

			// FileName breadcrumb must be preserved unchanged.
			assert.Equal(t, resource.FileName, patched.FileName,
				"PatchApplication must preserve FileName")

			got, err := patched.AsString()
			assert.NoError(t, err)

			assert.Equal(t, normalizeYAML(tt.wantYAML), normalizeYAML(got))
		})
	}
}

func TestRedirectSourceHydrator(t *testing.T) {
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
			name: "application with sourceHydrator converts to regular application",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  namespace: default
spec:
  sourceHydrator:
    drySource:
      repoURL: https://github.com/org/repo.git
      targetRevision: HEAD
      path: examples/kustomize
    syncSource:
      targetBranch: hydrated/branch
      path: output
  destination:
    server: https://kubernetes.default.svc
    namespace: demo-app
`,
			want: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  namespace: default
spec:
  source:
    repoURL: https://github.com/org/repo.git
    targetRevision: HEAD
    path: examples/kustomize
  destination:
    server: https://kubernetes.default.svc
    namespace: demo-app
`,
			redirectRevisions: []string{},
			expectErr:         nil,
		},
		{
			name: "application with sourceHydrator and hydrateTo converts to regular application",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  namespace: default
spec:
  sourceHydrator:
    drySource:
      repoURL: https://github.com/org/repo.git
      targetRevision: main
      path: examples/kustomize
    syncSource:
      targetBranch: environments/dev
      path: output
    hydrateTo:
      targetBranch: environments/dev-next
`,
			want: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  namespace: default
spec:
  source:
    repoURL: https://github.com/org/repo.git
    targetRevision: main
    path: examples/kustomize
`,
			redirectRevisions: []string{},
			expectErr:         nil,
		},
		{
			name: "application with sourceHydrator and helm config converts to regular application",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-helm-app
  namespace: argocd
spec:
  sourceHydrator:
    drySource:
      repoURL: https://github.com/argoproj/argocd-example-apps
      path: helm-guestbook
      targetRevision: HEAD
      helm:
        valueFiles:
          - values-prod.yaml
        parameters:
          - name: image.tag
            value: v1.2.3
        releaseName: my-release
    syncSource:
      targetBranch: environments/prod
      path: helm-guestbook-hydrated
  destination:
    server: https://kubernetes.default.svc
    namespace: production
`,
			want: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-helm-app
  namespace: argocd
spec:
  source:
    repoURL: https://github.com/argoproj/argocd-example-apps
    path: helm-guestbook
    targetRevision: HEAD
    helm:
      valueFiles:
        - values-prod.yaml
      parameters:
        - name: image.tag
          value: v1.2.3
      releaseName: my-release
  destination:
    server: https://kubernetes.default.svc
    namespace: production
`,
			redirectRevisions: []string{},
			expectErr:         nil,
		},
		{
			name: "application without sourceHydrator should do nothing",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  namespace: default
spec:
  source:
    repoURL: https://github.com/org/repo.git
    targetRevision: HEAD
    path: examples/kustomize
  destination:
    server: https://kubernetes.default.svc
    namespace: demo-app
`,
			want: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  namespace: default
spec:
  source:
    repoURL: https://github.com/org/repo.git
    targetRevision: HEAD
    path: examples/kustomize
  destination:
    server: https://kubernetes.default.svc
    namespace: demo-app
`,
			redirectRevisions: []string{},
			expectErr:         nil,
		},
		{
			name: "application with sourceHydrator missing targetRevision",
			yaml: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  namespace: default
spec:
  sourceHydrator:
    drySource:
      repoURL: https://github.com/org/repo.git
      path: examples/kustomize
    syncSource:
      targetBranch: hydrated/branch
      path: output
`,
			want: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  namespace: default
spec:
  source:
    repoURL: https://github.com/org/repo.git
    path: examples/kustomize
`,
			redirectRevisions: []string{},
			expectErr:         nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse YAML
			var node map[string]any
			err := yaml.Unmarshal([]byte(tt.yaml), &node)
			assert.NoError(t, err)

			// Create ArgoResource
			app := &ArgoResource{
				Yaml:     &unstructured.Unstructured{Object: node},
				Kind:     Application,
				Id:       "test-app",
				Name:     "test-app",
				FileName: "test-app.yaml",
			}

			// Run redirect source hydrator
			err = app.RedirectSourceHydrator(repo, branch, tt.redirectRevisions)

			// Check result
			if tt.expectErr == nil {
				assert.Nil(t, err)
				got, err := app.AsString()
				assert.Nil(t, err)

				// Normalize both expected and actual YAML for comparison
				expectedNormalized := normalizeYAML(tt.want)
				gotNormalized := normalizeYAML(got)
				assert.Equal(t, expectedNormalized, gotNormalized)
			} else {
				assert.Equal(t, tt.expectErr.Error(), err.Error())
			}
		})
	}
}

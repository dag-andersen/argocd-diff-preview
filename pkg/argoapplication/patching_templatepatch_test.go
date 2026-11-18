package argoapplication

import (
	"testing"

	"github.com/dag-andersen/argocd-diff-preview/pkg/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

func TestRedirectSources_TemplatePatch(t *testing.T) {
	const appSetTemplate = `
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: my-appset
  namespace: argocd
spec:
  goTemplate: true
  generators:
    - clusters: {}
  template:
    metadata:
      name: '{{ .name }}-my-app'
    spec:
      project: default
      destination:
        server: '{{ .server }}'
        namespace: my-app
`

	tests := []struct {
		name              string
		templatePatch     string
		redirectRevisions []string
		want              string
	}{
		{
			name: "redirects sources defined in templatePatch with conditional blocks",
			templatePatch: `spec:
  sources:
    - repoURL: https://github.com/my-org/my-repo.git
      targetRevision: main
      path: charts/my-app
      helm:
        valueFiles:
          - $values/apps/my-app/values.yaml
          - $values/apps/my-app/values-{{ .name }}.yaml
    - repoURL: https://github.com/my-org/my-repo.git
      targetRevision: main
      ref: values
  {{- if eq (index .metadata.labels "env") "prod" }}
    - repoURL: https://github.com/my-org/my-repo.git
      targetRevision: main
      path: charts/my-app-extras
  {{- end }}
    - repoURL: https://charts.example.com
      chart: some-chart
      targetRevision: main
    - repoURL: https://github.com/other-org/other-repo.git
      targetRevision: main
      path: manifests
`,
			want: `spec:
  sources:
    - repoURL: https://github.com/my-org/my-repo.git
      targetRevision: my-feature
      path: charts/my-app
      helm:
        valueFiles:
          - $values/apps/my-app/values.yaml
          - $values/apps/my-app/values-{{ .name }}.yaml
    - repoURL: https://github.com/my-org/my-repo.git
      targetRevision: my-feature
      ref: values
  {{- if eq (index .metadata.labels "env") "prod" }}
    - repoURL: https://github.com/my-org/my-repo.git
      targetRevision: my-feature
      path: charts/my-app-extras
  {{- end }}
    - repoURL: https://charts.example.com
      chart: some-chart
      targetRevision: main
    - repoURL: https://github.com/other-org/other-repo.git
      targetRevision: main
      path: manifests
`,
		},
		{
			name: "redirects single source with targetRevision before repoURL, quotes and comment",
			templatePatch: `spec:
  source:
    targetRevision: "main" # tracked branch
    path: apps/{{ .name }}
    repoURL: 'https://github.com/my-org/my-repo.git'
`,
			want: `spec:
  source:
    targetRevision: "my-feature" # tracked branch
    path: apps/{{ .name }}
    repoURL: 'https://github.com/my-org/my-repo.git'
`,
		},
		{
			name: "keeps targetRevision not listed in redirectRevisions",
			templatePatch: `spec:
  sources:
    - repoURL: https://github.com/my-org/my-repo.git
      targetRevision: v1.0.0
      path: charts/pinned
    - repoURL: https://github.com/my-org/my-repo.git
      targetRevision: main
      path: charts/tracked
`,
			redirectRevisions: []string{"main"},
			want: `spec:
  sources:
    - repoURL: https://github.com/my-org/my-repo.git
      targetRevision: v1.0.0
      path: charts/pinned
    - repoURL: https://github.com/my-org/my-repo.git
      targetRevision: my-feature
      path: charts/tracked
`,
		},
		{
			name: "ignores targetRevision text inside block scalars",
			templatePatch: `spec:
  source:
    repoURL: https://github.com/other-org/other-repo.git
    targetRevision: main
    helm:
      values: |
        repoURL: https://github.com/my-org/my-repo.git
        targetRevision: main
`,
			want: `spec:
  source:
    repoURL: https://github.com/other-org/other-repo.git
    targetRevision: main
    helm:
      values: |
        repoURL: https://github.com/my-org/my-repo.git
        targetRevision: main
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var obj map[string]any
			require.NoError(t, yaml.Unmarshal([]byte(appSetTemplate), &obj))
			require.NoError(t, unstructured.SetNestedField(obj, tt.templatePatch, "spec", "templatePatch"))

			resource := &ArgoResource{
				Yaml:     &unstructured.Unstructured{Object: obj},
				Kind:     ApplicationSet,
				Id:       "test-id",
				Name:     "my-appset",
				FileName: "appset.yaml",
			}

			repoSelector, err := repository.NewSelector("my-org/my-repo", "")
			require.NoError(t, err)

			require.NoError(t, resource.RedirectSources(repoSelector, "my-feature", tt.redirectRevisions))

			got, found, err := unstructured.NestedString(resource.Yaml.Object, "spec", "templatePatch")
			require.NoError(t, err)
			require.True(t, found)
			assert.Equal(t, tt.want, got)
		})
	}
}

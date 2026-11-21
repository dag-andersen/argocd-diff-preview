package argoapplication

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestRedirectSourcesLeavesTemplatePatchUntouched(t *testing.T) {
	const same = "https://github.com/test/repo.git"
	const external = "https://github.com/test/external.git"

	for _, tc := range []struct {
		name  string
		patch string
	}{
		{
			name: "conditional source repositories",
			patch: `spec:
  source:
  {{- if eq .env "prod" }}
    repoURL: ` + same + `
    targetRevision: main
  {{- else }}
    repoURL: ` + external + `
    targetRevision: main
  {{- end }}
    path: manifests
`,
		},
		{
			name: "ignore source-like Helm valuesObject data",
			patch: `spec:
  source:
    repoURL: ` + external + `
    targetRevision: main
    path: manifests
    helm:
      valuesObject:
        repoURL: ` + same + `
        targetRevision: main
`,
		},
		{
			name: "do not partially rewrite a multiline source revision",
			patch: `spec:
  source:
    repoURL: ` + same + `
    targetRevision: >-
      main
    path: manifests
`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			obj := &unstructured.Unstructured{Object: map[string]any{}}
			require.NoError(t, unstructured.SetNestedField(obj.Object, same, "spec", "template", "spec", "source", "repoURL"))
			require.NoError(t, unstructured.SetNestedField(obj.Object, "main", "spec", "template", "spec", "source", "targetRevision"))
			require.NoError(t, unstructured.SetNestedField(obj.Object, tc.patch, "spec", "templatePatch"))
			app := ArgoResource{Kind: ApplicationSet, Yaml: obj}
			require.NoError(t, app.RedirectSources(mustNewTestSelector(t, "test/repo", ""), "feature", nil))
			got, found, err := unstructured.NestedString(obj.Object, "spec", "templatePatch")
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, tc.patch, got)

			revision, _, err := unstructured.NestedString(obj.Object, "spec", "template", "spec", "source", "targetRevision")
			require.NoError(t, err)
			require.Equal(t, "feature", revision)
		})
	}
}

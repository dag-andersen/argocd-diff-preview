package diff

import (
	"strings"
	"testing"

	"github.com/dag-andersen/argocd-diff-preview/pkg/arrayorder"
	"github.com/dag-andersen/argocd-diff-preview/pkg/extract"
	"github.com/dag-andersen/argocd-diff-preview/pkg/git"
	"github.com/dag-andersen/argocd-diff-preview/pkg/matching"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

func deploymentWithEnv(t *testing.T, env string) unstructured.Unstructured {
	t.Helper()
	obj := map[string]any{}
	require.NoError(t, yaml.Unmarshal([]byte(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  namespace: default
spec:
  template:
    spec:
      containers:
      - name: app
        image: nginx
        env:
`+env), &obj))
	return unstructured.Unstructured{Object: obj}
}

func deploymentWithVolumes(name string, volumes []any) unstructured.Unstructured {
	return unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]any{
			"name":      name,
			"namespace": "default",
		},
		"spec": map[string]any{
			"template": map[string]any{
				"spec": map[string]any{
					"containers": []any{map[string]any{"name": "app", "image": "nginx"}},
					"volumes":    volumes,
				},
			},
		},
	}}
}

func envNames(t *testing.T, m unstructured.Unstructured) []string {
	t.Helper()
	containers, _, err := unstructured.NestedSlice(m.Object, "spec", "template", "spec", "containers")
	require.NoError(t, err)
	var names []string
	for _, e := range containers[0].(map[string]any)["env"].([]any) {
		names = append(names, e.(map[string]any)["name"].(string))
	}
	return names
}

func newApps(m unstructured.Unstructured, branch git.BranchType) []extract.ExtractedApp {
	return []extract.ExtractedApp{{Id: "app", Name: "app", SourcePath: "apps/app.yaml", Branch: branch, Manifests: []unstructured.Unstructured{m}}}
}

func TestSortArraysInApps_DoesNotMutateInput(t *testing.T) {
	rules, err := arrayorder.FromString(".spec.template.spec.containers[*].env")
	require.NoError(t, err)

	apps := newApps(deploymentWithEnv(t, `
        - name: VAR_B
        - name: VAR_A
`), git.Base)

	sorted := sortArraysInApps(apps, rules)

	assert.Equal(t, []string{"VAR_A", "VAR_B"}, envNames(t, sorted[0].Manifests[0]))
	assert.Equal(t, []string{"VAR_B", "VAR_A"}, envNames(t, apps[0].Manifests[0]))
}

func TestIgnoreArrayOrder_EndToEndDiff(t *testing.T) {
	rules, err := arrayorder.FromString(`$[?(@.kind == 'Deployment')].spec.template.spec.containers[*].env`)
	require.NoError(t, err)

	base := deploymentWithEnv(t, `
        - name: B
          value: "2"
        - name: A
          value: "1"
`)
	reordered := deploymentWithEnv(t, `
        - name: A
          value: "1"
        - name: B
          value: "2"
`)
	changed := deploymentWithEnv(t, `
        - name: A
          value: "1"
        - name: B
          value: "3"
`)

	// Without rules, a pure reorder produces a diff
	diffs, err := matching.GenerateAppDiffs(newApps(base, git.Base), newApps(reordered, git.Target), 3, nil, nil, nil)
	require.NoError(t, err)
	assert.Len(t, diffs, 1)

	// With rules, a pure reorder produces no diff
	diffs, err = matching.GenerateAppDiffs(
		sortArraysInApps(newApps(base, git.Base), rules),
		sortArraysInApps(newApps(reordered, git.Target), rules),
		3, nil, nil, nil)
	require.NoError(t, err)
	assert.Empty(t, diffs)

	// Real value changes are still reported
	diffs, err = matching.GenerateAppDiffs(
		sortArraysInApps(newApps(base, git.Base), rules),
		sortArraysInApps(newApps(changed, git.Target), rules),
		3, nil, nil, nil)
	require.NoError(t, err)
	require.Len(t, diffs, 1)
	require.Len(t, diffs[0].Resources, 1)
	assert.Contains(t, diffs[0].Resources[0].Content, `-          value: "2"`)
	assert.Contains(t, diffs[0].Resources[0].Content, `+          value: "3"`)
}

func TestIgnoreArrayOrder_ReorderedVolumesInTwoDeployments(t *testing.T) {
	rules, err := arrayorder.FromString(
		".spec.template.spec.volumes,.spec.template.spec.volumes[*].projected.sources",
	)
	require.NoError(t, err)

	diagnostics := map[string]any{"name": "diagnostics", "emptyDir": map[string]any{}}
	roleConfig := map[string]any{
		"name":      "role-config-volume",
		"configMap": map[string]any{"name": "default-role-config-cm"},
	}
	tokens := map[string]any{
		"name": "tokens",
		"projected": map[string]any{
			"defaultMode": int64(420),
			"sources": []any{map[string]any{
				"serviceAccountToken": map[string]any{
					"audience":          "api://AzureADTokenExchange",
					"expirationSeconds": int64(3600),
					"path":              "azure-identity-token",
				},
			}},
		},
	}

	baseApps := []extract.ExtractedApp{{
		Id: "app", Name: "app", SourcePath: "apps/app.yaml", Branch: git.Base,
		Manifests: []unstructured.Unstructured{
			deploymentWithVolumes("app-a", []any{diagnostics, roleConfig, tokens}),
			deploymentWithVolumes("app-b", []any{roleConfig, diagnostics, tokens}),
		},
	}}
	targetApps := []extract.ExtractedApp{{
		Id: "app", Name: "app", SourcePath: "apps/app.yaml", Branch: git.Target,
		Manifests: []unstructured.Unstructured{
			deploymentWithVolumes("app-a", []any{tokens, diagnostics, roleConfig}),
			deploymentWithVolumes("app-b", []any{diagnostics, tokens, roleConfig}),
		},
	}}

	diffs, err := matching.GenerateAppDiffs(
		sortArraysInApps(baseApps, rules),
		sortArraysInApps(targetApps, rules),
		3, nil, nil, nil,
	)
	require.NoError(t, err)
	assert.Empty(t, diffs)
}

func TestIgnoreArrayOrder_DoesNotReportUnchangedVolumesWhenOneIsRenamed(t *testing.T) {
	rules, err := arrayorder.FromString(`$[?(@.kind == 'Deployment')].spec.template.spec.volumes`)
	require.NoError(t, err)

	diagnostics := map[string]any{"name": "diagnostics", "emptyDir": map[string]any{}}
	roleConfig := map[string]any{
		"name":      "role-config-volume",
		"configMap": map[string]any{"name": "default-role-config-cm"},
	}
	projectedToken := func(name string) map[string]any {
		return map[string]any{
			"name": name,
			"projected": map[string]any{
				"defaultMode": int64(420),
				"sources": []any{map[string]any{
					"serviceAccountToken": map[string]any{
						"audience":          "api://AzureADTokenExchange",
						"expirationSeconds": int64(3600),
						"path":              "azure-identity-token",
					},
				}},
			},
		}
	}

	base := deploymentWithVolumes("app", []any{diagnostics, roleConfig, projectedToken("tokens")})
	target := deploymentWithVolumes("app", []any{projectedToken("azure-identity-token"), diagnostics, roleConfig})
	diffs, err := matching.GenerateAppDiffs(
		sortArraysInApps(newApps(base, git.Base), rules),
		sortArraysInApps(newApps(target, git.Target), rules),
		3, nil, nil, rules,
	)
	require.NoError(t, err)
	require.Len(t, diffs, 1)
	require.Len(t, diffs[0].Resources, 1)
	assert.Equal(t, 1, diffs[0].Resources[0].DeletedLines, diffs[0].Resources[0].Content)
	assert.Equal(t, 1, diffs[0].Resources[0].AddedLines, diffs[0].Resources[0].Content)
}

func TestIgnoreArrayOrder_ChangedEnvValueStaysAtSortedPosition(t *testing.T) {
	rules, err := arrayorder.FromString(".spec.template.spec.containers[*].env")
	require.NoError(t, err)

	base := deploymentWithEnv(t, `
        - name: LOG_LEVEL
          value: info
        - name: DB_PORT
          value: "5432"
        - name: DB_HOST
          value: postgres
`)
	target := deploymentWithEnv(t, `
        - name: DB_HOST
          value: postgres
        - name: LOG_LEVEL
          value: info
        - name: DB_PORT
          value: "5433"
`)

	diffs, err := matching.GenerateAppDiffs(
		sortArraysInApps(newApps(base, git.Base), rules),
		sortArraysInApps(newApps(target, git.Target), rules),
		10, nil, nil, rules,
	)
	require.NoError(t, err)
	require.Len(t, diffs, 1)
	require.Len(t, diffs[0].Resources, 1)

	content := diffs[0].Resources[0].Content
	assert.Equal(t, 1, diffs[0].Resources[0].AddedLines, content)
	assert.Equal(t, 1, diffs[0].Resources[0].DeletedLines, content)
	assert.Contains(t, content, "-          value: \"5432\"")
	assert.Contains(t, content, "+          value: \"5433\"")
	assert.Less(t, strings.Index(content, "name: DB_HOST"), strings.Index(content, "name: DB_PORT"), content)
	assert.Less(t, strings.Index(content, "name: DB_PORT"), strings.Index(content, "name: LOG_LEVEL"), content)
}

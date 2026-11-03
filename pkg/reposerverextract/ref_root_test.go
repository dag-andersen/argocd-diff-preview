package reposerverextract

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A source's path selects manifests to render; $values still starts at the
// repository root, including when the source provides both manifests and values.
func TestBuildManifestRequest_RefWithPathUsesRepositoryRoot(t *testing.T) {
	for _, chartType := range []string{"local", "remote"} {
		for _, fixture := range []string{"root-only", "shadowed", "optional"} {
			t.Run(chartType+"/"+fixture, func(t *testing.T) {
				branchFolder := makeBranchFolder(t, "apps/chart")
				const contentPath = "components/exporter/instances/prod"
				const valuePath = "base/tenants/tenants.yaml"
				const expectedValues = "replicas: 3\n"
				files := map[string]string{
					valuePath: expectedValues,
					filepath.Join(contentPath, "kustomization.yaml"): "resources: []\n",
					"unrelated.yaml": "ignored: true\n",
				}
				if fixture == "shadowed" {
					files[filepath.Join(contentPath, valuePath)] = "replicas: 99\n"
				}
				for path, content := range files {
					fullPath := filepath.Join(branchFolder, path)
					require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0o755))
					require.NoError(t, os.WriteFile(fullPath, []byte(content), 0o644))
				}

				app := makeApp(t, `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: exporter
spec:
  destination:
    namespace: exporter
  sources:
    - repoURL: https://github.com/org/repo.git
      targetRevision: HEAD
      path: components/exporter/instances/prod
      ref: values
    - repoURL: https://charts.example.com
      chart: namespace
      targetRevision: 1.0.0
      helm:
        valueFiles:
          - $values/base/tenants/tenants.yaml
        fileParameters:
          - name: tenants
            path: $values/base/tenants/tenants.yaml
`)
				contentSources, refSources, multiple, err := splitSources(app)
				require.NoError(t, err)
				require.Len(t, contentSources, 2)
				require.Len(t, refSources, 1)
				context := manifestRequestRenderContext{
					repoSelector: testRepoSelector(t, "org/repo"),
					puller:       &fakeChartPuller{},
				}

				req, streamDir, cleanup, err := buildManifestRequestForSource(app, contentSources[0], refSources, multiple, branchFolder, nil, context)
				if cleanup != nil {
					t.Cleanup(cleanup)
				}
				require.NoError(t, err)
				assert.Equal(t, contentPath, req.ApplicationSource.Path)
				assert.Equal(t, branchFolder, streamDir, "the ref source must still render its Kustomize content")

				primary := contentSources[1]
				if chartType == "local" {
					primary.Chart = ""
					primary.Path = "apps/chart"
					primary.RepoURL = refSources[0].RepoURL
				}
				primary.Helm.IgnoreMissingValueFiles = fixture == "optional"
				if fixture == "optional" {
					primary.Helm.FileParameters = nil
				}
				req, streamDir, cleanup, err = buildManifestRequestForSource(app, primary, refSources, multiple, branchFolder, nil, context)
				if cleanup != nil {
					t.Cleanup(cleanup)
				}
				require.NoError(t, err)
				require.NotEmpty(t, streamDir, "existing repository-root values must use local streaming")
				assert.Nil(t, req.RefSources)
				paths := append([]string(nil), req.ApplicationSource.Helm.ValueFiles...)
				for _, parameter := range req.ApplicationSource.Helm.FileParameters {
					paths = append(paths, parameter.Path)
				}
				for _, path := range paths {
					content, err := os.ReadFile(filepath.Join(streamDir, req.ApplicationSource.Path, path))
					require.NoError(t, err)
					assert.Equal(t, expectedValues, string(content), "use repository-root values even if a different file exists under source.path")
				}
				assert.NoFileExists(t, filepath.Join(streamDir, ".refs", "values", "unrelated.yaml"))
			})
		}
	}
}

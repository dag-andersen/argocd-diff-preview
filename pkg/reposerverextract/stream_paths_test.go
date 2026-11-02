package reposerverextract

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/stretchr/testify/require"
)

func TestLocalStreamPaths(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "apps/prod"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "apps/prod/kustomization.yaml"), []byte("resources: []\n"), 0o644))
	app := makeApp(t, `apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: example
  annotations:
    argocd.argoproj.io/manifest-generate-paths: ../shared;/base;../../config/*.yaml
`)
	source := v1alpha1.ApplicationSource{Path: "apps/prod"}
	require.Equal(t, []string{"apps/shared", "base", "config/*.yaml", "apps/prod"}, localStreamPaths(app, source, root, root))
	require.Nil(t, localStreamPaths(app, source, root, filepath.Join(root, "staged")))
	require.Nil(t, localStreamPaths(app, v1alpha1.ApplicationSource{Path: "raw"}, root, root))
	app.Yaml.SetAnnotations(nil)
	require.Nil(t, localStreamPaths(app, source, root, root))
}

func TestLocalStreamPathsRespectsExplicitSourceType(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "app"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "app/kustomization.yaml"), []byte("resources: []\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "app/Chart.yaml"), []byte("apiVersion: v2\nname: example\nversion: 1.0.0\n"), 0o644))
	app := makeApp(t, `apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: example
  annotations:
    argocd.argoproj.io/manifest-generate-paths: .
`)
	for _, tc := range []struct {
		name   string
		source v1alpha1.ApplicationSource
		narrow bool
	}{
		{name: "discovered Kustomize", narrow: true},
		{name: "explicit Kustomize", source: v1alpha1.ApplicationSource{Kustomize: &v1alpha1.ApplicationSourceKustomize{}}, narrow: true},
		{name: "Helm", source: v1alpha1.ApplicationSource{Helm: &v1alpha1.ApplicationSourceHelm{ValueFiles: []string{"../config/values.yaml"}, IgnoreMissingValueFiles: true}}},
		{name: "plugin", source: v1alpha1.ApplicationSource{Plugin: &v1alpha1.ApplicationSourcePlugin{Name: "example"}}},
		{name: "directory", source: v1alpha1.ApplicationSource{Directory: &v1alpha1.ApplicationSourceDirectory{}}},
		{name: "conflicting types", source: v1alpha1.ApplicationSource{Kustomize: &v1alpha1.ApplicationSourceKustomize{}, Helm: &v1alpha1.ApplicationSourceHelm{}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.source.Path = "app"
			paths := localStreamPaths(app, tc.source, root, root)
			if tc.narrow {
				require.Equal(t, []string{"app", "app"}, paths)
			} else {
				require.Nil(t, paths)
			}
		})
	}
}

func TestLocalStreamPathsKeepsCheckoutForSourceOverrides(t *testing.T) {
	for _, name := range []string{".argocd-source.yaml", ".argocd-source-example.yaml", ".argocd-source-other.yaml"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			require.NoError(t, os.MkdirAll(filepath.Join(root, "app"), 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(root, "app/kustomization.yaml"), []byte("resources: []\n"), 0o644))
			require.NoError(t, os.WriteFile(filepath.Join(root, "app", name), []byte("directory: {}\n"), 0o644))
			app := makeApp(t, `apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: example
  annotations:
    argocd.argoproj.io/manifest-generate-paths: .
`)
			paths := localStreamPaths(app, v1alpha1.ApplicationSource{Path: "app"}, root, root)
			if name == ".argocd-source-other.yaml" {
				require.NotEmpty(t, paths)
			} else {
				require.Nil(t, paths)
			}
		})
	}
}

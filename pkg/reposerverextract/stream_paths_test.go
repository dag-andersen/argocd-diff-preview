package reposerverextract

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/dag-andersen/argocd-diff-preview/pkg/argoapplication"
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
	source := v1alpha1.ApplicationSource{Path: "apps/prod", Kustomize: &v1alpha1.ApplicationSourceKustomize{}}
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
		{name: "automatic discovery may select a plugin"},
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
			paths := localStreamPaths(app, v1alpha1.ApplicationSource{Path: "app", Kustomize: &v1alpha1.ApplicationSourceKustomize{}}, root, root)
			if name == ".argocd-source-other.yaml" {
				require.NotEmpty(t, paths)
			} else {
				require.Nil(t, paths)
			}
		})
	}
}

func TestLocalStreamPathsUsesSameAnnotationAsSelection(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "app"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "app/kustomization.yaml"), []byte("resources:\n- ../shared\n"), 0o644))
	app := makeApp(t, `apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: example
  annotations:
    argocd.argoproj.io/manifest-generate-paths: |
      .;/shared
spec:
  source:
    path: app
    kustomize: {}
`)
	source := v1alpha1.ApplicationSource{Path: "app", Kustomize: &v1alpha1.ApplicationSourceKustomize{}}
	for _, annotation := range []string{
		app.Yaml.GetAnnotations()[v1alpha1.AnnotationKeyManifestGeneratePaths],
		" \t.;/shared \n",
		" \t\n",
	} {
		t.Run(annotation, func(t *testing.T) {
			app.Yaml.SetAnnotations(map[string]string{v1alpha1.AnnotationKeyManifestGeneratePaths: annotation})
			require.True(t, app.Filter(argoapplication.ApplicationSelectionOptions{
				FilesChanged: []string{"shared/kustomization.yaml"}, WatchIfNoWatchPatternFound: true,
			}))
			paths := localStreamPaths(app, source, root, root)
			if annotation == " \t\n" {
				require.Nil(t, paths, "blank annotations must retain the full checkout")
			} else {
				require.Equal(t, []string{"app", "shared", "app"}, paths)
			}
			require.Equal(t, annotation, app.Yaml.GetAnnotations()[v1alpha1.AnnotationKeyManifestGeneratePaths], "reading stream paths must not change the Application")
		})
	}
}

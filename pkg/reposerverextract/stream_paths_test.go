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

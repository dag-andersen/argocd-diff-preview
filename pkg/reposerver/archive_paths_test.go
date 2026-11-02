package reposerver

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/argoproj/argo-cd/v3/util/io/files"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"
)

func writeArchiveFixture(t *testing.T, root string, contents map[string]string) {
	t.Helper()
	for path, content := range contents {
		fullPath := filepath.Join(root, path)
		require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0o755))
		require.NoError(t, os.WriteFile(fullPath, []byte(content), 0o644))
	}
}

func extractArchive(t *testing.T, cache *tgzCache, root string, paths ...string) string {
	t.Helper()
	archive, err := cache.openCachedTgz(root, paths...)
	require.NoError(t, err)
	defer func() { require.NoError(t, archive.file.Close()) }()
	dest := t.TempDir()
	require.NoError(t, files.Untgz(dest, archive.file, 100<<20, true))
	return dest
}

func TestScopedArchivePreservesKustomizeCrossDirectoryDependencies(t *testing.T) {
	root := t.TempDir()
	writeArchiveFixture(t, root, map[string]string{
		"apps/prod/kustomization.yaml":    "resources:\n- ../../base\nconfigMapGenerator:\n- name: settings\n  envs:\n  - ../../config/prod.env\n",
		"base/kustomization.yaml":         "resources:\n- deployment.yaml\n",
		"base/deployment.yaml":            "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: example\nspec:\n  replicas: 2\n",
		"config/prod.env":                 "ENV=prod\n",
		"hydrated/cluster/unrelated.yaml": "large generated manifest\n",
		"docs[old]/README.md":             "unrelated documentation\n",
		".git/config":                     "must never be streamed\n",
	})
	cache := newTgzCache()
	t.Cleanup(cache.Cleanup)
	full := extractArchive(t, cache, root)
	narrow := extractArchive(t, cache, root, "apps/prod", "base", "config/prod.env")
	for _, path := range []string{"hydrated", "docs[old]", ".git"} {
		_, err := os.Stat(filepath.Join(narrow, path))
		require.True(t, os.IsNotExist(err), "unexpected archived path: %s", path)
	}
	render := func(root string) []byte {
		options := krusty.MakeDefaultOptions()
		options.LoadRestrictions = types.LoadRestrictionsNone
		result, err := krusty.MakeKustomizer(options).Run(filesys.MakeFsOnDisk(), filepath.Join(root, "apps/prod"))
		require.NoError(t, err)
		output, err := result.AsYaml()
		require.NoError(t, err)
		return output
	}
	require.Equal(t, render(full), render(narrow))
}

func TestScopedArchiveCacheSeparatesPathSets(t *testing.T) {
	root := t.TempDir()
	writeArchiveFixture(t, root, map[string]string{"a/one": "a", "b/two": "b", "common/file": "shared"})
	cache := newTgzCache()
	t.Cleanup(cache.Cleanup)
	a := extractArchive(t, cache, root, "a", "common")
	b := extractArchive(t, cache, root, "b", "common")
	extractArchive(t, cache, root, "common", "./a", "a")
	require.FileExists(t, filepath.Join(a, "a/one"))
	require.NoFileExists(t, filepath.Join(a, "b/two"))
	require.FileExists(t, filepath.Join(b, "b/two"))
	require.NoFileExists(t, filepath.Join(b, "a/one"))
	require.EqualValues(t, 2, cache.stats().Misses)
	require.EqualValues(t, 1, cache.stats().Hits)
}

func TestScopedArchivePaths(t *testing.T) {
	root := filepath.Join(t.TempDir(), "checkout[1]")
	writeArchiveFixture(t, root, map[string]string{"apps/v[1]/app": "app", "config/dev.yaml": "dev", "config/prod.yaml": "prod", "unrelated/file": "other"})
	cache := newTgzCache()
	t.Cleanup(cache.Cleanup)
	dest := extractArchive(t, cache, root, "apps/v[1]", "config/*.yaml", "optional-missing")
	for _, path := range []string{"apps/v[1]/app", "config/dev.yaml", "config/prod.yaml"} {
		require.FileExists(t, filepath.Join(dest, path))
	}
	require.NoDirExists(t, filepath.Join(dest, "unrelated"))
	for _, path := range []string{"../outside", "/outside", "[invalid"} {
		_, err := cache.openCachedTgz(root, path)
		require.Error(t, err, "path: %s", path)
	}
	dest = extractArchive(t, cache, root, ".")
	require.FileExists(t, filepath.Join(dest, "unrelated/file"))
}

func TestScopedArchiveRetainedSymlinkKeepsItsTarget(t *testing.T) {
	root := t.TempDir()
	writeArchiveFixture(t, root, map[string]string{"shared/value": "target", ".git/config": "excluded"})
	require.NoError(t, os.Mkdir(filepath.Join(root, "app"), 0o755))
	require.NoError(t, os.Symlink("../shared/value", filepath.Join(root, "app/value")))
	cache := newTgzCache()
	t.Cleanup(cache.Cleanup)
	dest := extractArchive(t, cache, root, "app")
	content, err := os.ReadFile(filepath.Join(dest, "app/value"))
	require.NoError(t, err)
	require.Equal(t, "target", string(content))
	require.NoDirExists(t, filepath.Join(dest, ".git"))
}

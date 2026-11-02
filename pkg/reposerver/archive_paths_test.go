package reposerver

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	argocdpath "github.com/argoproj/argo-cd/v3/util/app/path"
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
	writeArchiveFixture(t, root, map[string]string{"apps/v[1]/app": "app", "config/dev.yaml": "dev", "config/prod.yaml": "prod", "config/[literal": "literal", "unrelated/file": "other"})
	cache := newTgzCache()
	t.Cleanup(cache.Cleanup)
	dest := extractArchive(t, cache, root, "apps/v[1]", "config/*.yaml", "config/[literal", "optional-missing")
	for _, path := range []string{"apps/v[1]/app", "config/dev.yaml", "config/prod.yaml", "config/[literal"} {
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

func TestScopedArchiveIncludesLiteralAndGlobMatches(t *testing.T) {
	root := t.TempDir()
	writeArchiveFixture(t, root, map[string]string{
		"app/kustomization.yaml": "resources:\n- ../config/v1.yaml\n- ../config/v[1].yaml\n",
		"config/v1.yaml":         "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: matched\n",
		"config/v[1].yaml":       "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: literal\n",
		"unrelated/file":         "excluded\n",
	})
	paths := []string{"app", "config/v[1].yaml"}
	for _, path := range []string{"config/v1.yaml", "config/v[1].yaml"} {
		require.True(t, argocdpath.AppFilesHaveChanged(paths, []string{path}))
	}
	cache := newTgzCache()
	t.Cleanup(cache.Cleanup)
	full := extractArchive(t, cache, root)
	narrow := extractArchive(t, cache, root, paths...)
	require.NoDirExists(t, filepath.Join(narrow, "unrelated"))
	render := func(dir string) []byte {
		options := krusty.MakeDefaultOptions()
		options.LoadRestrictions = types.LoadRestrictionsNone
		result, err := krusty.MakeKustomizer(options).Run(filesys.MakeFsOnDisk(), filepath.Join(dir, "app"))
		require.NoError(t, err)
		output, err := result.AsYaml()
		require.NoError(t, err)
		return output
	}
	require.Equal(t, render(full), render(narrow))
}

func TestScopedArchiveFullCheckoutSharesCacheAcrossConcurrentScopes(t *testing.T) {
	for _, mode := range []string{"root", "symlink"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			writeArchiveFixture(t, root, map[string]string{"shared/value": "data"})
			// Include an unannotated request in the same group as the fallbacks.
			scopes := [][]string{nil}
			for i := range 12 {
				appPath := fmt.Sprintf("apps/app-%02d", i)
				writeArchiveFixture(t, root, map[string]string{appPath + "/kustomization.yaml": "resources: []\n"})
				paths := []string{appPath}
				if mode == "symlink" {
					require.NoError(t, os.Symlink("../../shared/value", filepath.Join(root, appPath, "value")))
				} else {
					paths = append(paths, ".")
				}
				scopes = append(scopes, paths)
			}
			var compressions atomic.Int32
			cache := newTgzCacheWithCompressor(func(dir string, excluded []string) (*os.File, int, string, error) {
				compressions.Add(1)
				return compressArchive(dir, excluded)
			})
			t.Cleanup(cache.Cleanup)
			type result struct {
				archive *tgzArchive
				err     error
			}
			results := make(chan result, len(scopes))
			start := make(chan struct{})
			var wg sync.WaitGroup
			for _, paths := range scopes {
				wg.Go(func() {
					<-start
					archive, err := cache.openCachedTgz(root, paths...)
					results <- result{archive, err}
				})
			}
			close(start)
			wg.Wait()
			close(results)
			var archivePath string
			for result := range results {
				require.NoError(t, result.err)
				require.NoError(t, result.archive.file.Close())
				if archivePath == "" {
					archivePath = result.archive.file.Name()
				}
				require.Equal(t, archivePath, result.archive.file.Name())
			}
			require.EqualValues(t, 1, compressions.Load())
			require.EqualValues(t, 1, cache.stats().Misses)
			require.EqualValues(t, len(scopes)-1, cache.stats().Hits)
		})
	}
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

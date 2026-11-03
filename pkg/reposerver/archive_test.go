package reposerver

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/argoproj/argo-cd/v3/util/tgzstream"
	"github.com/stretchr/testify/require"
)

// Reproduce the previous compressor, which received escaped glob patterns.
func compressArchiveWithGlobs(directory string, excluded []string) (*os.File, int, string, error) {
	escape := strings.NewReplacer("\\", "\\\\", "*", "\\*", "?", "\\?", "[", "\\[")
	globs := make([]string, len(excluded))
	for i, path := range excluded {
		globs[i] = escape.Replace(path)
	}
	return tgzstream.CompressFiles(directory, []string{"*"}, globs)
}

func TestCompressArchiveMatchesArgoCD(t *testing.T) {
	root := filepath.Join(t.TempDir(), "checkout[1]")
	writeArchiveFixture(t, root, map[string]string{
		"app/kustomization.yaml":          "resources:\n- ../base\n",
		"base/kustomization.yaml":         "resources:\n- deployment.yaml\n",
		"base/deployment.yaml":            "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: example\n",
		"config/prod.env":                 "ENV=prod\n",
		"config/v[1].yaml":                "literal",
		"config/v1.yaml":                  "glob match",
		"scripts/run.sh":                  "#!/bin/sh\necho example\n",
		"app/.config":                     "hidden file",
		"app/empty":                       "",
		"app/" + strings.Repeat("a", 110): "long filename",
		"docs[old]/README.md":             "excluded",
		"docs1/README.md":                 "retained by literal selection",
		"docs*odd/name?":                  "excluded",
		"back\\slash/README.md":           "excluded",
		".git/config":                     "excluded",
	})
	require.NoError(t, os.Mkdir(filepath.Join(root, "empty-directory"), 0o750))
	require.NoError(t, os.Chmod(filepath.Join(root, "scripts/run.sh"), 0o750))
	require.NoError(t, os.Mkdir(filepath.Join(root, "links"), 0o755))
	require.NoError(t, os.Symlink("../config/prod.env", filepath.Join(root, "links/value")))
	for _, tc := range []struct {
		name  string
		paths []string
	}{
		{"shared dependencies", []string{"app", "base", "config/prod.env"}},
		{"literal and glob matches", []string{"config/v[1].yaml", "docs1"}},
		{"modes and empty directories", []string{"scripts", "empty-directory"}},
		{"full checkout", nil},
		{"root selection", []string{"."}},
		{"retained symlink", []string{"links"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			excluded, err := archiveExclusions(root, tc.paths)
			require.NoError(t, err)
			old, oldCount, oldHash, err := compressArchiveWithGlobs(root, excluded)
			require.NoError(t, err)
			defer tgzstream.CloseAndDelete(old)
			archive, count, hash, err := compressArchive(root, excluded)
			require.NoError(t, err)
			defer tgzstream.CloseAndDelete(archive)
			oldBytes, err := io.ReadAll(old)
			require.NoError(t, err)
			newBytes, err := io.ReadAll(archive)
			require.NoError(t, err)
			require.Equal(t, oldCount, count)
			require.Equal(t, oldHash, hash)
			require.Equal(t, oldBytes, newBytes, "headers, file contents, and ordering must match Argo CD")
		})
	}
}

func TestCompressArchiveRemovesPartialArchiveOnError(t *testing.T) {
	archiveDir := t.TempDir()
	t.Setenv("TMPDIR", archiveDir)
	file, _, _, err := compressArchive(filepath.Join(archiveDir, "missing"), []string{".git", "excluded"})
	require.Error(t, err)
	require.Nil(t, file)
	entries, err := os.ReadDir(archiveDir)
	require.NoError(t, err)
	require.Empty(t, entries, "a failed archive must not leave a temporary file")
}

type archiveFailingWriter struct {
	err error
}

func (w archiveFailingWriter) Write([]byte) (int, error) { return 0, w.err }

func TestWriteScopedTarReturnsWriteErrors(t *testing.T) {
	root := t.TempDir()
	writeArchiveFixture(t, root, map[string]string{"app/file": "content"})
	writeErr := errors.New("disk full")
	_, err := writeScopedTar(archiveFailingWriter{err: writeErr}, root, []string{".git"})
	require.ErrorIs(t, err, writeErr)
}

// Includes selection and compression on every iteration, without archive-cache
// hits. Run with -benchtime=1x to compare large exclusion sets quickly.
func BenchmarkScopedArchiveManySiblings(b *testing.B) {
	for _, size := range []int{1000, 4000, 16000} {
		b.Run(fmt.Sprint(size), func(b *testing.B) {
			root := b.TempDir()
			for i := range size {
				dir := filepath.Join(root, fmt.Sprintf("apps/app-%05d", i))
				require.NoError(b, os.MkdirAll(dir, 0o755))
				require.NoError(b, os.WriteFile(filepath.Join(dir, "kustomization.yaml"), []byte("resources: []\n"), 0o644))
			}
			for _, method := range []struct {
				name     string
				compress tgzCompressor
			}{
				{"globs", compressArchiveWithGlobs},
				{"literal", compressArchive},
			} {
				b.Run(method.name, func(b *testing.B) {
					b.ReportAllocs()
					for b.Loop() {
						excluded, err := archiveExclusions(root, []string{"apps/app-00000"})
						require.NoError(b, err)
						file, count, _, err := method.compress(root, excluded)
						require.NoError(b, err)
						require.Equal(b, 1, count)
						tgzstream.CloseAndDelete(file)
					}
				})
			}
		})
	}
}

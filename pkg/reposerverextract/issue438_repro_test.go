package reposerverextract

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/argoproj/argo-cd/v3/util/tgzstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIssue438Repro_StreamedTarballContainsOnlyLocalHelmChart(t *testing.T) {
	baseFolder := createIssue438BranchFolder(t, "base")
	targetFolder := createIssue438BranchFolder(t, "target")

	for _, tc := range []struct {
		name         string
		branchFolder string
	}{
		{name: "base", branchFolder: baseFolder},
		{name: "target", branchFolder: targetFolder},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := makeApp(t, `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: argocd
spec:
  destination:
    namespace: argocd
  source:
    repoURL: https://github.com/org/repo.git
    path: infra/charts/argocd
    targetRevision: HEAD
`)

			contentSources, refSources, hasMultipleSources, err := splitSources(app)
			require.NoError(t, err)
			require.Len(t, contentSources, 1)

			req, streamDir, cleanup, err := buildManifestRequestForSource(app, contentSources[0], refSources, hasMultipleSources, tc.branchFolder, nil,
				manifestRequestRenderContext{repoSelector: testRepoSelector(t, "https://github.com/org/repo.git")})
			require.NoError(t, err)
			if cleanup != nil {
				defer cleanup()
			}

			assert.Equal(t, filepath.Join(tc.branchFolder, "infra", "charts", "argocd"), streamDir)
			assert.Empty(t, req.ApplicationSource.Path)

			tarEntries := compressAndListEntries(t, streamDir)
			assert.Contains(t, tarEntries, "Chart.yaml")
			assert.Contains(t, tarEntries, "values.yaml")
			assert.NotContains(t, tarEntries, "src/apps/web/public/avatars/avatar.png")
			assert.NotContains(t, tarEntries, "assets/co/avatar.png")
		})
	}
}

func createIssue438BranchFolder(t *testing.T, name string) string {
	t.Helper()
	branchFolder := filepath.Join(t.TempDir(), name)
	chartDir := filepath.Join(branchFolder, "infra", "charts", "argocd")
	assetDir := filepath.Join(branchFolder, "assets", "co")
	avatarDir := filepath.Join(branchFolder, "src", "apps", "web", "public", "avatars")
	require.NoError(t, os.MkdirAll(chartDir, 0o755))
	require.NoError(t, os.MkdirAll(assetDir, 0o755))
	require.NoError(t, os.MkdirAll(avatarDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte("apiVersion: v2\nname: argocd\nversion: 0.1.0\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(chartDir, "values.yaml"), []byte("replicas: 1\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(assetDir, "avatar.png"), []byte("png"), 0o644))
	require.NoError(t, os.Symlink(filepath.Join("..", "..", "..", "..", "..", "assets", "co", "avatar.png"), filepath.Join(avatarDir, "avatar.png")))
	return branchFolder
}

func compressAndListEntries(t *testing.T, dir string) []string {
	t.Helper()
	tgzFile, _, _, err := tgzstream.CompressFiles(dir, []string{"*"}, []string{".git"})
	require.NoError(t, err)
	defer tgzstream.CloseAndDelete(tgzFile)

	_, err = tgzFile.Seek(0, io.SeekStart)
	require.NoError(t, err)
	gzipReader, err := gzip.NewReader(tgzFile)
	require.NoError(t, err)
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	var entries []string
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		entries = append(entries, header.Name)
	}
	sort.Strings(entries)
	return entries
}

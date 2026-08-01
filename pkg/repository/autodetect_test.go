package repository

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectMatchingOriginRepo(t *testing.T) {
	dir := t.TempDir()
	baseFolder := filepath.Join(dir, "base-branch")
	targetFolder := filepath.Join(dir, "target-branch")
	writeGitConfig(t, baseFolder, "https://github.com/example/resource-config.git")
	writeGitConfig(t, targetFolder, "git@github.com:example/resource-config.git")

	repo, ok := DetectMatchingOriginRepo(baseFolder, targetFolder)

	require.True(t, ok)
	assert.Equal(t, "example/resource-config", repo)
}

func TestDetectMatchingOriginRepoReturnsFalseWhenFoldersDiffer(t *testing.T) {
	dir := t.TempDir()
	baseFolder := filepath.Join(dir, "base-branch")
	targetFolder := filepath.Join(dir, "target-branch")
	writeGitConfig(t, baseFolder, "https://github.com/example/resource-config.git")
	writeGitConfig(t, targetFolder, "https://github.com/example/application-config.git")

	repo, ok := DetectMatchingOriginRepo(baseFolder, targetFolder)

	assert.False(t, ok)
	assert.Empty(t, repo)
}

func TestRepoPathFromRemoteURL(t *testing.T) {
	tests := []struct {
		name      string
		remoteURL string
		expected  string
	}{
		{name: "https", remoteURL: "https://github.com/example/resource-config.git", expected: "example/resource-config"},
		{name: "ssh URL", remoteURL: "ssh://git@github.com/example/resource-config.git", expected: "example/resource-config"},
		{name: "scp style ssh", remoteURL: "git@github.com:example/resource-config.git", expected: "example/resource-config"},
		{name: "without git suffix", remoteURL: "https://github.com/example/resource-config", expected: "example/resource-config"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, ok := repoPathFromRemoteURL(tt.remoteURL)

			require.True(t, ok)
			assert.Equal(t, tt.expected, repo)
		})
	}
}

func writeGitConfig(t *testing.T, folder, remoteURL string) {
	t.Helper()
	gitDir := filepath.Join(folder, ".git")
	require.NoError(t, os.MkdirAll(gitDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(gitDir, "config"), []byte(`[remote "origin"]
	url = `+remoteURL+`
`), 0o644))
}

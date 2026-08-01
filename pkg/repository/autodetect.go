package repository

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// DetectMatchingOriginRepo reads Git metadata from two checkout folders and
// returns owner/repo when both checkouts have the same origin remote.
func DetectMatchingOriginRepo(baseFolder, targetFolder string) (string, bool) {
	baseRepo, err := repoFromGitFolder(baseFolder)
	if err != nil {
		return "", false
	}

	targetRepo, err := repoFromGitFolder(targetFolder)
	if err != nil {
		return "", false
	}

	if baseRepo == "" || targetRepo == "" || baseRepo != targetRepo {
		return "", false
	}

	return baseRepo, true
}

func repoFromGitFolder(folder string) (string, error) {
	gitDir, err := resolveGitDir(folder)
	if err != nil {
		return "", err
	}

	remoteURL, err := originRemoteURL(gitDir)
	if err != nil {
		return "", err
	}

	repo, ok := repoPathFromRemoteURL(remoteURL)
	if !ok {
		return "", fmt.Errorf("could not parse repository from remote URL")
	}

	return repo, nil
}

func resolveGitDir(folder string) (string, error) {
	gitPath := filepath.Join(folder, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		return "", err
	}

	if info.IsDir() {
		return gitPath, nil
	}

	content, err := os.ReadFile(gitPath)
	if err != nil {
		return "", err
	}

	gitDir, ok := strings.CutPrefix(strings.TrimSpace(string(content)), "gitdir:")
	if !ok {
		return "", fmt.Errorf("unsupported .git file format")
	}

	gitDir = strings.TrimSpace(gitDir)
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(folder, gitDir)
	}

	return filepath.Clean(gitDir), nil
}

func originRemoteURL(gitDir string) (string, error) {
	for _, configPath := range gitConfigPaths(gitDir) {
		remoteURL, err := originRemoteURLFromConfig(configPath)
		if err == nil && remoteURL != "" {
			return remoteURL, nil
		}
	}

	return "", fmt.Errorf("origin remote URL not found")
}

func gitConfigPaths(gitDir string) []string {
	paths := []string{filepath.Join(gitDir, "config")}

	commonDirPath := filepath.Join(gitDir, "commondir")
	content, err := os.ReadFile(commonDirPath)
	if err != nil {
		return paths
	}

	commonDir := strings.TrimSpace(string(content))
	if commonDir == "" {
		return paths
	}
	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Join(gitDir, commonDir)
	}

	paths = append(paths, filepath.Join(filepath.Clean(commonDir), "config"))
	return paths
}

func originRemoteURLFromConfig(configPath string) (string, error) {
	content, err := os.ReadFile(configPath)
	if err != nil {
		return "", err
	}

	inOriginRemote := false
	for line := range strings.SplitSeq(string(content), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			inOriginRemote = line == `[remote "origin"]`
			continue
		}
		if !inOriginRemote {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(key) != "url" {
			continue
		}
		return strings.TrimSpace(value), nil
	}

	return "", fmt.Errorf("origin remote URL not found in config")
}

func repoPathFromRemoteURL(remoteURL string) (string, bool) {
	remoteURL = strings.TrimSpace(remoteURL)
	if remoteURL == "" {
		return "", false
	}

	if parsed, err := url.Parse(remoteURL); err == nil && parsed.Scheme != "" {
		return repoPathFromPath(parsed.Path)
	}

	if before, after, ok := strings.Cut(remoteURL, ":"); ok && strings.Contains(before, "@") {
		return repoPathFromPath(after)
	}

	return repoPathFromPath(remoteURL)
}

func repoPathFromPath(path string) (string, bool) {
	path = strings.TrimSpace(path)
	path = strings.Trim(path, "/")
	path = strings.TrimSuffix(path, ".git")
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return "", false
	}

	owner := parts[len(parts)-2]
	repo := parts[len(parts)-1]
	if owner == "" || repo == "" {
		return "", false
	}

	return owner + "/" + repo, true
}

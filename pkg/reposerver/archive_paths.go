package reposerver

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// The Argo CD compressor matches inclusions against basenames, so use directory
// exclusions to retain repository-relative paths and prune unrelated subtrees.
func archiveExclusions(directory string, paths []string) ([]string, error) {
	excluded := []string{".git"}
	if len(paths) == 0 {
		return excluded, nil
	}
	var roots []string
	for _, path := range paths {
		path = filepath.Clean(path)
		if !filepath.IsLocal(path) {
			return nil, fmt.Errorf("stream path %q must stay within the checkout", path)
		}
		if path == "." {
			return excluded, nil
		}
		fullPath := filepath.Join(directory, path)
		if _, err := os.Lstat(fullPath); err == nil {
			roots = append(roots, path)
			continue
		} else if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		matches, err := fs.Glob(os.DirFS(directory), filepath.ToSlash(path))
		if err != nil {
			return nil, fmt.Errorf("invalid stream path %q: %w", path, err)
		}
		for _, match := range matches {
			roots = append(roots, filepath.FromSlash(match))
		}
	}
	// A retained symlink can point into an otherwise excluded subtree. Use the
	// full checkout in that case, preserving the repo server's symlink checks.
	symlink := errors.New("retained symlink needs the full checkout")
	escapeGlob := strings.NewReplacer("\\", "\\\\", "*", "\\*", "?", "\\?", "[", "\\[")
	err := filepath.WalkDir(directory, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(directory, path)
		if err != nil || rel == "." {
			return err
		}
		if rel == ".git" {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		for _, root := range roots {
			if rel == root || strings.HasPrefix(rel, root+string(filepath.Separator)) || strings.HasPrefix(root, rel+string(filepath.Separator)) {
				if entry.Type()&fs.ModeSymlink != 0 {
					return symlink
				}
				return nil
			}
		}
		// Exclusions are glob patterns, while rel is a literal filename.
		excluded = append(excluded, escapeGlob.Replace(rel))
		if entry.IsDir() {
			return filepath.SkipDir
		}
		return nil
	})
	if errors.Is(err, symlink) {
		return []string{".git"}, nil
	}
	return excluded, err
}

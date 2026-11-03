package reposerver

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// archiveExclusions returns literal checkout-relative paths. Directory exclusions
// retain the repository layout while pruning unrelated subtrees.
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
		_, statErr := os.Lstat(fullPath)
		if statErr == nil {
			roots = append(roots, path)
		} else if !errors.Is(statErr, fs.ErrNotExist) {
			return nil, statErr
		}
		// Argo CD matches both literal paths and globs. An existing literal
		// filename must not hide other files matched by the same pattern.
		matches, err := fs.Glob(os.DirFS(directory), filepath.ToSlash(path))
		if err != nil {
			if statErr == nil {
				// A literal filename can contain invalid glob syntax.
				continue
			}
			return nil, fmt.Errorf("invalid stream path %q: %w", path, err)
		}
		for _, match := range matches {
			roots = append(roots, filepath.FromSlash(match))
		}
	}
	// A retained symlink can point into an otherwise excluded subtree. Use the
	// full checkout in that case, preserving the repo server's symlink checks.
	symlink := errors.New("retained symlink needs the full checkout")
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
		excluded = append(excluded, rel)
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

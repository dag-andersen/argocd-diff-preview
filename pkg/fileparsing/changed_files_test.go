package fileparsing

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListChangedFiles(t *testing.T) {
	// Create temporary directories for testing
	tempDir, err := os.MkdirTemp("", "changed_files_test_*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	dir1 := filepath.Join(tempDir, "dir1")
	dir2 := filepath.Join(tempDir, "dir2")

	require.NoError(t, os.MkdirAll(dir1, 0755))
	require.NoError(t, os.MkdirAll(dir2, 0755))

	t.Run("identical directories", func(t *testing.T) {
		// Create identical files in both directories
		testFiles := map[string]string{
			"file1.txt":        "content1",
			"file2.txt":        "content2",
			"subdir/file3.txt": "content3",
		}

		createTestFiles(t, dir1, testFiles)
		createTestFiles(t, dir2, testFiles)

		changedFiles, err := ListChangedFiles(dir1, dir2)
		require.NoError(t, err)
		assert.Empty(t, changedFiles, "Identical directories should have no changed files")
	})

	t.Run("modified files", func(t *testing.T) {
		// Clean directories
		cleanDir(t, dir1)
		cleanDir(t, dir2)

		// Create files with same names but different content
		createTestFiles(t, dir1, map[string]string{
			"modified.txt": "original content",
			"same.txt":     "identical content",
		})
		createTestFiles(t, dir2, map[string]string{
			"modified.txt": "modified content",
			"same.txt":     "identical content",
		})

		changedFiles, err := ListChangedFiles(dir1, dir2)
		require.NoError(t, err)
		sort.Strings(changedFiles)

		expected := []string{"modified.txt"}
		assert.Equal(t, expected, changedFiles, "Should detect modified files")
	})

	t.Run("new files", func(t *testing.T) {
		// Clean directories
		cleanDir(t, dir1)
		cleanDir(t, dir2)

		// Create files only in dir2
		createTestFiles(t, dir1, map[string]string{
			"existing.txt": "existing content",
		})
		createTestFiles(t, dir2, map[string]string{
			"existing.txt":          "existing content",
			"new_file.txt":          "new content",
			"subdir/new_nested.txt": "nested content",
		})

		changedFiles, err := ListChangedFiles(dir1, dir2)
		require.NoError(t, err)
		sort.Strings(changedFiles)

		expected := []string{"new_file.txt", "subdir/new_nested.txt"}
		sort.Strings(expected)
		assert.Equal(t, expected, changedFiles, "Should detect new files")
	})

	t.Run("deleted files", func(t *testing.T) {
		// Clean directories
		cleanDir(t, dir1)
		cleanDir(t, dir2)

		// Create files only in dir1
		createTestFiles(t, dir1, map[string]string{
			"existing.txt":              "existing content",
			"deleted.txt":               "deleted content",
			"subdir/deleted_nested.txt": "nested deleted content",
		})
		createTestFiles(t, dir2, map[string]string{
			"existing.txt": "existing content",
		})

		changedFiles, err := ListChangedFiles(dir1, dir2)
		require.NoError(t, err)
		sort.Strings(changedFiles)

		expected := []string{"deleted.txt", "subdir/deleted_nested.txt"}
		sort.Strings(expected)
		assert.Equal(t, expected, changedFiles, "Should detect deleted files")
	})

	t.Run("mixed changes", func(t *testing.T) {
		// Clean directories
		cleanDir(t, dir1)
		cleanDir(t, dir2)

		// Create complex scenario with all types of changes
		createTestFiles(t, dir1, map[string]string{
			"same.txt":                  "identical content",
			"modified.txt":              "original content",
			"deleted.txt":               "will be deleted",
			"subdir/same.txt":           "identical nested content",
			"subdir/deleted_nested.txt": "will be deleted nested",
		})
		createTestFiles(t, dir2, map[string]string{
			"same.txt":              "identical content",
			"modified.txt":          "modified content",
			"new.txt":               "new content",
			"subdir/same.txt":       "identical nested content",
			"subdir/new_nested.txt": "new nested content",
		})

		changedFiles, err := ListChangedFiles(dir1, dir2)
		require.NoError(t, err)
		sort.Strings(changedFiles)

		expected := []string{
			"deleted.txt",
			"modified.txt",
			"new.txt",
			"subdir/deleted_nested.txt",
			"subdir/new_nested.txt",
		}
		sort.Strings(expected)
		assert.Equal(t, expected, changedFiles, "Should detect all types of changes")
	})

	t.Run("empty directories", func(t *testing.T) {
		// Clean directories
		cleanDir(t, dir1)
		cleanDir(t, dir2)

		changedFiles, err := ListChangedFiles(dir1, dir2)
		require.NoError(t, err)
		assert.Empty(t, changedFiles, "Empty directories should have no changed files")
	})

	t.Run("one empty directory", func(t *testing.T) {
		// Clean directories
		cleanDir(t, dir1)
		cleanDir(t, dir2)

		// Create files only in dir2
		createTestFiles(t, dir2, map[string]string{
			"file1.txt":        "content1",
			"subdir/file2.txt": "content2",
		})

		changedFiles, err := ListChangedFiles(dir1, dir2)
		require.NoError(t, err)
		sort.Strings(changedFiles)

		expected := []string{"file1.txt", "subdir/file2.txt"}
		sort.Strings(expected)
		assert.Equal(t, expected, changedFiles, "Should detect all files as new when comparing empty dir to non-empty")
	})

	t.Run("deep nested directories", func(t *testing.T) {
		// Clean directories
		cleanDir(t, dir1)
		cleanDir(t, dir2)

		// Create files with various levels of deep nesting
		createTestFiles(t, dir1, map[string]string{
			"root.txt":                                     "root content",
			"level1/file.txt":                              "level 1 content",
			"level1/level2/file.txt":                       "level 2 content",
			"level1/level2/level3/file.txt":                "level 3 content",
			"level1/level2/level3/level4/file.txt":         "level 4 content",
			"path/to/something/deep.txt":                   "deep path content",
			"very/deep/nested/structure/config.yaml":       "config content",
			"a/b/c/d/e/f/g/deeply_nested.txt":              "very deep content",
			"apps/frontend/src/components/Button.tsx":      "react component",
			"infrastructure/kubernetes/manifests/app.yaml": "k8s manifest",
		})

		createTestFiles(t, dir2, map[string]string{
			"root.txt":                                "root content",             // same
			"level1/file.txt":                         "modified level 1 content", // modified
			"level1/level2/file.txt":                  "level 2 content",          // same
			"level1/level2/level3/file.txt":           "modified level 3 content", // modified
			"level1/level2/level3/level4/file.txt":    "level 4 content",          // same
			"path/to/something/deep.txt":              "deep path content",        // same
			"very/deep/nested/structure/config.yaml":  "modified config content",  // modified
			"a/b/c/d/e/f/g/deeply_nested.txt":         "very deep content",        // same
			"apps/frontend/src/components/Button.tsx": "modified react component", // modified
			// "infrastructure/kubernetes/manifests/app.yaml" - deleted
			"new/deep/path/added.txt":                    "new deep file",      // new
			"level1/level2/level3/level4/level5/new.txt": "very deep new file", // new
		})

		changedFiles, err := ListChangedFiles(dir1, dir2)
		require.NoError(t, err)
		sort.Strings(changedFiles)

		expected := []string{
			"apps/frontend/src/components/Button.tsx",
			"infrastructure/kubernetes/manifests/app.yaml",
			"level1/file.txt",
			"level1/level2/level3/file.txt",
			"level1/level2/level3/level4/level5/new.txt",
			"new/deep/path/added.txt",
			"very/deep/nested/structure/config.yaml",
		}
		sort.Strings(expected)
		assert.Equal(t, expected, changedFiles, "Should detect changes in deeply nested directories")
	})
}

func TestListChangedFilesWithNonExistentDirectories(t *testing.T) {
	t.Run("non-existent first directory", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "changed_files_test_*")
		require.NoError(t, err)
		defer func() { _ = os.RemoveAll(tempDir) }()

		nonExistentDir := filepath.Join(tempDir, "non_existent")
		existingDir := filepath.Join(tempDir, "existing")
		require.NoError(t, os.MkdirAll(existingDir, 0755))

		changedFiles, err := ListChangedFiles(nonExistentDir, existingDir)
		assert.Error(t, err, "Should return error when first directory doesn't exist")
		assert.Empty(t, changedFiles, "Should return empty slice when first directory doesn't exist")
		assert.Contains(t, err.Error(), "no such file or directory", "Error should indicate directory doesn't exist")
	})

	t.Run("non-existent second directory", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "changed_files_test_*")
		require.NoError(t, err)
		defer func() { _ = os.RemoveAll(tempDir) }()

		existingDir := filepath.Join(tempDir, "existing")
		nonExistentDir := filepath.Join(tempDir, "non_existent")
		require.NoError(t, os.MkdirAll(existingDir, 0755))

		changedFiles, err := ListChangedFiles(existingDir, nonExistentDir)
		assert.Error(t, err, "Should return error when second directory doesn't exist")
		assert.Empty(t, changedFiles, "Should return empty slice when second directory doesn't exist")
		assert.Contains(t, err.Error(), "no such file or directory", "Error should indicate directory doesn't exist")
	})
}

func TestCalculateFileHash(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "hash_test_*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	t.Run("identical content produces same hash", func(t *testing.T) {
		file1 := filepath.Join(tempDir, "file1.txt")
		file2 := filepath.Join(tempDir, "file2.txt")
		content := "test content"

		require.NoError(t, os.WriteFile(file1, []byte(content), 0644))
		require.NoError(t, os.WriteFile(file2, []byte(content), 0644))

		hash1, err := calculateFileHash(file1)
		require.NoError(t, err)

		hash2, err := calculateFileHash(file2)
		require.NoError(t, err)

		assert.Equal(t, hash1, hash2, "Identical content should produce identical hashes")
		assert.NotEmpty(t, hash1, "Hash should not be empty")
	})

	t.Run("different content produces different hash", func(t *testing.T) {
		file1 := filepath.Join(tempDir, "file1.txt")
		file2 := filepath.Join(tempDir, "file2.txt")

		require.NoError(t, os.WriteFile(file1, []byte("content1"), 0644))
		require.NoError(t, os.WriteFile(file2, []byte("content2"), 0644))

		hash1, err := calculateFileHash(file1)
		require.NoError(t, err)

		hash2, err := calculateFileHash(file2)
		require.NoError(t, err)

		assert.NotEqual(t, hash1, hash2, "Different content should produce different hashes")
	})

	t.Run("non-existent file returns error", func(t *testing.T) {
		nonExistentFile := filepath.Join(tempDir, "non_existent.txt")
		_, err := calculateFileHash(nonExistentFile)
		assert.Error(t, err, "Should return error for non-existent file")
	})

	t.Run("empty file produces valid hash", func(t *testing.T) {
		emptyFile := filepath.Join(tempDir, "empty.txt")
		require.NoError(t, os.WriteFile(emptyFile, []byte(""), 0644))

		hash, err := calculateFileHash(emptyFile)
		require.NoError(t, err)
		assert.NotEmpty(t, hash, "Empty file should produce a valid hash")
	})
}

func TestGetDirectoryHashes(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "dir_hash_test_*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	t.Run("directory with files", func(t *testing.T) {
		testFiles := map[string]string{
			"file1.txt":        "content1",
			"file2.txt":        "content2",
			"subdir/file3.txt": "content3",
		}
		createTestFiles(t, tempDir, testFiles)

		hashes, err := getDirectoryHashes(tempDir)
		require.NoError(t, err)

		assert.Len(t, hashes, 3, "Should have hashes for all files")
		assert.Contains(t, hashes, "file1.txt")
		assert.Contains(t, hashes, "file2.txt")
		assert.Contains(t, hashes, "subdir/file3.txt")

		// Verify all hashes are non-empty
		for path, hash := range hashes {
			assert.NotEmpty(t, hash, "Hash for %s should not be empty", path)
		}
	})

	t.Run("empty directory", func(t *testing.T) {
		emptyDir := filepath.Join(tempDir, "empty")
		require.NoError(t, os.MkdirAll(emptyDir, 0755))

		hashes, err := getDirectoryHashes(emptyDir)
		require.NoError(t, err)
		assert.Empty(t, hashes, "Empty directory should return empty hash map")
	})

	t.Run("non-existent directory", func(t *testing.T) {
		nonExistentDir := filepath.Join(tempDir, "non_existent")
		_, err := getDirectoryHashes(nonExistentDir)
		assert.Error(t, err, "Should return error for non-existent directory")
	})

	t.Run("deeply nested directory structure", func(t *testing.T) {
		deepDir := filepath.Join(tempDir, "deep_test")
		require.NoError(t, os.MkdirAll(deepDir, 0755))
		defer func() { _ = os.RemoveAll(deepDir) }()

		testFiles := map[string]string{
			"root.txt":                               "root content",
			"level1/level2/level3/deep.txt":          "deep content",
			"path/to/something/file.txt":             "path content",
			"very/deep/nested/structure/config.yaml": "config content",
			"a/b/c/d/e/f/deeply_nested.txt":          "very deep content",
		}
		createTestFiles(t, deepDir, testFiles)

		hashes, err := getDirectoryHashes(deepDir)
		require.NoError(t, err)

		assert.Len(t, hashes, 5, "Should have hashes for all deeply nested files")
		assert.Contains(t, hashes, "root.txt")
		assert.Contains(t, hashes, "level1/level2/level3/deep.txt")
		assert.Contains(t, hashes, "path/to/something/file.txt")
		assert.Contains(t, hashes, "very/deep/nested/structure/config.yaml")
		assert.Contains(t, hashes, "a/b/c/d/e/f/deeply_nested.txt")

		// Verify all hashes are non-empty
		for path, hash := range hashes {
			assert.NotEmpty(t, hash, "Hash for %s should not be empty", path)
		}
	})
}

// Helper functions

// createTestFiles creates files with given content in the specified directory
func createTestFiles(t *testing.T, baseDir string, files map[string]string) {
	for relPath, content := range files {
		fullPath := filepath.Join(baseDir, relPath)
		dir := filepath.Dir(fullPath)

		require.NoError(t, os.MkdirAll(dir, 0755))
		require.NoError(t, os.WriteFile(fullPath, []byte(content), 0644))
	}
}

// cleanDir removes all contents from a directory
func cleanDir(t *testing.T, dir string) {
	require.NoError(t, os.RemoveAll(dir))
	require.NoError(t, os.MkdirAll(dir, 0755))
}

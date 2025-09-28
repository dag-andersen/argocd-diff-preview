package fileparsing

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"
)

// ListChangedFiles compares two directories and returns a list of files that have changed.
// It uses SHA-256 hashes to efficiently detect changes, additions, and modifications.
// Returns relative file paths of changed files.
func ListChangedFiles(folder1 string, folder2 string) ([]string, time.Duration, error) {
	startTime := time.Now()

	// Get file hashes for both directories
	hashes1, err := getDirectoryHashes(folder1)
	if err != nil {
		log.Error().Err(err).Str("folder", folder1).Msg("❌ Failed to get hashes for folder1")
		return []string{}, time.Since(startTime), err
	}

	hashes2, err := getDirectoryHashes(folder2)
	if err != nil {
		log.Error().Err(err).Str("folder", folder2).Msg("❌ Failed to get hashes for folder2")
		return []string{}, time.Since(startTime), err
	}

	var changedFiles []string

	// Check for modified and new files
	for relPath, hash2 := range hashes2 {
		if hash1, exists := hashes1[relPath]; !exists {
			// File exists in folder2 but not in folder1 (new file)
			changedFiles = append(changedFiles, relPath)
			log.Debug().Str("file", relPath).Msg("📄 New file detected")
		} else if hash1 != hash2 {
			// File exists in both but has different hash (modified file)
			changedFiles = append(changedFiles, relPath)
			log.Debug().Str("file", relPath).Msg("📝 Modified file detected")
		}
	}

	// Check for deleted files
	for relPath := range hashes1 {
		if _, exists := hashes2[relPath]; !exists {
			// File exists in folder1 but not in folder2 (deleted file)
			changedFiles = append(changedFiles, relPath)
			log.Debug().Str("file", relPath).Msg("🗑️ Deleted file detected")
		}
	}

	return changedFiles, time.Since(startTime), nil
}

// Ignore these folders when comparing files
var ignoreFolders = []string{
	".git",
}

// shouldIgnoreFolder checks if a folder should be ignored based on its name
func shouldIgnoreFolder(folderName string) bool {
	for _, ignoreFolder := range ignoreFolders {
		if folderName == ignoreFolder {
			return true
		}
	}
	return false
}

// getDirectoryHashes recursively walks a directory and returns a map of relative file paths to their SHA-256 hashes
func getDirectoryHashes(dirPath string) (map[string]string, error) {
	hashes := make(map[string]string)

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip ignored directories
		if info.IsDir() {
			if shouldIgnoreFolder(info.Name()) {
				log.Debug().Str("folder", info.Name()).Msg("🚫 Skipping ignored folder")
				return filepath.SkipDir
			}
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(dirPath, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path for %s: %w", path, err)
		}

		// Calculate hash for the file
		hash, err := calculateFileHash(path)
		if err != nil {
			return fmt.Errorf("failed to calculate hash for %s: %w", path, err)
		}

		hashes[relPath] = hash
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory %s: %w", dirPath, err)
	}

	return hashes, nil
}

// calculateFileHash calculates the SHA-256 hash of a file
func calculateFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Warn().Err(err).Str("file", filePath).Msg("⚠️ Failed to close file")
		}
	}()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}

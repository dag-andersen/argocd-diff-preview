package reposerver

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/argoproj/argo-cd/v3/util/tgzstream"
)

// compressArchive keeps the Argo CD compressor for full-checkout fallbacks.
// Scoped archives use literal exclusions to avoid checking every entry against
// every glob. Both writers preserve the same headers, file order, and checksum.
func compressArchive(directory string, excluded []string) (*os.File, int, string, error) {
	if len(excluded) == 1 && excluded[0] == ".git" {
		return tgzstream.CompressFiles(directory, []string{"*"}, excluded)
	}
	file, err := os.CreateTemp("", "argocd-diff-preview-*.tgz")
	if err != nil {
		return nil, 0, "", err
	}
	hash := sha256.New()
	gz := gzip.NewWriter(io.MultiWriter(file, hash))
	count, err := writeScopedTar(gz, directory, excluded)
	if err = errors.Join(err, gz.Close()); err != nil {
		tgzstream.CloseAndDelete(file)
		return nil, 0, "", fmt.Errorf("failed to write archive: %w", err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		tgzstream.CloseAndDelete(file)
		return nil, 0, "", err
	}
	return file, count, hex.EncodeToString(hash.Sum(nil)), nil
}

func writeScopedTar(writer io.Writer, directory string, excluded []string) (int, error) {
	blocked := make(map[string]bool, len(excluded))
	for _, path := range excluded {
		blocked[path] = true
	}
	tw := tar.NewWriter(writer)
	count := 0
	err := filepath.WalkDir(directory, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(directory, path)
		if err != nil {
			return err
		}
		// Check before entry.Info so excluded siblings need no metadata reads.
		if blocked[rel] {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		mode := info.Mode()
		if !mode.IsRegular() && !mode.IsDir() && mode&fs.ModeSymlink == 0 {
			return nil
		}
		link := ""
		if mode&fs.ModeSymlink != 0 {
			link, err = os.Readlink(path)
			if err != nil {
				return err
			}
		}
		header, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return err
		}
		header.Name = rel
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if mode.IsRegular() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(tw, file)
			if err := errors.Join(copyErr, file.Close()); err != nil {
				return err
			}
			count++
		}
		return nil
	})
	return count, errors.Join(err, tw.Close())
}

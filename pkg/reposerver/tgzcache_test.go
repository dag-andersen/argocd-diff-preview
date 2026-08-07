package reposerver

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTgzCacheOpenCachedTgzCompressesOnceForConcurrentCallers(t *testing.T) {
	appDir := t.TempDir()
	archiveDir := t.TempDir()

	var compressions atomic.Int32
	cache := newTgzCacheWithCompressor(func(string, []string, []string) (*os.File, int, string, error) {
		compressions.Add(1)
		time.Sleep(20 * time.Millisecond)
		file, err := createTestArchive(archiveDir, "archive")
		return file, 7, "checksum", err
	})

	const callers = 20
	start := make(chan struct{})
	errs := make(chan error, callers)

	var wg sync.WaitGroup
	for range callers {
		wg.Go(func() {
			<-start

			file, checksum, files, err := cache.openCachedTgz(appDir)
			if err != nil {
				errs <- err
				return
			}
			if err := file.Close(); err != nil {
				errs <- err
				return
			}

			if checksum != "checksum" {
				errs <- fmt.Errorf("checksum = %q, want checksum", checksum)
				return
			}
			if files != 7 {
				errs <- fmt.Errorf("files = %d, want 7", files)
				return
			}
		})
	}

	close(start)
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	if got := compressions.Load(); got != 1 {
		t.Fatalf("compressions = %d, want 1", got)
	}
}

func TestTgzCacheCleanupRemovesOnlyOwnArchives(t *testing.T) {
	appDir := t.TempDir()
	archiveDir := t.TempDir()

	newCache := func(prefix string) *tgzCache {
		return newTgzCacheWithCompressor(func(string, []string, []string) (*os.File, int, string, error) {
			file, err := createTestArchive(archiveDir, prefix)
			return file, 1, prefix, err
		})
	}

	cacheA := newCache("a")
	cacheB := newCache("b")

	fileA, _, _, err := cacheA.openCachedTgz(appDir)
	if err != nil {
		t.Fatal(err)
	}
	pathA := fileA.Name()
	if err := fileA.Close(); err != nil {
		t.Fatal(err)
	}

	fileB, _, _, err := cacheB.openCachedTgz(appDir)
	if err != nil {
		t.Fatal(err)
	}
	pathB := fileB.Name()
	if err := fileB.Close(); err != nil {
		t.Fatal(err)
	}

	cacheA.Cleanup()

	if _, err := os.Stat(pathA); !os.IsNotExist(err) {
		t.Fatalf("cacheA archive still exists or stat failed with unexpected error: %v", err)
	}
	if _, err := os.Stat(pathB); err != nil {
		t.Fatalf("cacheB archive was removed by cacheA cleanup: %v", err)
	}
}

func TestTgzCacheOpenCachedTgzReturnsErrorForMissingArchive(t *testing.T) {
	appDir := t.TempDir()
	archiveDir := t.TempDir()

	var compressions atomic.Int32
	cache := newTgzCacheWithCompressor(func(string, []string, []string) (*os.File, int, string, error) {
		compression := compressions.Add(1)
		checksum := fmt.Sprintf("checksum-%d", compression)
		file, err := createTestArchive(archiveDir, checksum)
		return file, 1, checksum, err
	})

	firstFile, _, _, err := cache.openCachedTgz(appDir)
	if err != nil {
		t.Fatal(err)
	}
	firstPath := firstFile.Name()
	if err := firstFile.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(firstPath); err != nil {
		t.Fatal(err)
	}

	if _, _, _, err := cache.openCachedTgz(appDir); err == nil {
		t.Fatal("expected missing cached archive to return an error")
	}
	if got := compressions.Load(); got != 1 {
		t.Fatalf("compressions = %d, want 1", got)
	}
}

func createTestArchive(dir, content string) (*os.File, error) {
	file, err := os.CreateTemp(dir, "archive-*")
	if err != nil {
		return nil, err
	}
	if _, err := file.WriteString(content); err != nil {
		if closeErr := file.Close(); closeErr != nil {
			return nil, fmt.Errorf("write archive: %w; close archive: %w", err, closeErr)
		}
		return nil, err
	}
	if _, err := file.Seek(0, 0); err != nil {
		if closeErr := file.Close(); closeErr != nil {
			return nil, fmt.Errorf("seek archive: %w; close archive: %w", err, closeErr)
		}
		return nil, err
	}
	return file, nil
}

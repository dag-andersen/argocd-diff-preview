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

			archive, err := cache.openCachedTgz(appDir)
			if err != nil {
				errs <- err
				return
			}
			if err := archive.file.Close(); err != nil {
				errs <- err
				return
			}

			if archive.checksum != "checksum" {
				errs <- fmt.Errorf("checksum = %q, want checksum", archive.checksum)
				return
			}
			if archive.files != 7 {
				errs <- fmt.Errorf("files = %d, want 7", archive.files)
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
	stats := cache.stats()
	if stats.Requests != callers || stats.Misses != 1 || stats.Hits != callers-1 {
		t.Fatalf("stats = %+v, want requests=%d misses=1 hits=%d", stats, callers, callers-1)
	}
	if stats.CompressedFiles != 7 {
		t.Fatalf("compressed files = %d, want 7", stats.CompressedFiles)
	}
	if stats.CompressedBytes == 0 {
		t.Fatal("compressed bytes = 0, want a non-zero archive size")
	}
	if stats.CompressionDuration < 20*time.Millisecond {
		t.Fatalf("compression duration = %s, want at least 20ms", stats.CompressionDuration)
	}
	if stats.HitWaitDuration == 0 {
		t.Fatal("hit wait duration = 0, want concurrent cache hits to record wait time")
	}
}

func TestTgzCacheOpenCachedTgzReportsArchiveMetrics(t *testing.T) {
	appDir := t.TempDir()
	archiveDir := t.TempDir()

	cache := newTgzCacheWithCompressor(func(string, []string, []string) (*os.File, int, string, error) {
		file, err := createTestArchive(archiveDir, "archive-content")
		return file, 3, "checksum", err
	})

	archive, err := cache.openCachedTgz(appDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := archive.file.Close(); err != nil {
		t.Fatal(err)
	}
	if archive.checksum != "checksum" || archive.files != 3 {
		t.Fatalf("checksum/files = %q/%d, want checksum/3", archive.checksum, archive.files)
	}
	if archive.bytes != int64(len("archive-content")) {
		t.Fatalf("archiveBytes = %d, want %d", archive.bytes, len("archive-content"))
	}
	if archive.cacheHit {
		t.Fatal("first archive open unexpectedly reported a cache hit")
	}

	archive, err = cache.openCachedTgz(appDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := archive.file.Close(); err != nil {
		t.Fatal(err)
	}
	if !archive.cacheHit {
		t.Fatal("second archive open did not report a cache hit")
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

	archiveA, err := cacheA.openCachedTgz(appDir)
	if err != nil {
		t.Fatal(err)
	}
	pathA := archiveA.file.Name()
	if err := archiveA.file.Close(); err != nil {
		t.Fatal(err)
	}

	archiveB, err := cacheB.openCachedTgz(appDir)
	if err != nil {
		t.Fatal(err)
	}
	pathB := archiveB.file.Name()
	if err := archiveB.file.Close(); err != nil {
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

	archive, err := cache.openCachedTgz(appDir)
	if err != nil {
		t.Fatal(err)
	}
	firstPath := archive.file.Name()
	if err := archive.file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(firstPath); err != nil {
		t.Fatal(err)
	}

	if _, err := cache.openCachedTgz(appDir); err == nil {
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

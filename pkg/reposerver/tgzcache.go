package reposerver

import (
	"os"
	"sync"

	"github.com/argoproj/argo-cd/v3/util/tgzstream"
	"github.com/rs/zerolog/log"
)

// Applications routinely share a source directory: one chart rendered per cluster
// is one Application each, all pointing at the same path. Compressing it per
// Application is pure waste — the archive and its checksum are identical, and the
// repo server's manifest cache keys on that checksum, so the repeats do not even
// produce different work on the far side.
//
// Measured on a 335-Application installation: 500 source entries resolve to 79
// distinct paths, i.e. every directory is compressed 6.3 times on average. The
// worst offender is compressed 120 times. A diff run doubles that again, since the
// base and target branches are rendered separately.
//
// So compress once per directory and hand out read-only handles to the result.
type tgzEntry struct {
	once     sync.Once
	path     string
	checksum string
	files    int
	err      error
}

var (
	tgzCacheMu sync.Mutex
	tgzCache   = map[string]*tgzEntry{}
)

// compressCached compresses appDir once and returns the archive's path, its
// checksum and the number of files written. Concurrent callers for the same
// directory block on the first one rather than each compressing their own copy.
//
// The caller must open its own handle: the archive is read to EOF per request and
// seeked back on retry, so a shared *os.File would race across goroutines.
func compressCached(appDir string) (string, string, int, error) {
	tgzCacheMu.Lock()
	entry, ok := tgzCache[appDir]
	if !ok {
		entry = &tgzEntry{}
		tgzCache[appDir] = entry
	}
	tgzCacheMu.Unlock()

	entry.once.Do(func() {
		file, files, checksum, err := tgzstream.CompressFiles(appDir, []string{"*"}, []string{".git"})
		if err != nil {
			entry.err = err
			return
		}
		// Keep the archive on disk — CloseAndDelete would defeat the cache.
		entry.path, entry.checksum, entry.files = file.Name(), checksum, files
		if err := file.Close(); err != nil {
			log.Debug().Err(err).Str("dir", appDir).Msg("Failed to close freshly compressed archive")
		}
	})

	return entry.path, entry.checksum, entry.files, entry.err
}

// openCachedTgz returns a fresh read handle on the archive for appDir, compressing
// it first if this is the first caller.
func openCachedTgz(appDir string) (*os.File, string, int, error) {
	path, checksum, files, err := compressCached(appDir)
	if err != nil {
		return nil, "", 0, err
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, "", 0, err
	}
	return file, checksum, files, nil
}

// CleanupTgzCache removes every cached archive. Safe to call more than once.
func CleanupTgzCache() {
	tgzCacheMu.Lock()
	defer tgzCacheMu.Unlock()

	for dir, entry := range tgzCache {
		if entry.path == "" {
			continue
		}
		if err := os.Remove(entry.path); err != nil && !os.IsNotExist(err) {
			log.Debug().Err(err).Str("dir", dir).Msg("Failed to remove cached archive")
		}
	}
	tgzCache = map[string]*tgzEntry{}
}

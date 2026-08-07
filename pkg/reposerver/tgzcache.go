package reposerver

import (
	"os"
	"sync"

	"github.com/argoproj/argo-cd/v3/util/tgzstream"
	"github.com/rs/zerolog/log"
)

// Applications routinely share a source directory: one chart rendered per cluster
// is one Application each, all pointing at the same path. Compressing it per
// Application is pure waste - the archive and its checksum are identical, and the
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

type tgzCompressor func(appDir string, included []string, excluded []string) (*os.File, int, string, error)

type tgzCache struct {
	mu       sync.Mutex
	entries  map[string]*tgzEntry
	compress tgzCompressor
}

func newTgzCache() *tgzCache {
	return newTgzCacheWithCompressor(tgzstream.CompressFiles)
}

func newTgzCacheWithCompressor(compress tgzCompressor) *tgzCache {
	return &tgzCache{
		entries:  map[string]*tgzEntry{},
		compress: compress,
	}
}

// compressCached compresses appDir once and returns the cache entry for the
// archive. Concurrent callers for the same directory block on the first one
// rather than each compressing their own copy.
//
// The caller must open its own handle: the archive is read to EOF per request and
// seeked back on retry, so a shared *os.File would race across goroutines.
func (c *tgzCache) compressCached(appDir string) (*tgzEntry, error) {
	c.mu.Lock()
	entry, ok := c.entries[appDir]
	if !ok {
		entry = &tgzEntry{}
		c.entries[appDir] = entry
	}
	c.mu.Unlock()

	entry.once.Do(func() {
		file, files, checksum, err := c.compress(appDir, []string{"*"}, []string{".git"})
		if err != nil {
			entry.err = err
			return
		}
		// Keep the archive on disk - CloseAndDelete would defeat the cache.
		entry.path, entry.checksum, entry.files = file.Name(), checksum, files
		if err := file.Close(); err != nil {
			log.Debug().Err(err).Str("dir", appDir).Msg("Failed to close freshly compressed archive")
		}
	})

	return entry, entry.err
}

// openCachedTgz returns a fresh read handle on the archive for appDir, compressing
// it first if this is the first caller.
func (c *tgzCache) openCachedTgz(appDir string) (*os.File, string, int, error) {
	entry, err := c.compressCached(appDir)
	if err != nil {
		return nil, "", 0, err
	}

	file, err := os.Open(entry.path)
	if err != nil {
		return nil, "", 0, err
	}
	return file, entry.checksum, entry.files, nil
}

// Cleanup removes every cached archive. Safe to call more than once.
func (c *tgzCache) Cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for dir, entry := range c.entries {
		if entry.path == "" {
			continue
		}
		if err := os.Remove(entry.path); err != nil && !os.IsNotExist(err) {
			log.Debug().Err(err).Str("dir", dir).Msg("Failed to remove cached archive")
		}
	}
	c.entries = map[string]*tgzEntry{}
}

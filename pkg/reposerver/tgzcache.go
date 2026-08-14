package reposerver

import (
	"math"
	"os"
	"sync"
	"sync/atomic"
	"time"

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

	requests         atomic.Int64
	hits             atomic.Int64
	misses           atomic.Int64
	compressionNanos atomic.Int64
	hitWaitNanos     atomic.Int64
	compressedBytes  atomic.Int64
	compressedFiles  atomic.Int64
}

type tgzCacheStats struct {
	Requests            int64
	Hits                int64
	Misses              int64
	CompressionDuration time.Duration
	HitWaitDuration     time.Duration
	CompressedBytes     int64
	CompressedFiles     int64
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
	c.requests.Add(1)

	c.mu.Lock()
	entry, hit := c.entries[appDir]
	if !hit {
		entry = &tgzEntry{}
		c.entries[appDir] = entry
		c.misses.Add(1)
	} else {
		c.hits.Add(1)
	}
	c.mu.Unlock()

	waitStart := time.Now()
	entry.once.Do(func() {
		compressStart := time.Now()
		file, files, checksum, err := c.compress(appDir, []string{"*"}, []string{".git"})
		c.compressionNanos.Add(int64(time.Since(compressStart)))
		if err != nil {
			entry.err = err
			return
		}
		// Keep the archive on disk - CloseAndDelete would defeat the cache.
		entry.path, entry.checksum, entry.files = file.Name(), checksum, files
		c.compressedFiles.Add(int64(files))
		if info, statErr := file.Stat(); statErr == nil {
			c.compressedBytes.Add(info.Size())
		}
		if err := file.Close(); err != nil {
			log.Debug().Err(err).Str("dir", appDir).Msg("Failed to close freshly compressed archive")
		}
	})
	if hit {
		c.hitWaitNanos.Add(int64(time.Since(waitStart)))
	}

	return entry, entry.err
}

func (c *tgzCache) stats() tgzCacheStats {
	return tgzCacheStats{
		Requests:            c.requests.Load(),
		Hits:                c.hits.Load(),
		Misses:              c.misses.Load(),
		CompressionDuration: time.Duration(c.compressionNanos.Load()),
		HitWaitDuration:     time.Duration(c.hitWaitNanos.Load()),
		CompressedBytes:     c.compressedBytes.Load(),
		CompressedFiles:     c.compressedFiles.Load(),
	}
}

func (c *tgzCache) logStats() {
	stats := c.stats()
	if stats.Requests == 0 {
		return
	}
	hitRate := float64(stats.Hits) * 100 / float64(stats.Requests)
	log.Info().
		Int64("requests", stats.Requests).
		Int64("hits", stats.Hits).
		Int64("misses", stats.Misses).
		Float64("hitRatePercent", math.Round(hitRate)).
		Int64("archives", stats.Misses).
		Int64("sourceFiles", stats.CompressedFiles).
		Int64("archiveBytes", stats.CompressedBytes).
		Float64("compressionTime", math.Round(float64(stats.CompressionDuration)/float64(time.Millisecond))).
		Float64("hitWaitTime", math.Round(float64(stats.HitWaitDuration)/float64(time.Millisecond))).
		Msg("📦 Cache stats")
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
	c.logStats()

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

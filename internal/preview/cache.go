package preview

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Cache stores base video frames on disk, keyed by (videoPath, time, widthPx).
// When total cache size exceeds maxBytes, the oldest (by mtime) entries are
// evicted until the cache is under the limit again.
type Cache struct {
	dir      string
	maxBytes int64
	mu       sync.Mutex
}

// NewCache creates a cache rooted at dir. maxBytes is the total size budget
// (0 disables eviction).
func NewCache(dir string, maxBytes int64) *Cache {
	return &Cache{dir: dir, maxBytes: maxBytes}
}

// Key derives a stable filename (no extension) from the cache inputs.
func (c *Cache) Key(videoPath string, at time.Duration, widthPx int) string {
	h := sha1.New()
	fmt.Fprintf(h, "%s|%d|%d", videoPath, at.Milliseconds(), widthPx)
	return hex.EncodeToString(h.Sum(nil))
}

// Path returns the absolute path to where a given key's data lives.
func (c *Cache) Path(key string) string {
	return filepath.Join(c.dir, key+".png")
}

// Exists reports whether the cache has an entry for this key.
func (c *Cache) Exists(key string) bool {
	info, err := os.Stat(c.Path(key))
	return err == nil && !info.IsDir()
}

// Read returns the cached bytes for this key.
func (c *Cache) Read(key string) ([]byte, error) {
	return os.ReadFile(c.Path(key))
}

// Write stores data under key. Creates the cache directory if needed.
// Triggers LRU eviction after the write.
func (c *Cache) Write(key string, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := os.MkdirAll(c.dir, 0755); err != nil {
		return fmt.Errorf("cache: create dir %q: %w", c.dir, err)
	}
	if err := os.WriteFile(c.Path(key), data, 0644); err != nil {
		return fmt.Errorf("cache: write %q: %w", key, err)
	}
	c.evictIfNeeded()
	return nil
}

// evictIfNeeded removes oldest entries until cache size is under maxBytes.
// Must be called with mu held.
func (c *Cache) evictIfNeeded() {
	if c.maxBytes <= 0 {
		return
	}
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return
	}

	type entryInfo struct {
		path  string
		size  int64
		mtime time.Time
	}
	var infos []entryInfo
	var total int64
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		infos = append(infos, entryInfo{
			path:  filepath.Join(c.dir, e.Name()),
			size:  info.Size(),
			mtime: info.ModTime(),
		})
		total += info.Size()
	}
	if total <= c.maxBytes {
		return
	}

	sort.Slice(infos, func(i, j int) bool {
		return infos[i].mtime.Before(infos[j].mtime)
	})

	for _, e := range infos {
		if total <= c.maxBytes {
			break
		}
		if err := os.Remove(e.path); err == nil {
			total -= e.size
		}
	}
}

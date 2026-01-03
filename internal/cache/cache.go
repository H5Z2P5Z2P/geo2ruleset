// Package cache provides in-memory caching for ZIP files and conversion results.
package cache

import (
	"archive/zip"
	"bytes"
	"encoding/gob"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ZipCache holds the cached ZIP file data
type ZipCache struct {
	mu          sync.RWMutex
	data        []byte
	reader      *zip.Reader
	etag        string
	timestamp   time.Time
	ttl         time.Duration
	persistPath string
}

// NewZipCache creates a new ZipCache with the specified TTL
func NewZipCache(ttl time.Duration) *ZipCache {
	return &ZipCache{
		ttl: ttl,
	}
}

// SetPersistPath enables on-disk persistence for the ZIP cache.
func (c *ZipCache) SetPersistPath(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.persistPath = path
}

// Get returns the cached zip.Reader if valid, and the current ETag
func (c *ZipCache) Get() (*zip.Reader, string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.reader == nil {
		return nil, "", false
	}

	if time.Since(c.timestamp) > c.ttl {
		return nil, c.etag, false
	}

	return c.reader, c.etag, true
}

// GetAny returns the cached zip.Reader regardless of TTL.
func (c *ZipCache) GetAny() (*zip.Reader, string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.reader == nil {
		return nil, "", false
	}

	return c.reader, c.etag, true
}

// Set updates the cache with new data
func (c *ZipCache) Set(data []byte, etag string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}

	c.data = data
	c.reader = reader
	c.etag = etag
	c.timestamp = time.Now()
	if c.persistPath == "" {
		return nil
	}
	return c.persistToFileLocked()
}

// GetETag returns the current ETag
func (c *ZipCache) GetETag() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.etag
}

type zipCachePersist struct {
	Data      []byte
	ETag      string
	Timestamp time.Time
}

// LoadFromFile restores cache data from disk if available.
func (c *ZipCache) LoadFromFile(path string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	var persisted zipCachePersist
	if err := gob.NewDecoder(file).Decode(&persisted); err != nil {
		return err
	}

	reader, err := zip.NewReader(bytes.NewReader(persisted.Data), int64(len(persisted.Data)))
	if err != nil {
		return err
	}

	c.data = persisted.Data
	c.reader = reader
	c.etag = persisted.ETag
	c.timestamp = persisted.Timestamp
	c.persistPath = path
	return nil
}

func (c *ZipCache) persistToFileLocked() error {
	if c.persistPath == "" {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(c.persistPath), 0o755); err != nil {
		return err
	}

	tmpPath := c.persistPath + ".tmp"
	file, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	enc := gob.NewEncoder(file)
	err = enc.Encode(zipCachePersist{
		Data:      c.data,
		ETag:      c.etag,
		Timestamp: c.timestamp,
	})
	closeErr := file.Close()
	if err != nil {
		os.Remove(tmpPath) // cleanup on failure
		return err
	}
	if closeErr != nil {
		os.Remove(tmpPath) // cleanup on failure
		return closeErr
	}

	return os.Rename(tmpPath, c.persistPath)
}

// ResultCache caches the conversion results
type ResultCache struct {
	mu      sync.RWMutex
	results map[string]*cacheEntry
	ttl     time.Duration
}

type cacheEntry struct {
	value     string
	timestamp time.Time
	etag      string
}

// NewResultCache creates a new ResultCache with the specified TTL
func NewResultCache(ttl time.Duration) *ResultCache {
	return &ResultCache{
		results: make(map[string]*cacheEntry),
		ttl:     ttl,
	}
}

// Get retrieves a cached result if valid
func (c *ResultCache) Get(key, etag string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.results[key]
	if !ok {
		return "", false
	}

	// Check if ETag matches and not expired
	if entry.etag != etag || time.Since(entry.timestamp) > c.ttl {
		return "", false
	}

	return entry.value, true
}

// Set stores a result in the cache
func (c *ResultCache) Set(key, value, etag string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.results[key] = &cacheEntry{
		value:     value,
		timestamp: time.Now(),
		etag:      etag,
	}
}

// Cleanup removes expired entries
func (c *ResultCache) Cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, entry := range c.results {
		if now.Sub(entry.timestamp) > c.ttl {
			delete(c.results, key)
		}
	}
}

package versioncheck

import (
	"os"
	"sync"
	"time"
)

// DefaultCacheTTL defines the default duration (5 minutes) to retain version check results.
const DefaultCacheTTL = 5 * time.Minute

type cacheEntry struct {
	status    ToolStatus
	binary    string
	modTime   time.Time
	expiresAt time.Time
}

// VersionCache is a thread-safe, TTL-based cache for tool version evaluation results.
type VersionCache struct {
	mu      sync.RWMutex
	entries map[string]cacheEntry
	ttl     time.Duration
	nowFunc func() time.Time
	stat    func(string) (os.FileInfo, error)
}

// NewVersionCache initializes a VersionCache with a specified TTL.
func NewVersionCache(ttl time.Duration) *VersionCache {
	if ttl <= 0 {
		ttl = DefaultCacheTTL
	}
	return &VersionCache{
		entries: make(map[string]cacheEntry),
		ttl:     ttl,
		nowFunc: time.Now,
		stat:    os.Stat,
	}
}

// DefaultCache is the global shared cache instance.
var DefaultCache = NewVersionCache(DefaultCacheTTL)

// Get retrieves a cached ToolStatus if valid and not expired.
// If binaryPath is provided, verifies that the binary's mtime matches the cached mtime.
func (c *VersionCache) Get(toolID, binaryPath string) (ToolStatus, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, found := c.entries[toolID]
	if !found {
		return ToolStatus{}, false
	}

	if c.nowFunc().After(entry.expiresAt) {
		return ToolStatus{}, false
	}

	// Verify binary has not been modified, replaced, or deleted
	if binaryPath != "" && binaryPath == entry.binary {
		fi, err := c.stat(binaryPath)
		if err != nil || !fi.ModTime().Equal(entry.modTime) {
			return ToolStatus{}, false
		}
	}

	return entry.status, true
}

// Set stores a ToolStatus in the cache with the current binary mtime.
func (c *VersionCache) Set(toolID, binaryPath string, status ToolStatus) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var mtime time.Time
	if binaryPath != "" {
		if fi, err := c.stat(binaryPath); err == nil {
			mtime = fi.ModTime()
		}
	}

	c.entries[toolID] = cacheEntry{
		status:    status,
		binary:    binaryPath,
		modTime:   mtime,
		expiresAt: c.nowFunc().Add(c.ttl),
	}
}

// Package cache provides secure caching for secrets with TTL support.
package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"runtime"
	"time"
)

// Backend represents a cache storage backend.
type Backend interface {
	// Get retrieves a cached secret. Returns nil if not found or expired.
	Get(key string) (*Entry, error)
	// Set stores a secret in the cache.
	Set(key string, entry *Entry) error
	// Delete removes a specific entry from the cache.
	Delete(key string) error
	// Clear removes all entries from the cache.
	Clear() error
	// Stats returns cache statistics.
	Stats() (*Stats, error)
	// Name returns the backend name.
	Name() string
}

// Entry represents a cached secret entry.
type Entry struct {
	// SecretData is the cached secret key-value pairs.
	SecretData map[string]string `json:"data"`
	// ExpiresAt is when the entry expires.
	ExpiresAt time.Time `json:"expires_at"`
	// CachedAt is when the entry was cached.
	CachedAt time.Time `json:"cached_at"`
	// Version is the cache format version for invalidation.
	Version int `json:"version"`
}

// Stats contains cache statistics.
type Stats struct {
	Backend    string
	EntryCount int
	HitCount   int64
	MissCount  int64
	Size       int64 // bytes, if available
}

// CacheVersion is the current cache format version.
// Increment this to invalidate all caches on upgrade.
const CacheVersion = 1

// IsExpired returns true if the entry has expired.
func (e *Entry) IsExpired() bool {
	return time.Now().After(e.ExpiresAt)
}

// IsValid returns true if the entry is valid (not expired and correct version).
func (e *Entry) IsValid() bool {
	return !e.IsExpired() && e.Version == CacheVersion
}

// Key generates a cache key from region and secret name.
// Uses SHA256 hash to avoid special characters in filenames/keyring keys.
func Key(region, secretName string) string {
	data := region + ":" + secretName
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:16]) // Use first 16 bytes (32 hex chars)
}

// ShouldCache returns true if caching should be enabled.
// Returns false if running as root (security risk).
func ShouldCache() bool {
	// Don't cache if running as root (security)
	if os.Geteuid() == 0 {
		return false
	}

	return true
}

// GetCacheDir returns the cache directory path.
func GetCacheDir() string {
	// Use XDG_CACHE_HOME if set
	if cacheHome := os.Getenv("XDG_CACHE_HOME"); cacheHome != "" {
		return cacheHome + "/envctl"
	}

	// Default locations
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}

	if runtime.GOOS == "darwin" {
		return home + "/Library/Caches/envctl"
	}

	return home + "/.cache/envctl"
}

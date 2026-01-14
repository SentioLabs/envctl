package cache

import (
	"fmt"
	"time"
)

// BackendType represents the type of cache backend.
type BackendType string

const (
	// BackendAuto automatically selects the best available backend.
	BackendAuto BackendType = "auto"
	// BackendKeyring uses the OS keyring.
	BackendKeyring BackendType = "keyring"
	// BackendFile uses encrypted files.
	BackendFile BackendType = "file"
	// BackendNone disables caching.
	BackendNone BackendType = "none"
)

// DefaultTTL is the default cache TTL.
const DefaultTTL = 15 * time.Minute

// Manager provides a unified interface for cache operations.
type Manager struct {
	backend Backend
	ttl     time.Duration
	enabled bool
}

// Options configures the cache manager.
type Options struct {
	// Enabled controls whether caching is active.
	Enabled bool
	// TTL is the time-to-live for cache entries.
	TTL time.Duration
	// Backend specifies the cache backend to use.
	Backend BackendType
}

// DefaultOptions returns the default cache options.
func DefaultOptions() Options {
	return Options{
		Enabled: true,
		TTL:     DefaultTTL,
		Backend: BackendAuto,
	}
}

// NewManager creates a new cache manager.
func NewManager(opts Options) (*Manager, error) {
	m := &Manager{
		ttl:     opts.TTL,
		enabled: opts.Enabled,
	}

	// Check if caching should be disabled
	if !opts.Enabled || opts.Backend == BackendNone || !ShouldCache() {
		m.enabled = false
		return m, nil
	}

	// Select backend
	var err error
	switch opts.Backend {
	case BackendKeyring:
		m.backend, err = NewKeyringBackend()
	case BackendFile:
		m.backend, err = NewFileBackend(GetCacheDir())
	case BackendAuto, "":
		m.backend, err = selectBestBackend()
	default:
		return nil, fmt.Errorf("unknown cache backend: %s", opts.Backend)
	}

	if err != nil {
		// If backend creation fails, disable caching but don't error
		m.enabled = false
		m.backend = nil
	}

	return m, nil
}

// selectBestBackend selects the best available cache backend.
func selectBestBackend() (Backend, error) {
	// Try keyring first (most secure)
	if IsKeyringAvailable() {
		backend, err := NewKeyringBackend()
		if err == nil {
			return backend, nil
		}
	}

	// Fall back to encrypted file cache
	return NewFileBackend(GetCacheDir())
}

// IsEnabled returns true if caching is enabled.
func (m *Manager) IsEnabled() bool {
	return m.enabled && m.backend != nil
}

// BackendName returns the name of the active backend.
func (m *Manager) BackendName() string {
	if m.backend == nil {
		return "none"
	}
	return m.backend.Name()
}

// Get retrieves a cached secret.
func (m *Manager) Get(region, secretName string) (map[string]string, error) {
	if !m.IsEnabled() {
		return nil, nil
	}

	key := CacheKey(region, secretName)
	entry, err := m.backend.Get(key)
	if err != nil {
		return nil, err
	}

	if entry == nil {
		return nil, nil
	}

	return entry.SecretData, nil
}

// Set stores a secret in the cache.
func (m *Manager) Set(region, secretName string, data map[string]string) error {
	if !m.IsEnabled() {
		return nil
	}

	key := CacheKey(region, secretName)
	entry := &Entry{
		SecretData: data,
		ExpiresAt:  time.Now().Add(m.ttl),
		CachedAt:   time.Now(),
		Version:    CacheVersion,
	}

	return m.backend.Set(key, entry)
}

// Delete removes a specific secret from the cache.
func (m *Manager) Delete(region, secretName string) error {
	if !m.IsEnabled() {
		return nil
	}

	key := CacheKey(region, secretName)
	return m.backend.Delete(key)
}

// Clear removes all cached secrets.
func (m *Manager) Clear() error {
	if m.backend == nil {
		return nil
	}
	return m.backend.Clear()
}

// Stats returns cache statistics.
func (m *Manager) Stats() (*Stats, error) {
	if m.backend == nil {
		return &Stats{Backend: "none"}, nil
	}
	return m.backend.Stats()
}

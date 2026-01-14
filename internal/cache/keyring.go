package cache

import (
	"encoding/json"
	"sync"

	"github.com/zalando/go-keyring"
)

const (
	// keyringService is the service name used in the OS keyring.
	keyringService = "envctl"
	// keyringIndexKey is the key used to store the index of cached entries.
	keyringIndexKey = "_index"
)

// KeyringBackend stores cache entries in the OS keyring.
type KeyringBackend struct {
	mu        sync.RWMutex
	hitCount  int64
	missCount int64
}

// NewKeyringBackend creates a new keyring-based cache backend.
func NewKeyringBackend() (*KeyringBackend, error) {
	// Test if keyring is available by trying to access it
	_, err := keyring.Get(keyringService, keyringIndexKey)
	if err != nil && err != keyring.ErrNotFound {
		// Keyring might not be available (e.g., no GUI session on Linux)
		return nil, err
	}

	return &KeyringBackend{}, nil
}

// Name returns the backend name.
func (k *KeyringBackend) Name() string {
	return "keyring"
}

// Get retrieves a cached entry from the keyring.
func (k *KeyringBackend) Get(key string) (*Entry, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()

	data, err := keyring.Get(keyringService, key)
	if err != nil {
		if err == keyring.ErrNotFound {
			k.missCount++
			return nil, nil
		}
		return nil, err
	}

	var entry Entry
	if err := json.Unmarshal([]byte(data), &entry); err != nil {
		// Corrupted - delete and return nil
		keyring.Delete(keyringService, key)
		k.missCount++
		return nil, nil
	}

	// Check validity
	if !entry.IsValid() {
		keyring.Delete(keyringService, key)
		k.removeFromIndex(key)
		k.missCount++
		return nil, nil
	}

	k.hitCount++
	return &entry, nil
}

// Set stores a cache entry in the keyring.
func (k *KeyringBackend) Set(key string, entry *Entry) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	if err := keyring.Set(keyringService, key, string(data)); err != nil {
		return err
	}

	// Update index
	return k.addToIndex(key)
}

// Delete removes a cache entry from the keyring.
func (k *KeyringBackend) Delete(key string) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	err := keyring.Delete(keyringService, key)
	if err != nil && err != keyring.ErrNotFound {
		return err
	}

	return k.removeFromIndex(key)
}

// Clear removes all cache entries from the keyring.
func (k *KeyringBackend) Clear() error {
	k.mu.Lock()
	defer k.mu.Unlock()

	// Get index
	keys := k.getIndex()

	// Delete all entries
	for _, key := range keys {
		keyring.Delete(keyringService, key)
	}

	// Delete index
	keyring.Delete(keyringService, keyringIndexKey)

	k.hitCount = 0
	k.missCount = 0

	return nil
}

// Stats returns cache statistics.
func (k *KeyringBackend) Stats() (*Stats, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()

	keys := k.getIndex()

	return &Stats{
		Backend:    "keyring",
		EntryCount: len(keys),
		HitCount:   k.hitCount,
		MissCount:  k.missCount,
		Size:       -1, // Size not available for keyring
	}, nil
}

// getIndex retrieves the list of cached keys.
func (k *KeyringBackend) getIndex() []string {
	data, err := keyring.Get(keyringService, keyringIndexKey)
	if err != nil {
		return nil
	}

	var keys []string
	json.Unmarshal([]byte(data), &keys)
	return keys
}

// addToIndex adds a key to the index.
func (k *KeyringBackend) addToIndex(key string) error {
	keys := k.getIndex()

	// Check if already exists
	for _, existing := range keys {
		if existing == key {
			return nil
		}
	}

	keys = append(keys, key)
	data, _ := json.Marshal(keys)
	return keyring.Set(keyringService, keyringIndexKey, string(data))
}

// removeFromIndex removes a key from the index.
func (k *KeyringBackend) removeFromIndex(key string) error {
	keys := k.getIndex()

	// Filter out the key
	filtered := make([]string, 0, len(keys))
	for _, existing := range keys {
		if existing != key {
			filtered = append(filtered, existing)
		}
	}

	if len(filtered) == 0 {
		keyring.Delete(keyringService, keyringIndexKey)
		return nil
	}

	data, _ := json.Marshal(filtered)
	return keyring.Set(keyringService, keyringIndexKey, string(data))
}

// IsKeyringAvailable checks if the OS keyring is available.
func IsKeyringAvailable() bool {
	// Try to access the keyring
	_, err := keyring.Get(keyringService, "_test_availability")
	// ErrNotFound means keyring is available but key doesn't exist
	// Any other error means keyring is not available
	return err == nil || err == keyring.ErrNotFound
}

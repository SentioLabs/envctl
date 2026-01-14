package cache

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// FileBackend stores encrypted cache entries in files.
type FileBackend struct {
	dir       string
	key       []byte
	mu        sync.RWMutex
	hitCount  int64
	missCount int64
}

// NewFileBackend creates a new file-based cache backend.
func NewFileBackend(dir string) (*FileBackend, error) {
	// Ensure directory exists with restrictive permissions
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Derive encryption key
	key, err := deriveKey()
	if err != nil {
		return nil, fmt.Errorf("failed to derive encryption key: %w", err)
	}

	return &FileBackend{
		dir: dir,
		key: key,
	}, nil
}

// Name returns the backend name.
func (f *FileBackend) Name() string {
	return "file"
}

// Get retrieves a cached entry.
func (f *FileBackend) Get(key string) (*Entry, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	path := f.entryPath(key)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			f.missCount++
			return nil, nil
		}
		return nil, err
	}

	// Decrypt
	plaintext, err := f.decrypt(data)
	if err != nil {
		// Corrupted or tampered - delete and return nil
		os.Remove(path)
		f.missCount++
		return nil, nil
	}

	// Unmarshal
	var entry Entry
	if err := json.Unmarshal(plaintext, &entry); err != nil {
		os.Remove(path)
		f.missCount++
		return nil, nil
	}

	// Check validity
	if !entry.IsValid() {
		os.Remove(path)
		f.missCount++
		return nil, nil
	}

	f.hitCount++
	return &entry, nil
}

// Set stores a cache entry.
func (f *FileBackend) Set(key string, entry *Entry) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Marshal
	plaintext, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	// Encrypt
	ciphertext, err := f.encrypt(plaintext)
	if err != nil {
		return err
	}

	// Write with restrictive permissions
	path := f.entryPath(key)
	return os.WriteFile(path, ciphertext, 0600)
}

// Delete removes a cache entry.
func (f *FileBackend) Delete(key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	path := f.entryPath(key)
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// Clear removes all cache entries.
func (f *FileBackend) Clear() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	entries, err := os.ReadDir(f.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(f.dir, entry.Name())
		os.Remove(path)
	}

	f.hitCount = 0
	f.missCount = 0

	return nil
}

// Stats returns cache statistics.
func (f *FileBackend) Stats() (*Stats, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	entries, err := os.ReadDir(f.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return &Stats{Backend: "file"}, nil
		}
		return nil, err
	}

	var size int64
	count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err == nil {
			size += info.Size()
			count++
		}
	}

	return &Stats{
		Backend:    "file",
		EntryCount: count,
		HitCount:   f.hitCount,
		MissCount:  f.missCount,
		Size:       size,
	}, nil
}

// entryPath returns the file path for a cache key.
func (f *FileBackend) entryPath(key string) string {
	return filepath.Join(f.dir, key+".enc")
}

// encrypt encrypts data using AES-GCM.
func (f *FileBackend) encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(f.key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	// Prepend nonce to ciphertext
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// decrypt decrypts data using AES-GCM.
func (f *FileBackend) decrypt(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(f.key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// deriveKey derives an encryption key from machine-specific data.
func deriveKey() ([]byte, error) {
	// Combine multiple sources for key derivation
	var keyMaterial string

	// User ID
	keyMaterial += fmt.Sprintf("uid:%d:", os.Getuid())

	// Home directory (stable per-user identifier)
	if home, err := os.UserHomeDir(); err == nil {
		keyMaterial += "home:" + home + ":"
	}

	// Machine ID (Linux)
	if data, err := os.ReadFile("/etc/machine-id"); err == nil {
		keyMaterial += "machine:" + string(data) + ":"
	}

	// Hardware UUID (macOS)
	// Note: This is a fallback; in production you might use IOKit
	if data, err := os.ReadFile("/var/db/dslocal/nodes/Default/users/"); err == nil {
		keyMaterial += "darwin:" + string(data) + ":"
	}

	// Hostname as additional entropy
	if hostname, err := os.Hostname(); err == nil {
		keyMaterial += "host:" + hostname + ":"
	}

	// Add a static salt for envctl
	keyMaterial += "envctl:cache:v1"

	// Derive 32-byte key using SHA256
	hash := sha256.Sum256([]byte(keyMaterial))
	return hash[:], nil
}

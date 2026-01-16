//nolint:testpackage // Testing internal functions requires same package
package cache

import (
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestEntry_IsExpired(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt time.Time
		want      bool
	}{
		{
			name:      "expired - past time",
			expiresAt: time.Now().Add(-1 * time.Hour),
			want:      true,
		},
		{
			name:      "not expired - future time",
			expiresAt: time.Now().Add(1 * time.Hour),
			want:      false,
		},
		{
			name:      "expired - just past",
			expiresAt: time.Now().Add(-1 * time.Millisecond),
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := &Entry{
				ExpiresAt: tt.expiresAt,
				Version:   CacheVersion,
			}
			assert.Equal(t, tt.want, entry.IsExpired())
		})
	}
}

func TestEntry_IsValid(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt time.Time
		version   int
		want      bool
	}{
		{
			name:      "valid - not expired and correct version",
			expiresAt: time.Now().Add(1 * time.Hour),
			version:   CacheVersion,
			want:      true,
		},
		{
			name:      "invalid - expired",
			expiresAt: time.Now().Add(-1 * time.Hour),
			version:   CacheVersion,
			want:      false,
		},
		{
			name:      "invalid - wrong version",
			expiresAt: time.Now().Add(1 * time.Hour),
			version:   CacheVersion - 1,
			want:      false,
		},
		{
			name:      "invalid - expired and wrong version",
			expiresAt: time.Now().Add(-1 * time.Hour),
			version:   CacheVersion - 1,
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := &Entry{
				ExpiresAt: tt.expiresAt,
				Version:   tt.version,
			}
			assert.Equal(t, tt.want, entry.IsValid())
		})
	}
}

func TestKey(t *testing.T) {
	tests := []struct {
		name       string
		region     string
		secretName string
	}{
		{
			name:       "basic key",
			region:     "us-east-1",
			secretName: "my-app/dev",
		},
		{
			name:       "different region",
			region:     "eu-west-1",
			secretName: "my-app/dev",
		},
		{
			name:       "different secret",
			region:     "us-east-1",
			secretName: "other-app/prod",
		},
		{
			name:       "special characters in secret",
			region:     "us-east-1",
			secretName: "my/app/with/slashes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := Key(tt.region, tt.secretName)

			// Key should be 32 hex characters (16 bytes)
			assert.Len(t, key, 32)

			// Key should be deterministic
			key2 := Key(tt.region, tt.secretName)
			assert.Equal(t, key, key2)

			// Key should contain only hex characters
			for _, c := range key {
				assert.True(t, (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f'),
					"key should only contain hex characters")
			}
		})
	}

	// Different inputs should produce different keys
	key1 := Key("us-east-1", "secret1")
	key2 := Key("us-east-1", "secret2")
	key3 := Key("us-west-2", "secret1")
	assert.NotEqual(t, key1, key2, "different secrets should have different keys")
	assert.NotEqual(t, key1, key3, "different regions should have different keys")
}

func TestGetCacheDir(t *testing.T) {
	tests := []struct {
		name         string
		xdgCacheHome string
		wantSuffix   string
	}{
		{
			name:         "with XDG_CACHE_HOME",
			xdgCacheHome: "/custom/cache",
			wantSuffix:   "/custom/cache/envctl",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("XDG_CACHE_HOME", tt.xdgCacheHome)
			got := GetCacheDir()
			assert.Equal(t, tt.wantSuffix, got)
		})
	}

	// Test default paths (without XDG_CACHE_HOME)
	t.Run("default path", func(t *testing.T) {
		t.Setenv("XDG_CACHE_HOME", "")
		got := GetCacheDir()

		home, _ := os.UserHomeDir()
		if runtime.GOOS == "darwin" {
			assert.Equal(t, home+"/Library/Caches/envctl", got)
		} else {
			assert.Equal(t, home+"/.cache/envctl", got)
		}
	})
}

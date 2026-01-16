// Package secrets provides a common interface for secret backends.
package secrets

import (
	"context"
)

// Client defines the interface for secret backends.
// Implementations include AWS Secrets Manager, 1Password, and others.
type Client interface {
	// GetSecret retrieves all key-value pairs from a secret.
	// For AWS: returns all keys from the JSON secret.
	// For 1Password: returns all fields from an item.
	GetSecret(ctx context.Context, secretRef string) (map[string]string, error)

	// GetSecretKey retrieves a specific key from a secret.
	GetSecretKey(ctx context.Context, secretRef, key string) (string, error)

	// Name returns the backend name (e.g., "aws", "1password").
	Name() string
}

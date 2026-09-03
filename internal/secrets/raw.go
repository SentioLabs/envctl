package secrets

import "context"

// RawReader is an optional interface for backends that can return a secret's
// content verbatim, without JSON parsing or whitespace trimming. File sinks
// without key: require it. Discover it with a type assertion on a Client.
type RawReader interface {
	GetSecretRaw(ctx context.Context, secretRef string) ([]byte, error)
}

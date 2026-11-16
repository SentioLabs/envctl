package secrets

import (
	"context"
	"maps"
	"sync"

	"github.com/sentiolabs/envctl/internal/config"
	envctlerrors "github.com/sentiolabs/envctl/internal/errors"
)

// Memo remembers what clients read during one envctl invocation, so a secret
// that several applications include reaches the backend once. Entries live in
// process memory only and never expire. Errors are not remembered.
//
// A read is served from the memo only for the exact call that produced it:
// GetSecret(ref) answers GetSecret(ref), and GetSecretKey(ref, key) answers
// GetSecretKey(ref, key). A whole secret never answers a keyed read, because
// backends resolve a key differently than they label a whole secret's fields.
type Memo struct {
	mu      sync.Mutex
	secrets map[memoKey]map[string]string
	keys    map[memoKey]string
}

// memoKey scopes a read to the backend settings that resolved it, so the
// same reference in two AWS regions or two 1Password vaults stays distinct.
type memoKey struct {
	scope string
	ref   string
	key   string
}

// NewMemo returns an empty Memo.
func NewMemo() *Memo {
	return &Memo{
		secrets: make(map[memoKey]map[string]string),
		keys:    make(map[memoKey]string),
	}
}

// Scope names the backend settings env resolves to under cfg. Clients that
// share a scope read the same secrets, so they can share memo entries.
func Scope(cfg *config.Config, env *config.Environment) string {
	if cfg == nil {
		return config.BackendAWS
	}
	backend := cfg.ResolveBackend(env)
	if backend == config.Backend1Pass {
		op := cfg.ResolveOnePassConfig(env)
		return backend + "|" + op.Account + "|" + op.Vault
	}
	aws := cfg.ResolveAWSConfig(env)
	return backend + "|" + aws.Region + "|" + aws.Profile
}

// Wrap returns client with its reads memoized under scope. A nil Memo returns
// client unchanged, so callers can wrap without checking.
func (m *Memo) Wrap(client Client, scope string) Client {
	if m == nil {
		return client
	}
	return &memoClient{inner: client, memo: m, scope: scope}
}

// memoClient serves repeated reads from its Memo.
type memoClient struct {
	inner Client
	memo  *Memo
	scope string
}

var (
	_ Client    = (*memoClient)(nil)
	_ RawReader = (*memoClient)(nil)
)

// Name returns the wrapped backend's name.
func (c *memoClient) Name() string { return c.inner.Name() }

// GetSecret returns a copy, so a caller that edits the map can't change what
// the next caller gets.
func (c *memoClient) GetSecret(ctx context.Context, secretRef string) (map[string]string, error) {
	k := memoKey{scope: c.scope, ref: secretRef}
	c.memo.mu.Lock()
	cached, ok := c.memo.secrets[k]
	c.memo.mu.Unlock()
	if ok {
		return maps.Clone(cached), nil
	}

	values, err := c.inner.GetSecret(ctx, secretRef)
	if err != nil {
		return nil, err
	}
	c.memo.mu.Lock()
	c.memo.secrets[k] = maps.Clone(values)
	c.memo.mu.Unlock()
	return values, nil
}

func (c *memoClient) GetSecretKey(ctx context.Context, secretRef, key string) (string, error) {
	k := memoKey{scope: c.scope, ref: secretRef, key: key}
	c.memo.mu.Lock()
	cached, ok := c.memo.keys[k]
	c.memo.mu.Unlock()
	if ok {
		return cached, nil
	}

	value, err := c.inner.GetSecretKey(ctx, secretRef, key)
	if err != nil {
		return "", err
	}
	c.memo.mu.Lock()
	c.memo.keys[k] = value
	c.memo.mu.Unlock()
	return value, nil
}

// GetSecretRaw passes through unmemoized. The wrapper always satisfies
// RawReader, so a backend without raw reads gets the error the builder would
// have returned on its own.
func (c *memoClient) GetSecretRaw(ctx context.Context, secretRef string) ([]byte, error) {
	raw, ok := c.inner.(RawReader)
	if !ok {
		return nil, &envctlerrors.FileSinkKeyRequiredError{Secret: secretRef, Backend: c.inner.Name()}
	}
	return raw.GetSecretRaw(ctx, secretRef)
}

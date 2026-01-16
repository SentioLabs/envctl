package onepassword

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/sentiolabs/envctl/internal/errors"
)

// Client provides access to 1Password via the op CLI.
// It implements the secrets.Client interface.
type Client struct {
	defaultVault string // Default vault to use when not specified in reference
	account      string // Account shorthand for multi-account setups
}

// ClientOptions configures the 1Password client.
type ClientOptions struct {
	DefaultVault string // Default vault name or ID
	Account      string // Account shorthand (optional)
}

// NewClient creates a new 1Password client.
func NewClient(opts ClientOptions) (*Client, error) {
	// Verify op CLI is available
	if err := checkCLI(); err != nil {
		return nil, err
	}

	return &Client{
		defaultVault: opts.DefaultVault,
		account:      opts.Account,
	}, nil
}

// Name returns the backend name.
func (c *Client) Name() string {
	return "1password"
}

// GetSecret retrieves all key-value pairs from a 1Password item.
// secretRef can be:
//   - item name (uses default vault)
//   - vault/item
//   - op://vault/item
func (c *Client) GetSecret(ctx context.Context, secretRef string) (map[string]string, error) {
	ref, err := ParseReference(secretRef)
	if err != nil {
		return nil, &errors.SecretNotFoundError{SecretName: secretRef}
	}

	// Use default vault if not specified
	if ref.Vault == "" {
		ref.Vault = c.defaultVault
	}

	// Fetch the item
	item, err := c.getItem(ctx, ref)
	if err != nil {
		return nil, err
	}

	// Convert fields to map
	filter := DefaultFieldFilter()
	return item.ToMap(filter), nil
}

// GetSecretKey retrieves a specific field from a 1Password item.
func (c *Client) GetSecretKey(ctx context.Context, secretRef, key string) (string, error) {
	ref, err := ParseReference(secretRef)
	if err != nil {
		return "", &errors.SecretNotFoundError{SecretName: secretRef}
	}

	// Use default vault if not specified
	if ref.Vault == "" {
		ref.Vault = c.defaultVault
	}

	// If we have a field specified in the ref, use that instead of key parameter
	fieldName := key
	if ref.Field != "" {
		fieldName = ref.Field
	}

	// Try to read the field directly using op read
	fieldRef := &Reference{
		Vault: ref.Vault,
		Item:  ref.Item,
		Field: fieldName,
	}

	value, err := c.readField(ctx, fieldRef)
	if err != nil {
		// Fall back to getting the full item
		item, itemErr := c.getItem(ctx, ref)
		if itemErr != nil {
			return "", itemErr
		}

		value = item.GetField(fieldName)
		if value == "" {
			// Collect available field names
			var fields []string
			for _, f := range item.Fields {
				if f.Label != "" {
					fields = append(fields, f.Label)
				}
			}
			return "", &errors.KeyNotFoundError{
				SecretName:    secretRef,
				Key:           fieldName,
				AvailableKeys: fields,
			}
		}
	}

	return value, nil
}

// getItem fetches a full 1Password item.
func (c *Client) getItem(ctx context.Context, ref *Reference) (*Item, error) {
	args := ref.CLIArgs()
	if c.account != "" {
		args = append(args, "--account", c.account)
	}

	output, err := c.runOP(ctx, args...)
	if err != nil {
		return nil, c.mapError(ref.ItemRef(), err)
	}

	var item Item
	if err := json.Unmarshal(output, &item); err != nil {
		return nil, &errors.InvalidSecretFormatError{SecretName: ref.ItemRef()}
	}

	return &item, nil
}

// readField reads a single field value using op read.
func (c *Client) readField(ctx context.Context, ref *Reference) (string, error) {
	args := []string{"read", ref.String()}
	if c.account != "" {
		args = append(args, "--account", c.account)
	}

	output, err := c.runOP(ctx, args...)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

// runOP executes the op CLI with the given arguments.
func (c *Client) runOP(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "op", args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		stderrStr := stderr.String()
		if strings.Contains(stderrStr, "not signed in") ||
			strings.Contains(stderrStr, "session expired") ||
			strings.Contains(stderrStr, "biometric") {
			return nil, &errors.CredentialsError{
				Message: "1Password authentication required. Please unlock 1Password or run 'op signin'",
			}
		}
		return nil, fmt.Errorf("op command failed: %s", stderrStr)
	}

	return stdout.Bytes(), nil
}

// mapError converts op CLI errors to envctl error types.
func (c *Client) mapError(secretRef string, err error) error {
	errStr := err.Error()

	if strings.Contains(errStr, "isn't an item") ||
		strings.Contains(errStr, "not found") ||
		strings.Contains(errStr, "doesn't exist") {
		return &errors.SecretNotFoundError{SecretName: secretRef}
	}

	if strings.Contains(errStr, "not signed in") ||
		strings.Contains(errStr, "session expired") {
		return &errors.CredentialsError{
			Message: "1Password authentication required. Please unlock 1Password or run 'op signin'",
		}
	}

	if strings.Contains(errStr, "access denied") ||
		strings.Contains(errStr, "permission") {
		return &errors.AccessDeniedError{SecretName: secretRef}
	}

	return err
}

// checkCLI verifies the op CLI is installed and accessible.
func checkCLI() error {
	_, err := exec.LookPath("op")
	if err != nil {
		return &errors.CredentialsError{
			Message: "1Password CLI (op) not found. Please install it: https://developer.1password.com/docs/cli/get-started/",
		}
	}
	return nil
}

// CheckAuth verifies 1Password authentication status.
// Returns nil if authenticated, error otherwise.
func (c *Client) CheckAuth(ctx context.Context) error {
	// Try to list vaults as a quick auth check
	_, err := c.runOP(ctx, "vault", "list", "--format", "json")
	return err
}

// ListVaults returns available vaults.
func (c *Client) ListVaults(ctx context.Context) ([]VaultRef, error) {
	output, err := c.runOP(ctx, "vault", "list", "--format", "json")
	if err != nil {
		return nil, err
	}

	var vaults []VaultRef
	if err := json.Unmarshal(output, &vaults); err != nil {
		return nil, fmt.Errorf("failed to parse vault list: %w", err)
	}

	return vaults, nil
}

package onepassword

import (
	"errors"
	"fmt"
	"strings"
)

// Reference part counts.
const (
	partsItem            = 1 // Just item name
	partsVaultItem       = 2 // vault/item
	partsVaultItemField  = 3 // vault/item/field
	partsVaultItemSecFld = 4 // vault/item/section/field
)

// errEmptyReference is returned when the reference string is empty.
var errEmptyReference = errors.New("empty reference")

// Reference represents a parsed op:// secret reference.
// Format: op://vault/item[/section]/field
// Or simplified: vault/item or just item (uses default vault)
type Reference struct {
	Vault   string // Vault name or ID
	Item    string // Item name or ID
	Section string // Optional section within item
	Field   string // Optional specific field
}

// ParseReference parses a 1Password secret reference.
// Supported formats:
//   - op://vault/item           - full item reference
//   - op://vault/item/field     - specific field
//   - op://vault/item/section/field - field in section
//   - vault/item                - short form (no op:// prefix)
//   - item                      - just item name (uses default vault)
func ParseReference(ref string) (*Reference, error) {
	// Strip op:// prefix if present
	ref = strings.TrimPrefix(ref, "op://")

	parts := strings.Split(ref, "/")

	switch len(parts) {
	case partsItem:
		// Just item name, no vault specified
		if parts[0] == "" {
			return nil, errEmptyReference
		}
		return &Reference{
			Item: parts[0],
		}, nil

	case partsVaultItem:
		// vault/item
		if parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("invalid reference format: %s", ref)
		}
		return &Reference{
			Vault: parts[0],
			Item:  parts[1],
		}, nil

	case partsVaultItemField:
		// vault/item/field
		if parts[0] == "" || parts[1] == "" || parts[2] == "" {
			return nil, fmt.Errorf("invalid reference format: %s", ref)
		}
		return &Reference{
			Vault: parts[0],
			Item:  parts[1],
			Field: parts[2],
		}, nil

	case partsVaultItemSecFld:
		// vault/item/section/field
		if parts[0] == "" || parts[1] == "" || parts[2] == "" || parts[3] == "" {
			return nil, fmt.Errorf("invalid reference format: %s", ref)
		}
		return &Reference{
			Vault:   parts[0],
			Item:    parts[1],
			Section: parts[2],
			Field:   parts[3],
		}, nil

	default:
		return nil, fmt.Errorf(
			"invalid reference format: %s (expected vault/item[/field] or op://vault/item[/field])", ref)
	}
}

// String returns the canonical op:// format for this reference.
// The strings.Builder Write methods never return errors, so we ignore them.
//
//nolint:revive // strings.Builder Write methods always return nil error
func (r *Reference) String() string {
	var sb strings.Builder
	sb.WriteString("op://")

	if r.Vault != "" {
		sb.WriteString(r.Vault)
		sb.WriteString("/")
	}

	sb.WriteString(r.Item)

	if r.Section != "" {
		sb.WriteString("/")
		sb.WriteString(r.Section)
	}

	if r.Field != "" {
		sb.WriteString("/")
		sb.WriteString(r.Field)
	}

	return sb.String()
}

// HasField returns true if this reference specifies a field.
func (r *Reference) HasField() bool {
	return r.Field != ""
}

// ItemRef returns the op:// reference for just the item (no field).
// This is useful for fetching the full item when we need a specific field.
func (r *Reference) ItemRef() string {
	if r.Vault == "" {
		return r.Item
	}
	return r.Vault + "/" + r.Item
}

// CLIArgs returns arguments for the op CLI to read this reference.
// For items: ["item", "get", "item-name", "--vault", "vault-name"]
// For fields: ["read", "op://vault/item/field"]
func (r *Reference) CLIArgs() []string {
	if r.HasField() {
		// Use 'op read' for specific fields
		return []string{"read", r.String()}
	}

	// Use 'op item get' for full items
	args := []string{"item", "get", r.Item, "--format", "json"}
	if r.Vault != "" {
		args = append(args, "--vault", r.Vault)
	}
	return args
}

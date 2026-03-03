// Package onepassword provides 1Password integration via the op CLI.
package onepassword

import "slices"

// Item represents a 1Password item as returned by the CLI.
// This is a simplified version focusing on what we need for secret retrieval.
type Item struct {
	ID       string    `json:"id"`
	Title    string    `json:"title"`
	Category string    `json:"category"`
	Vault    VaultRef  `json:"vault"`
	Fields   []Field   `json:"fields"`
	Sections []Section `json:"sections,omitzero"`
}

// VaultRef is a reference to a vault.
type VaultRef struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

// Section is a grouping of fields within an item.
type Section struct {
	ID    string `json:"id"`
	Label string `json:"label,omitempty"`
}

// Field represents a single field in a 1Password item.
type Field struct {
	ID        string `json:"id"`
	Type      string `json:"type"`      // "CONCEALED", "STRING", "URL", "EMAIL", etc.
	Purpose   string `json:"purpose"`   // "USERNAME", "PASSWORD", "NOTES", etc.
	Label     string `json:"label"`     // Display name
	Value     string `json:"value"`     // The actual value
	Reference string `json:"reference"` // op:// reference
	Section   *struct {
		ID    string `json:"id"`
		Label string `json:"label,omitempty"`
	} `json:"section,omitzero"`
}

// FieldFilter defines which fields to include when extracting key-value pairs.
type FieldFilter struct {
	// IncludeConcealed includes fields with type "CONCEALED" (passwords, secrets).
	IncludeConcealed bool
	// IncludeStrings includes fields with type "STRING" (text fields).
	IncludeStrings bool
	// IncludeOther includes other field types (URL, EMAIL, etc.).
	IncludeOther bool
	// ExcludePurposes excludes fields with these purposes (e.g., "NOTES").
	ExcludePurposes []string
}

// DefaultFieldFilter returns a filter suitable for environment variables.
// Includes concealed and string fields, excludes notes.
func DefaultFieldFilter() FieldFilter {
	return FieldFilter{
		IncludeConcealed: true,
		IncludeStrings:   true,
		IncludeOther:     true,
		ExcludePurposes:  []string{"NOTES"},
	}
}

// ToMap extracts key-value pairs from an item's fields.
// The key is the field label (normalized for env vars).
// Returns only non-empty values that pass the filter.
func (item *Item) ToMap(filter FieldFilter) map[string]string {
	result := make(map[string]string)

	for _, field := range item.Fields {
		// Skip empty values
		if field.Value == "" {
			continue
		}

		// Skip excluded purposes
		if slices.Contains(filter.ExcludePurposes, field.Purpose) {
			continue
		}

		// Apply type filter
		switch field.Type {
		case "CONCEALED":
			if !filter.IncludeConcealed {
				continue
			}
		case "STRING":
			if !filter.IncludeStrings {
				continue
			}
		default:
			if !filter.IncludeOther {
				continue
			}
		}

		// Use label as key, skip fields without labels
		if field.Label == "" {
			continue
		}

		result[field.Label] = field.Value
	}

	return result
}

// GetField returns the value of a field by label.
// Returns empty string if not found.
func (item *Item) GetField(label string) string {
	for _, field := range item.Fields {
		if field.Label == label {
			return field.Value
		}
	}
	return ""
}

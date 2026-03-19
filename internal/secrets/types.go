package secrets

// Vault represents a secret vault or grouping (e.g., 1Password vault, AWS path prefix).
type Vault struct {
	ID   string
	Name string
}

// Item represents a secret item within a vault.
type Item struct {
	ID    string
	Name  string
	Vault string
}

// Field represents a single key-value field within a secret item.
type Field struct {
	ID      string    // backend-specific field ID
	Key     string
	Value   string
	Type    FieldType
	Section string // 1Password sections (empty for AWS)
}

// FieldType indicates whether a field's value is visible or hidden.
type FieldType string

const (
	FieldText      FieldType = "text"
	FieldConcealed FieldType = "concealed"
)

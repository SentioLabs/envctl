package secrets

import "context"

// Editor extends Client with write operations for managing secrets.
// Backends implement this to support the TUI editor.
// Methods accept Field structs (rather than bare key strings) so that
// backends can use Section and ID for disambiguation when needed
// (e.g., 1Password items with duplicate field labels across sections).
type Editor interface {
	Client
	ListVaults(ctx context.Context) ([]Vault, error)
	ListItems(ctx context.Context, vault string) ([]Item, error)
	GetFields(ctx context.Context, ref string) ([]Field, error)
	UpdateField(ctx context.Context, ref string, field Field) error
	DeleteField(ctx context.Context, ref string, field Field) error
	RenameField(ctx context.Context, ref string, field Field, newKey string) error
	CreateItem(ctx context.Context, vault string, name string, fields []Field) error
}

// FieldTypeEditor is an optional interface for backends that support
// changing field visibility (e.g., 1Password concealed <-> text).
type FieldTypeEditor interface {
	SetFieldType(ctx context.Context, ref string, field Field, ft FieldType) error
}

// Change represents a pending modification to apply via BatchSaver.
type Change struct {
	Type    string    // "update", "delete", "rename", "set_type"
	Field   Field     // the field being changed
	OldKey  string    // original key (for rename)
	NewType FieldType // target type (for set_type)
}

// BatchSaver is an optional interface for backends that can apply multiple
// changes in fewer operations than one-at-a-time. For example, 1Password
// supports multiple assignments in a single `op item edit` CLI call.
type BatchSaver interface {
	BatchSave(ctx context.Context, ref string, changes []Change) error
}

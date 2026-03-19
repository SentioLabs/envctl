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

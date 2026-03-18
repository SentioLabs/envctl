package secrets

import "context"

// Editor extends Client with write operations for managing secrets.
// Backends implement this to support the TUI editor.
type Editor interface {
	Client
	ListVaults(ctx context.Context) ([]Vault, error)
	ListItems(ctx context.Context, vault string) ([]Item, error)
	GetFields(ctx context.Context, ref string) ([]Field, error)
	UpdateField(ctx context.Context, ref string, field Field) error
	DeleteField(ctx context.Context, ref string, key string) error
	RenameField(ctx context.Context, ref string, oldKey string, newKey string) error
	CreateItem(ctx context.Context, vault string, name string, fields []Field) error
}

// FieldTypeEditor is an optional interface for backends that support
// changing field visibility (e.g., 1Password concealed <-> text).
type FieldTypeEditor interface {
	SetFieldType(ctx context.Context, ref string, key string, ft FieldType) error
}

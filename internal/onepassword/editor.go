package onepassword

import (
	"context"
	"encoding/json"
	"fmt"
)

// commandRunner is a function type for executing op CLI commands.
// It allows injecting a mock for testing.
type commandRunner func(ctx context.Context, args ...string) ([]byte, error)

// OPEditor wraps the 1Password Client to provide write operations.
type OPEditor struct {
	*Client
	runCmd commandRunner
}

// NewEditor creates a new 1Password editor.
func NewEditor(opts ClientOptions) (*OPEditor, error) {
	if err := checkCLI(); err != nil {
		return nil, err
	}

	client := &Client{
		defaultVault: opts.DefaultVault,
		account:      opts.Account,
	}
	editor := &OPEditor{
		Client: client,
	}
	// Default to the real Client.runOP method.
	editor.runCmd = client.runOP
	return editor, nil
}

// FieldPair holds a key-value pair for creating items.
type FieldPair struct {
	Key   string
	Value string
}

// EditorVault represents a vault returned by the editor.
type EditorVault struct {
	ID   string
	Name string
}

// EditorItem represents an item returned by the editor.
type EditorItem struct {
	ID    string
	Name  string
	Vault string
}

// FieldType indicates whether a field's value is visible or hidden.
type FieldType string

const (
	// FieldText indicates a visible text field.
	FieldText FieldType = "text"
	// FieldConcealed indicates a hidden/password field.
	FieldConcealed FieldType = "concealed"
)

// EditorField represents a field returned by the editor.
type EditorField struct {
	ID      string
	Key     string
	Value   string
	Type    FieldType
	Section string
}

// ListEditorVaults returns available 1Password vaults.
func (e *OPEditor) ListEditorVaults(ctx context.Context) ([]EditorVault, error) {
	args := []string{"vault", "list", "--format", "json"}
	if e.account != "" {
		args = append(args, "--account", e.account)
	}

	output, err := e.runCmd(ctx, args...)
	if err != nil {
		return nil, err
	}

	var vaultRefs []VaultRef
	if err := json.Unmarshal(output, &vaultRefs); err != nil {
		return nil, fmt.Errorf("failed to parse vault list: %w", err)
	}

	result := make([]EditorVault, len(vaultRefs))
	for i, v := range vaultRefs {
		result[i] = EditorVault(v)
	}
	return result, nil
}

// opItemListEntry represents a single item in the op item list JSON output.
type opItemListEntry struct {
	ID    string   `json:"id"`
	Title string   `json:"title"`
	Vault VaultRef `json:"vault"`
}

// ListEditorItems returns items in a vault.
func (e *OPEditor) ListEditorItems(ctx context.Context, vault string) ([]EditorItem, error) {
	args := []string{"item", "list", "--vault", vault, "--format", "json"}
	if e.account != "" {
		args = append(args, "--account", e.account)
	}

	output, err := e.runCmd(ctx, args...)
	if err != nil {
		return nil, err
	}

	var entries []opItemListEntry
	if err := json.Unmarshal(output, &entries); err != nil {
		return nil, fmt.Errorf("failed to parse item list: %w", err)
	}

	result := make([]EditorItem, len(entries))
	for i, entry := range entries {
		result[i] = EditorItem{
			ID:    entry.ID,
			Name:  entry.Title,
			Vault: entry.Vault.Name,
		}
	}
	return result, nil
}

// GetEditorFields returns user-created fields for a 1Password item.
// Skips structural fields (those with a Purpose like "NOTES", "USERNAME", "PASSWORD")
// and fields without labels, matching the read path's DefaultFieldFilter behavior.
func (e *OPEditor) GetEditorFields(ctx context.Context, ref string) ([]EditorField, error) {
	item, err := e.fetchItem(ctx, ref)
	if err != nil {
		return nil, err
	}

	var result []EditorField
	for _, f := range item.Fields {
		// Skip structural fields (notesPlain, username, password)
		if f.Purpose != "" {
			continue
		}
		// Skip fields without labels
		if f.Label == "" {
			continue
		}
		ef := EditorField{
			ID:    f.ID,
			Key:   f.Label,
			Value: f.Value,
			Type:  mapFieldType(f.Type),
		}
		if f.Section != nil {
			ef.Section = f.Section.Label
		}
		result = append(result, ef)
	}
	return result, nil
}

// mapFieldType converts 1Password field type strings to FieldType.
func mapFieldType(opType string) FieldType {
	switch opType {
	case "CONCEALED":
		return FieldConcealed
	default:
		return FieldText
	}
}

// UpdateField updates a single field on a 1Password item.
// When section is non-empty, uses "section.key=value" syntax to disambiguate
// duplicate field labels across sections.
func (e *OPEditor) UpdateField(ctx context.Context, ref string, key, value, section string) error {
	parsedRef, err := ParseReference(ref)
	if err != nil {
		return fmt.Errorf("invalid reference %q: %w", ref, err)
	}
	if parsedRef.Vault == "" {
		parsedRef.Vault = e.defaultVault
	}

	fieldRef := key
	if section != "" {
		fieldRef = section + "." + key
	}
	assignment := fmt.Sprintf("%s=%s", fieldRef, value)
	args := []string{"item", "edit", parsedRef.Item, assignment, "--vault", parsedRef.Vault}
	if e.account != "" {
		args = append(args, "--account", e.account)
	}

	_, err = e.runCmd(ctx, args...)
	return err
}

// DeleteField removes a field from a 1Password item.
// When section is non-empty, uses "section.key[delete]" to disambiguate.
func (e *OPEditor) DeleteField(ctx context.Context, ref, key, section string) error {
	parsedRef, err := ParseReference(ref)
	if err != nil {
		return fmt.Errorf("invalid reference %q: %w", ref, err)
	}
	if parsedRef.Vault == "" {
		parsedRef.Vault = e.defaultVault
	}

	fieldRef := key
	if section != "" {
		fieldRef = section + "." + key
	}
	deleteExpr := fieldRef + "[delete]"
	args := []string{"item", "edit", parsedRef.Item, deleteExpr, "--vault", parsedRef.Vault}
	if e.account != "" {
		args = append(args, "--account", e.account)
	}

	_, err = e.runCmd(ctx, args...)
	return err
}

// RenameField renames a field by deleting the old key and creating a new one.
// When section is non-empty, uses section-qualified references.
// This is a non-atomic operation.
func (e *OPEditor) RenameField(ctx context.Context, ref, oldKey, newKey, section string) error {
	// Get the current value of the field
	fields, err := e.GetEditorFields(ctx, ref)
	if err != nil {
		return fmt.Errorf("failed to get fields for rename: %w", err)
	}

	var found bool
	var value string
	for _, f := range fields {
		if f.Key == oldKey {
			found = true
			value = f.Value
			break
		}
	}
	if !found {
		return fmt.Errorf("field %q not found in %s", oldKey, ref)
	}

	// Delete the old field
	if err := e.DeleteField(ctx, ref, oldKey, section); err != nil {
		return fmt.Errorf("failed to delete old field %q: %w", oldKey, err)
	}

	// Create the new field with the same value
	if err := e.UpdateField(ctx, ref, newKey, value, section); err != nil {
		return fmt.Errorf("failed to create new field %q: %w", newKey, err)
	}

	return nil
}

// CreateEditorItem creates a new 1Password item with the given fields.
func (e *OPEditor) CreateEditorItem(ctx context.Context, vault, name string, fields []FieldPair) error {
	args := []string{"item", "create", "--vault", vault, "--title", name, "--category", "SecureNote"}
	for _, f := range fields {
		args = append(args, fmt.Sprintf("%s=%s", f.Key, f.Value))
	}
	if e.account != "" {
		args = append(args, "--account", e.account)
	}

	_, err := e.runCmd(ctx, args...)
	return err
}

// SetEditorFieldType changes a field's type between concealed and text.
// When section is non-empty, uses section-qualified references.
func (e *OPEditor) SetEditorFieldType(ctx context.Context, ref, key, section string, ft FieldType) error {
	// Get current value
	fields, err := e.GetEditorFields(ctx, ref)
	if err != nil {
		return fmt.Errorf("failed to get fields for type change: %w", err)
	}

	var found bool
	var value string
	for _, f := range fields {
		if f.Key == key {
			found = true
			value = f.Value
			break
		}
	}
	if !found {
		return fmt.Errorf("field %q not found in %s", key, ref)
	}

	// Map field type to op CLI type label
	opType := "text"
	if ft == FieldConcealed {
		opType = "password"
	}

	parsedRef, err := ParseReference(ref)
	if err != nil {
		return fmt.Errorf("invalid reference %q: %w", ref, err)
	}
	if parsedRef.Vault == "" {
		parsedRef.Vault = e.defaultVault
	}

	fieldRef := key
	if section != "" {
		fieldRef = section + "." + key
	}
	typeExpr := fmt.Sprintf("%s[%s]=%s", fieldRef, opType, value)
	args := []string{"item", "edit", parsedRef.Item, typeExpr, "--vault", parsedRef.Vault}
	if e.account != "" {
		args = append(args, "--account", e.account)
	}

	_, err = e.runCmd(ctx, args...)
	return err
}

// fetchItem retrieves a 1Password item using the mock-able runCmd.
func (e *OPEditor) fetchItem(ctx context.Context, ref string) (*Item, error) {
	parsedRef, err := ParseReference(ref)
	if err != nil {
		return nil, fmt.Errorf("invalid reference %q: %w", ref, err)
	}
	if parsedRef.Vault == "" {
		parsedRef.Vault = e.defaultVault
	}

	args := parsedRef.CLIArgs()
	if e.account != "" {
		args = append(args, "--account", e.account)
	}

	output, err := e.runCmd(ctx, args...)
	if err != nil {
		return nil, err
	}

	var item Item
	if err := json.Unmarshal(output, &item); err != nil {
		return nil, fmt.Errorf("failed to parse item: %w", err)
	}

	return &item, nil
}

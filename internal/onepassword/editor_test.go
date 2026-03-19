//nolint:testpackage // Testing internal functions requires same package
package onepassword

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRunner captures the args passed to runCmd and returns canned output.
type mockRunner struct {
	calls  [][]string
	output []byte
	err    error
}

func (m *mockRunner) run(_ context.Context, args ...string) ([]byte, error) {
	m.calls = append(m.calls, args)
	return m.output, m.err
}

// newTestEditor creates an OPEditor with a mock command runner.
func newTestEditor(m *mockRunner) *OPEditor {
	client := &Client{
		defaultVault: "TestVault",
		account:      "test-account",
	}
	editor := &OPEditor{
		Client: client,
		runCmd: m.run,
	}
	return editor
}

func TestListVaults(t *testing.T) {
	vaults := []VaultRef{
		{ID: "abc123", Name: "Personal"},
		{ID: "def456", Name: "Shared"},
	}
	data, err := json.Marshal(vaults)
	require.NoError(t, err)

	m := &mockRunner{output: data}
	editor := newTestEditor(m)

	result, err := editor.ListEditorVaults(context.Background())
	require.NoError(t, err)

	assert.Len(t, result, 2)
	assert.Equal(t, "abc123", result[0].ID)
	assert.Equal(t, "Personal", result[0].Name)
	assert.Equal(t, "def456", result[1].ID)
	assert.Equal(t, "Shared", result[1].Name)

	// Verify correct op command was issued
	require.Len(t, m.calls, 1)
	assert.Equal(t, []string{"vault", "list", "--format", "json", "--account", "test-account"}, m.calls[0])
}

func TestListItems(t *testing.T) {
	type opItem struct {
		ID    string   `json:"id"`
		Title string   `json:"title"`
		Vault VaultRef `json:"vault"`
	}
	items := []opItem{
		{ID: "item1", Title: "DB Creds", Vault: VaultRef{ID: "v1", Name: "MyVault"}},
		{ID: "item2", Title: "API Keys", Vault: VaultRef{ID: "v1", Name: "MyVault"}},
	}
	data, err := json.Marshal(items)
	require.NoError(t, err)

	m := &mockRunner{output: data}
	editor := newTestEditor(m)

	result, err := editor.ListEditorItems(context.Background(), "MyVault")
	require.NoError(t, err)

	assert.Len(t, result, 2)
	assert.Equal(t, "item1", result[0].ID)
	assert.Equal(t, "DB Creds", result[0].Name)
	assert.Equal(t, "MyVault", result[0].Vault)
	assert.Equal(t, "item2", result[1].ID)
	assert.Equal(t, "API Keys", result[1].Name)

	require.Len(t, m.calls, 1)
	expectedArgs := []string{
		"item", "list", "--vault", "MyVault",
		"--format", "json", "--account", "test-account",
	}
	assert.Equal(t, expectedArgs, m.calls[0])
}

func TestGetFields(t *testing.T) {
	item := Item{
		ID:    "item1",
		Title: "Test Item",
		Fields: []Field{
			{ID: "f1", Label: "username", Value: "admin", Type: "STRING"},
			{ID: "f2", Label: "password", Value: "secret", Type: "CONCEALED"},
			{ID: "f3", Label: "url", Value: "https://example.com", Type: "URL"},
		},
	}
	data, err := json.Marshal(item)
	require.NoError(t, err)

	m := &mockRunner{output: data}
	editor := newTestEditor(m)

	result, err := editor.GetEditorFields(context.Background(), "TestVault/item1")
	require.NoError(t, err)

	assert.Len(t, result, 3)

	assert.Equal(t, "f1", result[0].ID)
	assert.Equal(t, "username", result[0].Key)
	assert.Equal(t, "admin", result[0].Value)
	assert.Equal(t, FieldText, result[0].Type)

	assert.Equal(t, "f2", result[1].ID)
	assert.Equal(t, "password", result[1].Key)
	assert.Equal(t, "secret", result[1].Value)
	assert.Equal(t, FieldConcealed, result[1].Type)

	// URL and other types map to text
	assert.Equal(t, FieldText, result[2].Type)
}

func TestGetFields_WithSection(t *testing.T) {
	item := Item{
		ID:    "item1",
		Title: "Test Item",
		Fields: []Field{
			{
				ID: "f1", Label: "db_host", Value: "localhost", Type: "STRING",
				Section: &struct {
					ID    string `json:"id"`
					Label string `json:"label,omitempty"`
				}{ID: "sec1", Label: "Database"},
			},
		},
	}
	data, err := json.Marshal(item)
	require.NoError(t, err)

	m := &mockRunner{output: data}
	editor := newTestEditor(m)

	result, err := editor.GetEditorFields(context.Background(), "TestVault/item1")
	require.NoError(t, err)

	require.Len(t, result, 1)
	assert.Equal(t, "Database", result[0].Section)
}

func TestUpdateField(t *testing.T) {
	m := &mockRunner{output: []byte("{}")}
	editor := newTestEditor(m)

	err := editor.UpdateField(context.Background(), "TestVault/myitem", "DB_HOST", "localhost", "")
	require.NoError(t, err)

	require.Len(t, m.calls, 1)
	assert.Equal(t, []string{
		"item", "edit", "myitem",
		"DB_HOST=localhost",
		"--vault", "TestVault",
		"--account", "test-account",
	}, m.calls[0])
}

func TestDeleteField(t *testing.T) {
	m := &mockRunner{output: []byte("{}")}
	editor := newTestEditor(m)

	err := editor.DeleteField(context.Background(), "TestVault/myitem", "OLD_KEY", "")
	require.NoError(t, err)

	require.Len(t, m.calls, 1)
	assert.Equal(t, []string{
		"item", "edit", "myitem",
		"OLD_KEY[delete]",
		"--vault", "TestVault",
		"--account", "test-account",
	}, m.calls[0])
}

func TestRenameField(t *testing.T) {
	item := Item{
		ID:    "myitem",
		Title: "Test",
		Fields: []Field{
			{ID: "f1", Label: "old_key", Value: "the-value", Type: "STRING"},
		},
	}
	itemData, err := json.Marshal(item)
	require.NoError(t, err)

	callCount := 0
	editor := newTestEditor(&mockRunner{})
	editor.runCmd = func(_ context.Context, args ...string) ([]byte, error) {
		callCount++
		switch callCount {
		case 1:
			return itemData, nil
		default:
			return []byte("{}"), nil
		}
	}

	err = editor.RenameField(context.Background(), "TestVault/myitem", "old_key", "new_key", "")
	require.NoError(t, err)
	assert.Equal(t, 3, callCount, "expected 3 op calls: getItem, delete, update")
}

func TestRenameField_KeyNotFound(t *testing.T) {
	item := Item{
		ID:    "myitem",
		Title: "Test",
		Fields: []Field{
			{ID: "f1", Label: "other_key", Value: "val", Type: "STRING"},
		},
	}
	itemData, err := json.Marshal(item)
	require.NoError(t, err)

	m := &mockRunner{output: itemData}
	editor := newTestEditor(m)

	err = editor.RenameField(context.Background(), "TestVault/myitem", "nonexistent", "new_key", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent")
}

func TestCreateItem(t *testing.T) {
	m := &mockRunner{output: []byte("{}")}
	editor := newTestEditor(m)

	fields := []FieldPair{
		{Key: "DB_HOST", Value: "localhost"},
		{Key: "DB_PASS", Value: "secret"},
	}
	err := editor.CreateEditorItem(context.Background(), "MyVault", "new-item", fields)
	require.NoError(t, err)

	require.Len(t, m.calls, 1)
	args := m.calls[0]
	assert.Equal(t, "item", args[0])
	assert.Equal(t, "create", args[1])
	assert.Contains(t, args, "--vault")
	assert.Contains(t, args, "MyVault")
	assert.Contains(t, args, "--title")
	assert.Contains(t, args, "new-item")
	assert.Contains(t, args, "--category")
	assert.Contains(t, args, "SecureNote")
	assert.Contains(t, args, "DB_HOST=localhost")
	assert.Contains(t, args, "DB_PASS=secret")
	assert.Contains(t, args, "--account")
	assert.Contains(t, args, "test-account")
}

func TestSetFieldType_Concealed(t *testing.T) {
	item := Item{
		ID:    "myitem",
		Title: "Test",
		Fields: []Field{
			{ID: "f1", Label: "API_KEY", Value: "abc123", Type: "STRING"},
		},
	}
	itemData, err := json.Marshal(item)
	require.NoError(t, err)

	callCount := 0
	var capturedCalls [][]string
	editor := newTestEditor(&mockRunner{})
	editor.runCmd = func(_ context.Context, args ...string) ([]byte, error) {
		capturedCalls = append(capturedCalls, args)
		callCount++
		if callCount == 1 {
			return itemData, nil
		}
		return []byte("{}"), nil
	}

	err = editor.SetEditorFieldType(context.Background(), "TestVault/myitem", "API_KEY", "", FieldConcealed)
	require.NoError(t, err)

	require.Len(t, capturedCalls, 2)
	editArgs := capturedCalls[1]
	assert.Equal(t, "item", editArgs[0])
	assert.Equal(t, "edit", editArgs[1])
	assert.Equal(t, "myitem", editArgs[2])
	assert.Equal(t, "API_KEY[password]=abc123", editArgs[3])
}

func TestSetFieldType_Text(t *testing.T) {
	item := Item{
		ID:    "myitem",
		Title: "Test",
		Fields: []Field{
			{ID: "f1", Label: "API_KEY", Value: "abc123", Type: "CONCEALED"},
		},
	}
	itemData, err := json.Marshal(item)
	require.NoError(t, err)

	callCount := 0
	var capturedCalls [][]string
	editor := newTestEditor(&mockRunner{})
	editor.runCmd = func(_ context.Context, args ...string) ([]byte, error) {
		capturedCalls = append(capturedCalls, args)
		callCount++
		if callCount == 1 {
			return itemData, nil
		}
		return []byte("{}"), nil
	}

	err = editor.SetEditorFieldType(context.Background(), "TestVault/myitem", "API_KEY", "", FieldText)
	require.NoError(t, err)

	require.Len(t, capturedCalls, 2)
	editArgs := capturedCalls[1]
	assert.Equal(t, "API_KEY[text]=abc123", editArgs[3])
}

func TestSetFieldType_KeyNotFound(t *testing.T) {
	item := Item{
		ID:    "myitem",
		Title: "Test",
		Fields: []Field{
			{ID: "f1", Label: "OTHER", Value: "val", Type: "STRING"},
		},
	}
	itemData, err := json.Marshal(item)
	require.NoError(t, err)

	m := &mockRunner{output: itemData}
	editor := newTestEditor(m)

	err = editor.SetEditorFieldType(context.Background(), "TestVault/myitem", "MISSING", "", FieldConcealed)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MISSING")
}

func TestListVaults_Error(t *testing.T) {
	m := &mockRunner{err: errors.New("op command failed: not signed in")}
	editor := newTestEditor(m)

	_, err := editor.ListEditorVaults(context.Background())
	require.Error(t, err)
}

func TestListItems_Error(t *testing.T) {
	m := &mockRunner{err: errors.New("op command failed")}
	editor := newTestEditor(m)

	_, err := editor.ListEditorItems(context.Background(), "vault")
	require.Error(t, err)
}

func TestUpdateField_RefParsing(t *testing.T) {
	// Test that ref without vault uses default vault
	m := &mockRunner{output: []byte("{}")}
	editor := newTestEditor(m)

	err := editor.UpdateField(context.Background(), "myitem", "KEY", "val", "")
	require.NoError(t, err)

	require.Len(t, m.calls, 1)
	args := m.calls[0]
	// Should use default vault
	assert.Contains(t, strings.Join(args, " "), "--vault")
	assert.Contains(t, strings.Join(args, " "), "TestVault")
}

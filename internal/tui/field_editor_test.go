package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sentiolabs/envctl/internal/secrets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testFields() []secrets.Field {
	return []secrets.Field{
		{ID: "1", Key: "DB_HOST", Value: "localhost", Type: secrets.FieldText},
		{ID: "2", Key: "DB_PASS", Value: "s3cret", Type: secrets.FieldConcealed},
		{ID: "3", Key: "API_KEY", Value: "abc123", Type: secrets.FieldText},
	}
}

func keyMsg(key string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
}

func specialKeyMsg(k tea.KeyType) tea.KeyMsg {
	return tea.KeyMsg{Type: k}
}

func TestNewFieldEditor_InitializesWithFieldsAndTitle(t *testing.T) {
	fields := testFields()
	editor := NewFieldEditor("op://vault/item", "My Item", fields, true)

	assert.Equal(t, fields, editor.fields)
	assert.Equal(t, "My Item", editor.itemName)
	assert.Equal(t, "op://vault/item", editor.itemRef)
	assert.Equal(t, true, editor.hasTypeEditor)
	assert.Equal(t, 0, editor.cursor)
	assert.Equal(t, modeNormal, editor.mode)
	assert.Empty(t, editor.changes)

	view := editor.View()
	assert.Contains(t, view, "My Item")
}

func TestFieldEditor_EditMode_EnterAndSave(t *testing.T) {
	fields := testFields()
	editor := NewFieldEditor("ref", "Item", fields, false)

	// Press 'e' to enter edit mode
	editor, _ = editor.Update(keyMsg("e"))
	assert.Equal(t, modeEdit, editor.mode)

	// The text input should have the current value
	assert.Equal(t, "localhost", editor.input.Value())

	// Type a new value: clear and type "newhost"
	editor.input.SetValue("newhost")

	// Press Enter to save
	editor, _ = editor.Update(specialKeyMsg(tea.KeyEnter))
	assert.Equal(t, modeNormal, editor.mode)

	changes := editor.PendingChanges()
	require.Len(t, changes, 1)
	assert.Equal(t, "update", changes[0].Type)
	assert.Equal(t, "DB_HOST", changes[0].Field.Key)
	assert.Equal(t, "newhost", changes[0].Field.Value)
}

func TestFieldEditor_EditMode_EscCancels(t *testing.T) {
	fields := testFields()
	editor := NewFieldEditor("ref", "Item", fields, false)

	// Press 'e' to enter edit mode
	editor, _ = editor.Update(keyMsg("e"))
	assert.Equal(t, modeEdit, editor.mode)

	// Change the value
	editor.input.SetValue("changed")

	// Press Esc to cancel
	editor, _ = editor.Update(specialKeyMsg(tea.KeyEscape))
	assert.Equal(t, modeNormal, editor.mode)
	assert.Empty(t, editor.PendingChanges())
	// Original value should be unchanged
	assert.Equal(t, "localhost", editor.fields[0].Value)
}

func TestFieldEditor_Delete_ShowsConfirmation(t *testing.T) {
	fields := testFields()
	editor := NewFieldEditor("ref", "Item", fields, false)

	// Press 'd' to delete
	editor, _ = editor.Update(keyMsg("d"))
	assert.Equal(t, modeConfirmDelete, editor.mode)

	view := editor.View()
	assert.Contains(t, view, "DB_HOST")
}

func TestFieldEditor_Delete_ConfirmWithY(t *testing.T) {
	fields := testFields()
	editor := NewFieldEditor("ref", "Item", fields, false)

	// Press 'd' then 'y'
	editor, _ = editor.Update(keyMsg("d"))
	editor, _ = editor.Update(keyMsg("y"))

	assert.Equal(t, modeNormal, editor.mode)
	changes := editor.PendingChanges()
	require.Len(t, changes, 1)
	assert.Equal(t, "delete", changes[0].Type)
	assert.Equal(t, "DB_HOST", changes[0].Field.Key)
}

func TestFieldEditor_Delete_DismissWithN(t *testing.T) {
	fields := testFields()
	editor := NewFieldEditor("ref", "Item", fields, false)

	// Press 'd' then 'n'
	editor, _ = editor.Update(keyMsg("d"))
	editor, _ = editor.Update(keyMsg("n"))

	assert.Equal(t, modeNormal, editor.mode)
	assert.Empty(t, editor.PendingChanges())
}

func TestFieldEditor_RenameMode(t *testing.T) {
	fields := testFields()
	editor := NewFieldEditor("ref", "Item", fields, false)

	// Press 'r' to rename
	editor, _ = editor.Update(keyMsg("r"))
	assert.Equal(t, modeRename, editor.mode)
	assert.Equal(t, "DB_HOST", editor.input.Value())

	// Set new key name
	editor.input.SetValue("DATABASE_HOST")

	// Press Enter to save
	editor, _ = editor.Update(specialKeyMsg(tea.KeyEnter))
	assert.Equal(t, modeNormal, editor.mode)

	changes := editor.PendingChanges()
	require.Len(t, changes, 1)
	assert.Equal(t, "rename", changes[0].Type)
	assert.Equal(t, "DB_HOST", changes[0].OldKey)
	assert.Equal(t, "DATABASE_HOST", changes[0].Field.Key)
}

func TestFieldEditor_ToggleType_WhenEnabled(t *testing.T) {
	fields := testFields()
	editor := NewFieldEditor("ref", "Item", fields, true)

	// First field is FieldText, press 't' to toggle
	editor, _ = editor.Update(keyMsg("t"))

	changes := editor.PendingChanges()
	require.Len(t, changes, 1)
	assert.Equal(t, "set_type", changes[0].Type)
	assert.Equal(t, "DB_HOST", changes[0].Field.Key)
	assert.Equal(t, secrets.FieldConcealed, changes[0].NewType)
}

func TestFieldEditor_ToggleType_WhenDisabled(t *testing.T) {
	fields := testFields()
	editor := NewFieldEditor("ref", "Item", fields, false)

	// Press 't' — should do nothing when hasTypeEditor is false
	editor, _ = editor.Update(keyMsg("t"))
	assert.Empty(t, editor.PendingChanges())
}

func TestFieldEditor_NewField(t *testing.T) {
	fields := testFields()
	editor := NewFieldEditor("ref", "Item", fields, false)

	// Press 'n' for new field — but note 'n' in normal mode conflicts with dismiss
	// In normal mode, 'n' means "new field"
	editor, _ = editor.Update(keyMsg("n"))
	assert.Equal(t, modeNewFieldKey, editor.mode)

	// Type key name
	editor.input.SetValue("NEW_VAR")
	editor, _ = editor.Update(specialKeyMsg(tea.KeyEnter))
	assert.Equal(t, modeNewFieldValue, editor.mode)
	assert.Equal(t, "NEW_VAR", editor.newFieldKey)

	// Type value
	editor.input.SetValue("new_value")
	editor, _ = editor.Update(specialKeyMsg(tea.KeyEnter))
	assert.Equal(t, modeNormal, editor.mode)

	changes := editor.PendingChanges()
	require.Len(t, changes, 1)
	assert.Equal(t, "update", changes[0].Type)
	assert.Equal(t, "NEW_VAR", changes[0].Field.Key)
	assert.Equal(t, "new_value", changes[0].Field.Value)
}

func TestFieldEditor_PendingChanges_Accumulates(t *testing.T) {
	fields := testFields()
	editor := NewFieldEditor("ref", "Item", fields, true)

	// Edit first field
	editor, _ = editor.Update(keyMsg("e"))
	editor.input.SetValue("newval")
	editor, _ = editor.Update(specialKeyMsg(tea.KeyEnter))

	// Toggle type on first field
	editor, _ = editor.Update(keyMsg("t"))

	// Move to second field and delete it
	editor, _ = editor.Update(specialKeyMsg(tea.KeyDown))
	editor, _ = editor.Update(keyMsg("d"))
	editor, _ = editor.Update(keyMsg("y"))

	changes := editor.PendingChanges()
	assert.Len(t, changes, 3)
}

func TestFieldEditor_EscInNormalMode_SignalsBack(t *testing.T) {
	fields := testFields()
	editor := NewFieldEditor("ref", "Item", fields, false)

	assert.False(t, editor.GoBack())

	editor, _ = editor.Update(specialKeyMsg(tea.KeyEscape))
	assert.True(t, editor.GoBack())
}

func TestFieldEditor_ConcealedFields_DisplayMasked(t *testing.T) {
	fields := testFields()
	editor := NewFieldEditor("ref", "Item", fields, false)

	view := editor.View()
	// DB_PASS is concealed — should show masked value
	assert.Contains(t, view, "********")
	// Should NOT contain the actual secret value in the view
	assert.NotContains(t, view, "s3cret")
	// DB_HOST is text — should show actual value
	assert.Contains(t, view, "localhost")
}

func TestFieldEditor_Navigation(t *testing.T) {
	fields := testFields()
	editor := NewFieldEditor("ref", "Item", fields, false)

	assert.Equal(t, 0, editor.cursor)

	editor, _ = editor.Update(specialKeyMsg(tea.KeyDown))
	assert.Equal(t, 1, editor.cursor)

	editor, _ = editor.Update(specialKeyMsg(tea.KeyDown))
	assert.Equal(t, 2, editor.cursor)

	// Should not go past the end
	editor, _ = editor.Update(specialKeyMsg(tea.KeyDown))
	assert.Equal(t, 2, editor.cursor)

	editor, _ = editor.Update(specialKeyMsg(tea.KeyUp))
	assert.Equal(t, 1, editor.cursor)

	editor, _ = editor.Update(specialKeyMsg(tea.KeyUp))
	assert.Equal(t, 0, editor.cursor)

	// Should not go below 0
	editor, _ = editor.Update(specialKeyMsg(tea.KeyUp))
	assert.Equal(t, 0, editor.cursor)
}

func TestFieldEditor_Quit(t *testing.T) {
	fields := testFields()
	editor := NewFieldEditor("ref", "Item", fields, false)

	assert.False(t, editor.Quitting())

	editor, _ = editor.Update(keyMsg("q"))
	assert.True(t, editor.Quitting())
}

// Confirm model tests
func TestConfirm_NewConfirm(t *testing.T) {
	c := NewConfirm("Delete this?")
	assert.Equal(t, "Delete this?", c.Message)
	assert.False(t, c.Confirmed())
	assert.False(t, c.Dismissed())
}

func TestConfirm_PressY_Confirms(t *testing.T) {
	c := NewConfirm("Delete?")
	c, _ = c.Update(keyMsg("y"))
	assert.True(t, c.Confirmed())
	assert.False(t, c.Dismissed())
}

func TestConfirm_PressN_Dismisses(t *testing.T) {
	c := NewConfirm("Delete?")
	c, _ = c.Update(keyMsg("n"))
	assert.False(t, c.Confirmed())
	assert.True(t, c.Dismissed())
}

func TestConfirm_PressEsc_Dismisses(t *testing.T) {
	c := NewConfirm("Delete?")
	c, _ = c.Update(specialKeyMsg(tea.KeyEscape))
	assert.False(t, c.Confirmed())
	assert.True(t, c.Dismissed())
}

func TestConfirm_View_ShowsMessage(t *testing.T) {
	c := NewConfirm("Delete field X?")
	view := c.View()
	assert.Contains(t, view, "Delete field X?")
	assert.Contains(t, view, "y")
	assert.Contains(t, view, "n")
}

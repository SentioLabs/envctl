package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/sentiolabs/envctl/internal/secrets"
)

// visualTestFields returns a standard set of fields for visual tests.
func visualTestFields() []secrets.Field {
	return []secrets.Field{
		{ID: "1", Key: "APP_SERVER_PORT", Value: "8080", Type: secrets.FieldText},
		{ID: "2", Key: "DATABASE_URL", Value: "postgres://localhost:5432/mydb", Type: secrets.FieldConcealed},
		{ID: "3", Key: "REDIS_HOST", Value: "localhost", Type: secrets.FieldText},
		{ID: "4", Key: "API_KEY", Value: "sk-1234567890", Type: secrets.FieldConcealed},
		{ID: "5", Key: "APP_ENV", Value: "local", Type: secrets.FieldText},
	}
}

// manyFields generates n fields for viewport scroll testing.
func manyFields(n int) []secrets.Field {
	fields := make([]secrets.Field, n)
	for i := range n {
		fields[i] = secrets.Field{
			ID:    string(rune('a' + i%26)),
			Key:   "FIELD_" + string(rune('A'+i%26)) + string(rune('0'+i/26)),
			Value: "value-" + string(rune('0'+i%10)),
			Type:  secrets.FieldText,
		}
	}
	return fields
}

// sendKey is a helper that sends a key and returns the updated editor.
func sendKey(m FieldEditor, key tea.KeyMsg) FieldEditor {
	m, _ = m.Update(key)
	return m
}

func keyRune(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

// TestVisualNormalMode verifies the default rendered state:
// field table with cursor on first row, concealed values masked, help bar.
func TestVisualNormalMode(t *testing.T) {
	editor := NewFieldEditor("op://BACstack/Test Item", "Test Item", visualTestFields(), true)
	editor.SetHeight(24)

	golden.RequireEqual(t, []byte(editor.View()))
}

// TestVisualEditMode verifies the edit mode renders the text input
// with the current field's value pre-filled.
func TestVisualEditMode(t *testing.T) {
	editor := NewFieldEditor("op://BACstack/Test Item", "Test Item", visualTestFields(), true)
	editor.SetHeight(24)

	// Move to REDIS_HOST (3rd field) and press Enter to edit
	editor = sendKey(editor, tea.KeyMsg{Type: tea.KeyDown})
	editor = sendKey(editor, tea.KeyMsg{Type: tea.KeyDown})
	editor = sendKey(editor, tea.KeyMsg{Type: tea.KeyEnter})

	golden.RequireEqual(t, []byte(editor.View()))
}

// TestVisualPendingChanges verifies the status bar shows pending change count
// after making edits.
func TestVisualPendingChanges(t *testing.T) {
	editor := NewFieldEditor("op://BACstack/Test Item", "Test Item", visualTestFields(), true)
	editor.SetHeight(24)

	// Edit first field: enter edit mode, type new value, confirm
	editor = sendKey(editor, keyRune('e'))
	editor = sendKey(editor, tea.KeyMsg{Type: tea.KeyEnter}) // accept current value as "change"

	// Toggle type on second field
	editor = sendKey(editor, tea.KeyMsg{Type: tea.KeyDown})
	editor = sendKey(editor, keyRune('t'))

	golden.RequireEqual(t, []byte(editor.View()))
}

// TestVisualFilter verifies the filter narrows visible fields and shows
// the active filter indicator.
func TestVisualFilter(t *testing.T) {
	editor := NewFieldEditor("op://BACstack/Test Item", "Test Item", visualTestFields(), true)
	editor.SetHeight(24)

	// Activate filter, type "app", accept
	editor = sendKey(editor, keyRune('/'))
	// Simulate typing "app" by sending rune keys through the text input
	for _, r := range "app" {
		editor = sendKey(editor, keyRune(r))
	}
	editor = sendKey(editor, tea.KeyMsg{Type: tea.KeyEnter})

	golden.RequireEqual(t, []byte(editor.View()))
}

// TestVisualConfirmDelete verifies the delete confirmation overlay renders.
func TestVisualConfirmDelete(t *testing.T) {
	editor := NewFieldEditor("op://BACstack/Test Item", "Test Item", visualTestFields(), true)
	editor.SetHeight(24)

	editor = sendKey(editor, keyRune('d'))

	golden.RequireEqual(t, []byte(editor.View()))
}

// TestVisualConfirmDiscard verifies the discard confirmation overlay
// when pressing Esc with pending changes.
func TestVisualConfirmDiscard(t *testing.T) {
	editor := NewFieldEditor("op://BACstack/Test Item", "Test Item", visualTestFields(), true)
	editor.SetHeight(24)

	// Make a change first
	editor = sendKey(editor, keyRune('e'))
	editor = sendKey(editor, tea.KeyMsg{Type: tea.KeyEnter})

	// Now press Esc — should show discard confirmation
	editor = sendKey(editor, tea.KeyMsg{Type: tea.KeyEscape})

	golden.RequireEqual(t, []byte(editor.View()))
}

// TODO(bfirestone): TestVisualViewportScroll
//
// This is your test to write! It verifies that when there are more fields
// than fit on screen, the viewport renders scroll indicators correctly.
//
// Here's the scaffold:
//
//   func TestVisualViewportScroll(t *testing.T) {
//       // Create editor with 20 fields but only height=15
//       // (fieldCapacity reserves ~11 lines for chrome, so ~4 fields visible)
//       fields := manyFields(20)
//       editor := NewFieldEditor("test/scroll", "Scroll Test", fields, false)
//       editor.SetHeight(15)
//
//       // Step 1: Capture initial state — should show "↓ N more below"
//       // (but NO "↑ more above" since we're at the top)
//
//       // Step 2: Send Ctrl+D to scroll down half a page
//       //   editor = sendKey(editor, tea.KeyMsg{Type: tea.KeyCtrlD})
//
//       // Step 3: Now capture — should show BOTH scroll indicators
//       //   golden.RequireEqual(t, []byte(editor.View()))
//
//       // Design choice: should you test the initial state AND the scrolled
//       // state as separate golden files? Or just the scrolled state?
//       // Hint: separate golden files (two sub-tests) are more useful
//       // because when one breaks you know exactly which state regressed.
//   }
//
// To generate golden files after writing: go test -run TestVisualViewportScroll -update ./internal/tui/...
// To run against golden files:            go test -run TestVisualViewportScroll ./internal/tui/...

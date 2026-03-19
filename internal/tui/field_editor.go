package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sentiolabs/envctl/internal/secrets"
)

type editMode int

const (
	modeNormal editMode = iota
	modeEdit
	modeRename
	modeNewFieldKey
	modeNewFieldValue
	modeConfirmDelete
)

// concealedPlaceholder is displayed instead of the actual value for concealed fields.
const concealedPlaceholder = "********"

// PendingChange represents a single pending modification to a field.
type PendingChange struct {
	Type    string            // "update", "delete", "rename", "set_type"
	Field   secrets.Field     // the field being changed
	OldKey  string            // original key name (for rename)
	NewType secrets.FieldType // target type (for set_type)
}

// FieldEditor is a TUI screen that displays a table of secret fields and
// supports CRUD operations: edit value, rename key, delete, toggle type,
// and add new fields.
type FieldEditor struct {
	fields        []secrets.Field
	cursor        int
	mode          editMode
	input         textinput.Model
	confirm       Confirm
	changes       []PendingChange
	hasTypeEditor bool
	itemRef       string
	itemName      string
	newFieldKey   string // temp storage during new-field flow
	back          bool
	quitting      bool
}

// NewFieldEditor creates a new field editor for the given item.
func NewFieldEditor(itemRef, itemName string, fields []secrets.Field, hasTypeEditor bool) FieldEditor {
	ti := textinput.New()
	ti.Focus()

	return FieldEditor{
		fields:        fields,
		itemRef:       itemRef,
		itemName:      itemName,
		hasTypeEditor: hasTypeEditor,
		input:         ti,
		mode:          modeNormal,
	}
}

// Init returns nil; no initial command is needed.
func (m FieldEditor) Init() tea.Cmd {
	return nil
}

// Update processes key messages according to the current mode.
func (m FieldEditor) Update(msg tea.Msg) (FieldEditor, tea.Cmd) {
	switch m.mode {
	case modeEdit:
		return m.updateEdit(msg)
	case modeRename:
		return m.updateRename(msg)
	case modeNewFieldKey:
		return m.updateNewFieldKey(msg)
	case modeNewFieldValue:
		return m.updateNewFieldValue(msg)
	case modeConfirmDelete:
		return m.updateConfirmDelete(msg)
	default:
		return m.updateNormal(msg)
	}
}

func (m FieldEditor) updateNormal(msg tea.Msg) (FieldEditor, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch keyMsg.Type {
	case tea.KeyUp:
		if m.cursor > 0 {
			m.cursor--
		}
	case tea.KeyDown:
		if m.cursor < len(m.fields)-1 {
			m.cursor++
		}
	case tea.KeyEscape:
		m.back = true
	case tea.KeyRunes:
		switch string(keyMsg.Runes) {
		case "e":
			m.mode = modeEdit
			m.input.SetValue(m.fields[m.cursor].Value)
			m.input.CursorEnd()
			return m, m.input.Focus()
		case "d":
			m.mode = modeConfirmDelete
			m.confirm = NewConfirm(fmt.Sprintf("Delete %s?", m.fields[m.cursor].Key))
		case "r":
			m.mode = modeRename
			m.input.SetValue(m.fields[m.cursor].Key)
			m.input.CursorEnd()
			return m, m.input.Focus()
		case "t":
			if m.hasTypeEditor {
				field := m.fields[m.cursor]
				newType := secrets.FieldConcealed
				if field.Type == secrets.FieldConcealed {
					newType = secrets.FieldText
				}
				m.changes = append(m.changes, PendingChange{
					Type:    "set_type",
					Field:   field,
					NewType: newType,
				})
				m.fields[m.cursor].Type = newType
			}
		case "n":
			m.mode = modeNewFieldKey
			m.input.SetValue("")
			return m, m.input.Focus()
		case "q":
			m.quitting = true
		}
	}

	return m, nil
}

func (m FieldEditor) updateEdit(msg tea.Msg) (FieldEditor, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	switch keyMsg.Type {
	case tea.KeyEnter:
		field := m.fields[m.cursor]
		field.Value = m.input.Value()
		m.changes = append(m.changes, PendingChange{
			Type:  "update",
			Field: field,
		})
		m.fields[m.cursor].Value = m.input.Value()
		m.mode = modeNormal
		return m, nil
	case tea.KeyEscape:
		m.mode = modeNormal
		return m, nil
	default:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
}

func (m FieldEditor) updateRename(msg tea.Msg) (FieldEditor, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	switch keyMsg.Type {
	case tea.KeyEnter:
		oldKey := m.fields[m.cursor].Key
		field := m.fields[m.cursor]
		field.Key = m.input.Value()
		m.changes = append(m.changes, PendingChange{
			Type:   "rename",
			Field:  field,
			OldKey: oldKey,
		})
		m.fields[m.cursor].Key = m.input.Value()
		m.mode = modeNormal
		return m, nil
	case tea.KeyEscape:
		m.mode = modeNormal
		return m, nil
	default:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
}

func (m FieldEditor) updateNewFieldKey(msg tea.Msg) (FieldEditor, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	switch keyMsg.Type {
	case tea.KeyEnter:
		m.newFieldKey = m.input.Value()
		m.mode = modeNewFieldValue
		m.input.SetValue("")
		return m, m.input.Focus()
	case tea.KeyEscape:
		m.mode = modeNormal
		return m, nil
	default:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
}

func (m FieldEditor) updateNewFieldValue(msg tea.Msg) (FieldEditor, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	switch keyMsg.Type {
	case tea.KeyEnter:
		newField := secrets.Field{
			Key:   m.newFieldKey,
			Value: m.input.Value(),
			Type:  secrets.FieldText,
		}
		m.changes = append(m.changes, PendingChange{
			Type:  "update",
			Field: newField,
		})
		m.fields = append(m.fields, newField)
		m.newFieldKey = ""
		m.mode = modeNormal
		return m, nil
	case tea.KeyEscape:
		m.newFieldKey = ""
		m.mode = modeNormal
		return m, nil
	default:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
}

func (m FieldEditor) updateConfirmDelete(msg tea.Msg) (FieldEditor, tea.Cmd) {
	var cmd tea.Cmd
	m.confirm, cmd = m.confirm.Update(msg)

	if m.confirm.Confirmed() {
		field := m.fields[m.cursor]
		m.changes = append(m.changes, PendingChange{
			Type:  "delete",
			Field: field,
		})
		m.mode = modeNormal
	} else if m.confirm.Dismissed() {
		m.mode = modeNormal
	}

	return m, cmd
}

// View renders the field editor screen.
func (m FieldEditor) View() string {
	var b strings.Builder

	b.WriteString(Title.Render(fmt.Sprintf("Fields: %s", m.itemName)))
	b.WriteString("\n")
	b.WriteString(Subtitle.Render(m.itemRef))
	b.WriteString("\n\n")

	// Render field table
	for i, f := range m.fields {
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}

		displayValue := f.Value
		if f.Type == secrets.FieldConcealed {
			displayValue = concealedPlaceholder
		}

		line := fmt.Sprintf("%s%-20s  %-30s  %s", cursor, f.Key, displayValue, string(f.Type))
		if i == m.cursor {
			b.WriteString(Selected.Render(line))
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Render mode-specific UI
	switch m.mode {
	case modeEdit:
		b.WriteString(fmt.Sprintf("  Edit value for %s:\n", m.fields[m.cursor].Key))
		b.WriteString("  " + m.input.View() + "\n")
	case modeRename:
		b.WriteString(fmt.Sprintf("  Rename %s:\n", m.fields[m.cursor].Key))
		b.WriteString("  " + m.input.View() + "\n")
	case modeNewFieldKey:
		b.WriteString("  New field key:\n")
		b.WriteString("  " + m.input.View() + "\n")
	case modeNewFieldValue:
		b.WriteString(fmt.Sprintf("  Value for %s:\n", m.newFieldKey))
		b.WriteString("  " + m.input.View() + "\n")
	case modeConfirmDelete:
		b.WriteString(m.confirm.View())
	}

	// Status bar
	changeCount := len(m.changes)
	if changeCount > 0 {
		b.WriteString(StatusBar.Render(fmt.Sprintf("\n  %d pending change(s)", changeCount)))
		b.WriteString("\n")
	}

	// Help
	helpText := "e:edit  d:delete  r:rename  n:new  esc:back  q:quit"
	if m.hasTypeEditor {
		helpText = "e:edit  d:delete  r:rename  t:toggle type  n:new  esc:back  q:quit"
	}
	b.WriteString("\n")
	b.WriteString(Help.Render(helpText))

	return b.String()
}

// PendingChanges returns the accumulated list of changes.
func (m FieldEditor) PendingChanges() []PendingChange {
	return m.changes
}

// GoBack returns true if the user pressed Esc in normal mode.
func (m FieldEditor) GoBack() bool {
	return m.back
}

// Quitting returns true if the user pressed q.
func (m FieldEditor) Quitting() bool {
	return m.quitting
}

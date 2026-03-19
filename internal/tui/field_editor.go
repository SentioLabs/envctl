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
	modeConfirmDiscard
	modeFilter
)

// concealedPlaceholder is displayed instead of the actual value for concealed fields.
const concealedPlaceholder = "********"

// pendingActionType tracks what to do after a discard confirmation.
type pendingActionType int

const (
	actionBack pendingActionType = iota
	actionQuit
)

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
	newFieldKey     string            // temp storage during new-field flow
	pendingAction   pendingActionType // what to do after discard confirm
	filterText      string            // current filter query
	filteredIndices []int             // indices into fields that match filter
	height          int              // terminal height for viewport scrolling
	viewportOffset  int              // first visible row index
	back            bool
	saving        bool
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

// applyFilter recalculates filteredIndices based on filterText.
func (m *FieldEditor) applyFilter() {
	if m.filterText == "" {
		m.filteredIndices = nil
		return
	}
	query := strings.ToLower(m.filterText)
	m.filteredIndices = nil
	for i, f := range m.fields {
		if strings.Contains(strings.ToLower(f.Key), query) ||
			strings.Contains(strings.ToLower(f.Value), query) {
			m.filteredIndices = append(m.filteredIndices, i)
		}
	}
	if m.cursor >= len(m.filteredIndices) {
		m.cursor = max(0, len(m.filteredIndices)-1)
	}
}

// visibleFields returns the indices to display (filtered or all).
func (m FieldEditor) visibleFields() []int {
	if m.filteredIndices != nil {
		return m.filteredIndices
	}
	indices := make([]int, len(m.fields))
	for i := range m.fields {
		indices[i] = i
	}
	return indices
}

// realIndex maps the cursor position to the actual index in m.fields.
func (m FieldEditor) realIndex() int {
	visible := m.visibleFields()
	if len(visible) == 0 {
		return -1
	}
	return visible[m.cursor]
}

// SetHeight sets the terminal height for viewport scrolling.
func (m *FieldEditor) SetHeight(h int) {
	m.height = h
}

// fieldCapacity returns how many field rows fit in the viewport.
// Accounts for chrome: title(1) + subtitle(1) + filter(1) + blank(1) +
// mode UI(~3) + status(2) + help(2) = ~11 lines of chrome.
func (m FieldEditor) fieldCapacity() int {
	chrome := 11
	if m.filterText != "" && m.mode != modeFilter {
		chrome++ // active filter indicator
	}
	cap := m.height - chrome
	if cap < 3 {
		cap = 3
	}
	return cap
}

// ensureCursorVisible adjusts viewportOffset so the cursor is within the visible window.
func (m *FieldEditor) ensureCursorVisible() {
	cap := m.fieldCapacity()
	if m.cursor < m.viewportOffset {
		m.viewportOffset = m.cursor
	}
	if m.cursor >= m.viewportOffset+cap {
		m.viewportOffset = m.cursor - cap + 1
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
	case modeConfirmDiscard:
		return m.updateConfirmDiscard(msg)
	case modeFilter:
		return m.updateFilter(msg)
	default:
		return m.updateNormal(msg)
	}
}

func (m FieldEditor) updateNormal(msg tea.Msg) (FieldEditor, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	visible := m.visibleFields()
	idx := m.realIndex()

	switch keyMsg.Type {
	case tea.KeyUp:
		if m.cursor > 0 {
			m.cursor--
		}
		m.ensureCursorVisible()
	case tea.KeyDown:
		if m.cursor < len(visible)-1 {
			m.cursor++
		}
		m.ensureCursorVisible()
	case tea.KeyCtrlU:
		// Half-page up
		jump := m.fieldCapacity() / 2
		m.cursor -= jump
		if m.cursor < 0 {
			m.cursor = 0
		}
		m.ensureCursorVisible()
	case tea.KeyCtrlD:
		// Half-page down
		jump := m.fieldCapacity() / 2
		m.cursor += jump
		if m.cursor >= len(visible) {
			m.cursor = len(visible) - 1
		}
		m.ensureCursorVisible()
	case tea.KeyEscape:
		// If filtering, clear filter first
		if m.filterText != "" {
			m.filterText = ""
			m.filteredIndices = nil
			m.cursor = 0
			return m, nil
		}
		if len(m.changes) > 0 {
			m.mode = modeConfirmDiscard
			m.confirm = NewConfirm(fmt.Sprintf("Discard %d unsaved change(s)?", len(m.changes)))
			m.pendingAction = actionBack
			return m, nil
		}
		m.back = true
	case tea.KeyEnter:
		if idx >= 0 {
			m.mode = modeEdit
			m.input.SetValue(m.fields[idx].Value)
			m.input.CursorEnd()
			return m, m.input.Focus()
		}
	case tea.KeyRunes:
		switch string(keyMsg.Runes) {
		case "/":
			m.mode = modeFilter
			m.input.SetValue(m.filterText)
			m.input.CursorEnd()
			return m, m.input.Focus()
		case "e":
			if idx >= 0 {
				m.mode = modeEdit
				m.input.SetValue(m.fields[idx].Value)
				m.input.CursorEnd()
				return m, m.input.Focus()
			}
		case "s":
			if len(m.changes) > 0 {
				m.saving = true
			}
		case "d":
			if idx >= 0 {
				m.mode = modeConfirmDelete
				m.confirm = NewConfirm(fmt.Sprintf("Delete %s?", m.fields[idx].Key))
			}
		case "r":
			if idx >= 0 {
				m.mode = modeRename
				m.input.SetValue(m.fields[idx].Key)
				m.input.CursorEnd()
				return m, m.input.Focus()
			}
		case "t":
			if m.hasTypeEditor && idx >= 0 {
				field := m.fields[idx]
				newType := secrets.FieldConcealed
				if field.Type == secrets.FieldConcealed {
					newType = secrets.FieldText
				}
				m.changes = append(m.changes, PendingChange{
					Type:    "set_type",
					Field:   field,
					NewType: newType,
				})
				m.fields[idx].Type = newType
			}
		case "n":
			m.mode = modeNewFieldKey
			m.input.SetValue("")
			return m, m.input.Focus()
		case "q":
			if len(m.changes) > 0 {
				m.mode = modeConfirmDiscard
				m.confirm = NewConfirm(fmt.Sprintf("Discard %d unsaved change(s)?", len(m.changes)))
				m.pendingAction = actionQuit
				return m, nil
			}
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

	idx := m.realIndex()

	switch keyMsg.Type {
	case tea.KeyEnter:
		field := m.fields[idx]
		field.Value = m.input.Value()
		m.changes = append(m.changes, PendingChange{
			Type:  "update",
			Field: field,
		})
		m.fields[idx].Value = m.input.Value()
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

	idx := m.realIndex()

	switch keyMsg.Type {
	case tea.KeyEnter:
		oldKey := m.fields[idx].Key
		field := m.fields[idx]
		field.Key = m.input.Value()
		m.changes = append(m.changes, PendingChange{
			Type:   "rename",
			Field:  field,
			OldKey: oldKey,
		})
		m.fields[idx].Key = m.input.Value()
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
		idx := m.realIndex()
		if idx >= 0 {
			field := m.fields[idx]
			m.changes = append(m.changes, PendingChange{
				Type:  "delete",
				Field: field,
			})
		}
		m.mode = modeNormal
	} else if m.confirm.Dismissed() {
		m.mode = modeNormal
	}

	return m, cmd
}

func (m FieldEditor) updateFilter(msg tea.Msg) (FieldEditor, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	switch keyMsg.Type {
	case tea.KeyEnter, tea.KeyEscape:
		// Accept filter and return to normal mode
		m.filterText = m.input.Value()
		m.applyFilter()
		m.mode = modeNormal
		return m, nil
	default:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		// Live-filter as user types
		m.filterText = m.input.Value()
		m.applyFilter()
		return m, cmd
	}
}

func (m FieldEditor) updateConfirmDiscard(msg tea.Msg) (FieldEditor, tea.Cmd) {
	var cmd tea.Cmd
	m.confirm, cmd = m.confirm.Update(msg)

	if m.confirm.Confirmed() {
		m.changes = nil
		m.mode = modeNormal
		switch m.pendingAction {
		case actionBack:
			m.back = true
		case actionQuit:
			m.quitting = true
		}
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
	b.WriteString("\n")

	// Show active filter
	if m.filterText != "" && m.mode != modeFilter {
		b.WriteString(StatusBar.Render(fmt.Sprintf("  filter: %s", m.filterText)))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Render field table (filtered + viewport)
	visible := m.visibleFields()
	cap := m.fieldCapacity()
	endIdx := m.viewportOffset + cap
	if endIdx > len(visible) {
		endIdx = len(visible)
	}

	// Scroll indicators
	if m.viewportOffset > 0 {
		b.WriteString(Subtitle.Render(fmt.Sprintf("  ↑ %d more above", m.viewportOffset)))
		b.WriteString("\n")
	}

	for vi := m.viewportOffset; vi < endIdx; vi++ {
		fi := visible[vi]
		f := m.fields[fi]
		cursor := "  "
		if vi == m.cursor {
			cursor = "> "
		}

		displayValue := f.Value
		if f.Type == secrets.FieldConcealed {
			displayValue = concealedPlaceholder
		}

		line := fmt.Sprintf("%s%-20s  %-30s  %s", cursor, f.Key, displayValue, string(f.Type))
		if vi == m.cursor {
			b.WriteString(Selected.Render(line))
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
	}

	// Scroll indicators
	if endIdx < len(visible) {
		b.WriteString(Subtitle.Render(fmt.Sprintf("  ↓ %d more below", len(visible)-endIdx)))
		b.WriteString("\n")
	}

	if len(visible) == 0 && m.filterText != "" {
		b.WriteString(Subtitle.Render("  No fields match filter"))
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Render mode-specific UI
	idx := m.realIndex()
	switch m.mode {
	case modeEdit:
		if idx >= 0 {
			b.WriteString(fmt.Sprintf("  Edit value for %s:\n", m.fields[idx].Key))
		}
		b.WriteString("  " + m.input.View() + "\n")
	case modeRename:
		if idx >= 0 {
			b.WriteString(fmt.Sprintf("  Rename %s:\n", m.fields[idx].Key))
		}
		b.WriteString("  " + m.input.View() + "\n")
	case modeNewFieldKey:
		b.WriteString("  New field key:\n")
		b.WriteString("  " + m.input.View() + "\n")
	case modeNewFieldValue:
		b.WriteString(fmt.Sprintf("  Value for %s:\n", m.newFieldKey))
		b.WriteString("  " + m.input.View() + "\n")
	case modeFilter:
		b.WriteString("  Filter: ")
		b.WriteString(m.input.View())
		b.WriteString("\n")
	case modeConfirmDelete:
		b.WriteString(m.confirm.View())
	case modeConfirmDiscard:
		b.WriteString(m.confirm.View())
	}

	// Status bar
	changeCount := len(m.changes)
	if changeCount > 0 {
		b.WriteString(StatusBar.Render(fmt.Sprintf("\n  %d pending change(s)", changeCount)))
		b.WriteString("\n")
	}

	// Help
	helpText := "enter/e:edit  d:delete  r:rename  n:new  /:filter  s:save  esc:back  q:quit"
	if m.hasTypeEditor {
		helpText = "enter/e:edit  d:delete  r:rename  t:toggle  n:new  /:filter  s:save  esc:back  q:quit"
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

// Saving returns true if the user pressed s to apply pending changes.
func (m FieldEditor) Saving() bool {
	return m.saving
}

// Quitting returns true if the user pressed q.
func (m FieldEditor) Quitting() bool {
	return m.quitting
}

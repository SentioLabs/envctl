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

// Change types recorded in PendingChange.Type.
const (
	changeUpdate  = "update"
	changeDelete  = "delete"
	changeRename  = "rename"
	changeSetType = "set_type"
)

// Layout limits for the field table.
const (
	// minFieldRows is the smallest viewport the table ever renders.
	minFieldRows = 3
	// halfPageDivisor turns the viewport height into a ctrl-u/ctrl-d jump.
	halfPageDivisor = 2
	// maxValueWidth caps the value column so wide secrets don't wrap the table.
	maxValueWidth = 50
)

// pendingActionType tracks what to do after a discard confirmation.
type pendingActionType int

const (
	actionBack pendingActionType = iota
	actionQuit
)

// PendingChange represents a single pending modification to a field.
type PendingChange struct {
	Type    string            // one of changeUpdate, changeDelete, changeRename, changeSetType
	Field   secrets.Field     // the field being changed
	OldKey  string            // original key name (for rename)
	NewType secrets.FieldType // target type (for set_type)
}

// FieldEditor is a TUI screen that displays a table of secret fields and
// supports CRUD operations: edit value, rename key, delete, toggle type,
// and add new fields.
type FieldEditor struct {
	fields          []secrets.Field
	cursor          int
	mode            editMode
	input           textinput.Model
	confirm         Confirm
	changes         []PendingChange
	hasTypeEditor   bool
	itemRef         string
	itemName        string
	newFieldKey     string            // temp storage during new-field flow
	pendingAction   pendingActionType // what to do after discard confirm
	filterText      string            // current filter query
	filteredIndices []int             // indices into fields that match filter
	revealed        map[int]bool      // field indices with concealed values shown
	height          int               // terminal height for viewport scrolling
	viewportOffset  int               // first visible row index
	back            bool
	saving          bool
	quitting        bool
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
	rows := m.height - chrome
	if rows < minFieldRows {
		rows = minFieldRows
	}
	return rows
}

// ensureCursorVisible adjusts viewportOffset so the cursor is within the visible window.
func (m *FieldEditor) ensureCursorVisible() {
	rows := m.fieldCapacity()
	if m.cursor < m.viewportOffset {
		m.viewportOffset = m.cursor
	}
	if m.cursor >= m.viewportOffset+rows {
		m.viewportOffset = m.cursor - rows + 1
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

// updateNormal handles keys while the table has focus and no prompt is open.
// Navigation keys move the cursor, everything else is delegated by rune.
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
		jump := m.fieldCapacity() / halfPageDivisor
		m.cursor -= jump
		if m.cursor < 0 {
			m.cursor = 0
		}
		m.ensureCursorVisible()
	case tea.KeyCtrlD:
		// Half-page down
		jump := m.fieldCapacity() / halfPageDivisor
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
		return m.updateNormalRune(string(keyMsg.Runes), idx)
	}

	return m, nil
}

// updateNormalRune handles the single-rune shortcuts available in normal mode.
// idx is the index into m.fields under the cursor, or -1 when the table is empty.
func (m FieldEditor) updateNormalRune(r string, idx int) (FieldEditor, tea.Cmd) {
	switch r {
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
	case "v":
		if idx >= 0 && m.fields[idx].Type == secrets.FieldConcealed {
			m.toggleReveal(idx)
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
			m.toggleFieldType(idx)
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

	return m, nil
}

// toggleReveal flips whether the concealed value at index idx is shown in clear text.
// The reveal state is display-only and is never recorded as a pending change.
func (m *FieldEditor) toggleReveal(idx int) {
	if m.revealed == nil {
		m.revealed = make(map[int]bool)
	}
	m.revealed[idx] = !m.revealed[idx]
}

// toggleFieldType swaps the field at index idx between text and concealed,
// and queues the swap as a pending change for the next save.
func (m *FieldEditor) toggleFieldType(idx int) {
	field := m.fields[idx]
	newType := secrets.FieldConcealed
	if field.Type == secrets.FieldConcealed {
		newType = secrets.FieldText
	}
	m.changes = append(m.changes, PendingChange{
		Type:    changeSetType,
		Field:   field,
		NewType: newType,
	})
	m.fields[idx].Type = newType
}

// updateEdit handles the value prompt. Enter queues an update for the field
// under the cursor and Esc discards the typed value.
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
			Type:  changeUpdate,
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

// updateRename handles the key-rename prompt. Enter queues a rename that
// carries the original key so the backend can find the field.
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
			Type:   changeRename,
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

// updateNewFieldKey handles the first step of the new-field flow. The typed key
// is parked in newFieldKey and the prompt moves on to the value.
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

// updateNewFieldValue handles the second step of the new-field flow. Enter adds
// the field to the table and queues it as an update.
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
			Type:  changeUpdate,
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

// updateConfirmDelete waits on the delete confirmation. Confirming queues a
// delete for the field under the cursor; dismissing changes nothing.
func (m FieldEditor) updateConfirmDelete(msg tea.Msg) (FieldEditor, tea.Cmd) {
	var cmd tea.Cmd
	m.confirm, cmd = m.confirm.Update(msg)

	if m.confirm.Confirmed() {
		idx := m.realIndex()
		if idx >= 0 {
			field := m.fields[idx]
			m.changes = append(m.changes, PendingChange{
				Type:  changeDelete,
				Field: field,
			})
		}
		m.mode = modeNormal
	} else if m.confirm.Dismissed() {
		m.mode = modeNormal
	}

	return m, cmd
}

// updateFilter handles the filter prompt. The table filters live as the user
// types, so Enter and Esc both just close the prompt.
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

// updateConfirmDiscard waits on the unsaved-changes confirmation. Confirming
// drops every pending change and runs the action that triggered the prompt.
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
//
//nolint:revive // strings.Builder Write methods always return nil error
func (m FieldEditor) View() string {
	var b strings.Builder

	m.renderHeader(&b)
	m.renderTable(&b)
	b.WriteString("\n")
	m.renderModeUI(&b)
	m.renderFooter(&b)

	return b.String()
}

// renderHeader writes the item title, its reference, and any active filter.
//
//nolint:revive // strings.Builder Write methods always return nil error
func (m FieldEditor) renderHeader(b *strings.Builder) {
	b.WriteString(Title.Render("Fields: " + m.itemName))
	b.WriteString("\n")
	b.WriteString(Subtitle.Render(m.itemRef))
	b.WriteString("\n")

	if m.filterText != "" && m.mode != modeFilter {
		b.WriteString(StatusBar.Render("  filter: " + m.filterText))
		b.WriteString("\n")
	}
	b.WriteString("\n")
}

// columnWidths measures the key and value columns across the visible fields.
// The value column is capped at maxValueWidth.
func (m FieldEditor) columnWidths(visible []int) (keyWidth, valWidth int) {
	for _, fi := range visible {
		f := m.fields[fi]
		if len(f.Key) > keyWidth {
			keyWidth = len(f.Key)
		}
		v := f.Value
		if f.Type == secrets.FieldConcealed && !m.revealed[fi] {
			v = concealedPlaceholder
		}
		if len(v) > valWidth {
			valWidth = len(v)
		}
	}
	if valWidth > maxValueWidth {
		valWidth = maxValueWidth
	}
	return keyWidth, valWidth
}

// displayValue returns the string shown in the value column for field index fi,
// masking concealed values and truncating anything wider than valWidth.
func (m FieldEditor) displayValue(fi, valWidth int) string {
	f := m.fields[fi]
	if f.Type == secrets.FieldConcealed && !m.revealed[fi] {
		return concealedPlaceholder
	}
	// Revealed secrets are shown in full even when they overflow the column.
	if f.Type == secrets.FieldConcealed {
		return f.Value
	}
	if len(f.Value) > valWidth {
		return f.Value[:valWidth-1] + "…"
	}
	return f.Value
}

// renderTable writes the field rows for the current viewport plus scroll hints.
//
//nolint:revive // strings.Builder Write methods always return nil error
func (m FieldEditor) renderTable(b *strings.Builder) {
	visible := m.visibleFields()
	endIdx := m.viewportOffset + m.fieldCapacity()
	if endIdx > len(visible) {
		endIdx = len(visible)
	}
	keyWidth, valWidth := m.columnWidths(visible)

	if m.viewportOffset > 0 {
		b.WriteString(Subtitle.Render(fmt.Sprintf("  ↑ %d more above", m.viewportOffset)))
		b.WriteString("\n")
	}

	rowFmt := fmt.Sprintf("%%s%%-%ds  %%-%ds  %%s", keyWidth, valWidth)
	for vi := m.viewportOffset; vi < endIdx; vi++ {
		fi := visible[vi]
		cursor := "  "
		if vi == m.cursor {
			cursor = "> "
		}
		line := fmt.Sprintf(rowFmt, cursor, m.fields[fi].Key, m.displayValue(fi, valWidth), string(m.fields[fi].Type))
		if vi == m.cursor {
			line = Selected.Render(line)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	if endIdx < len(visible) {
		b.WriteString(Subtitle.Render(fmt.Sprintf("  ↓ %d more below", len(visible)-endIdx)))
		b.WriteString("\n")
	}

	if len(visible) == 0 && m.filterText != "" {
		b.WriteString(Subtitle.Render("  No fields match filter"))
		b.WriteString("\n")
	}
}

// renderModeUI writes the prompt or confirmation belonging to the current mode.
//
//nolint:revive // strings.Builder Write methods always return nil error
func (m FieldEditor) renderModeUI(b *strings.Builder) {
	idx := m.realIndex()
	switch m.mode {
	case modeEdit:
		if idx >= 0 {
			b.WriteString("  Edit value for " + m.fields[idx].Key + ":\n")
		}
		b.WriteString("  " + m.input.View() + "\n")
	case modeRename:
		if idx >= 0 {
			b.WriteString("  Rename " + m.fields[idx].Key + ":\n")
		}
		b.WriteString("  " + m.input.View() + "\n")
	case modeNewFieldKey:
		b.WriteString("  New field key:\n")
		b.WriteString("  " + m.input.View() + "\n")
	case modeNewFieldValue:
		b.WriteString("  Value for " + m.newFieldKey + ":\n")
		b.WriteString("  " + m.input.View() + "\n")
	case modeFilter:
		b.WriteString("  Filter: ")
		b.WriteString(m.input.View())
		b.WriteString("\n")
	case modeConfirmDelete, modeConfirmDiscard:
		b.WriteString(m.confirm.View())
	}
}

// renderFooter writes the pending-change count and the key help line.
//
//nolint:revive // strings.Builder Write methods always return nil error
func (m FieldEditor) renderFooter(b *strings.Builder) {
	if changeCount := len(m.changes); changeCount > 0 {
		b.WriteString(StatusBar.Render(fmt.Sprintf("\n  %d pending change(s)", changeCount)))
		b.WriteString("\n")
	}

	helpText := "enter/e:edit  v:reveal  d:delete  r:rename  n:new  /:filter  s:save  esc:back  q:quit"
	if m.hasTypeEditor {
		helpText = "enter/e:edit  v:reveal  d:delete  r:rename  t:toggle  n:new  /:filter  s:save  esc:back  q:quit"
	}
	b.WriteString("\n")
	b.WriteString(Help.Render(helpText))
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

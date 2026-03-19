package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sentiolabs/envctl/internal/secrets"
)

type screen int

const (
	screenVaultPicker screen = iota
	screenItemList
	screenFieldEditor
)

// Options configures the root TUI model.
type Options struct {
	Editor secrets.Editor
	Vault  string // pre-select vault (skip picker)
	Item   string // pre-select item (skip to field editor)
}

// Model is the root Bubble Tea model that orchestrates screen transitions
// between VaultPicker, ItemList, and FieldEditor, and calls secrets.Editor
// methods for loading data and applying changes.
type Model struct {
	editor       secrets.Editor
	screen       screen
	vaultPicker  VaultPicker
	itemList     ItemList
	fieldEditor  FieldEditor
	currentVault *secrets.Vault
	currentItem  *secrets.Item
	loading      bool
	err          error
	width        int
	height       int
	presetVault  string
	presetItem   string
}

// Message types for async operations.
type vaultsLoadedMsg struct{ vaults []secrets.Vault }
type itemsLoadedMsg struct{ items []secrets.Item }
type fieldsLoadedMsg struct {
	fields   []secrets.Field
	itemRef  string
	itemName string
}
type saveCompleteMsg struct{ err error }
type errMsg struct{ err error }

// New creates a new root Model from the given Options. If Vault is set, it
// skips the vault picker. If both Vault and Item are set, it skips directly
// to the field editor.
func New(opts Options) Model {
	m := Model{
		editor:      opts.Editor,
		loading:     true,
		presetVault: opts.Vault,
		presetItem:  opts.Item,
	}

	switch {
	case opts.Vault != "" && opts.Item != "":
		m.screen = screenFieldEditor
	case opts.Vault != "":
		m.screen = screenItemList
		m.currentVault = &secrets.Vault{ID: opts.Vault, Name: opts.Vault}
	default:
		m.screen = screenVaultPicker
	}

	return m
}

// Init returns the command to load initial data for the starting screen.
func (m Model) Init() tea.Cmd {
	switch m.screen {
	case screenVaultPicker:
		return m.loadVaults()
	case screenItemList:
		return m.loadItems(m.presetVault)
	case screenFieldEditor:
		return m.loadFields(m.presetItem)
	}
	return nil
}

// Update processes messages and delegates to the active screen.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case vaultsLoadedMsg:
		m.loading = false
		m.vaultPicker = NewVaultPicker(msg.vaults)
		return m, nil

	case itemsLoadedMsg:
		m.loading = false
		m.itemList = NewItemList(m.vaultName(), msg.items)
		return m, nil

	case fieldsLoadedMsg:
		m.loading = false
		_, hasTypeEditor := m.editor.(secrets.FieldTypeEditor)
		m.fieldEditor = NewFieldEditor(msg.itemRef, msg.itemName, msg.fields, hasTypeEditor)
		return m, nil

	case saveCompleteMsg:
		if msg.err != nil {
			m.err = msg.err
			m.loading = false
			return m, nil
		}
		// Reload fields after successful save.
		m.loading = true
		return m, m.loadFields(m.currentItem.ID)

	case errMsg:
		m.err = msg.err
		m.loading = false
		return m, nil
	}

	if m.loading {
		return m, nil
	}

	switch m.screen {
	case screenVaultPicker:
		return m.updateVaultPicker(msg)
	case screenItemList:
		return m.updateItemList(msg)
	case screenFieldEditor:
		return m.updateFieldEditor(msg)
	}

	return m, nil
}

func (m Model) updateVaultPicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.vaultPicker, cmd = m.vaultPicker.Update(msg)

	if m.vaultPicker.Quitting() {
		return m, tea.Quit
	}

	if v := m.vaultPicker.Selected(); v != nil {
		m.currentVault = v
		m.screen = screenItemList
		m.loading = true
		return m, m.loadItems(v.ID)
	}

	return m, cmd
}

func (m Model) updateItemList(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.itemList, cmd = m.itemList.Update(msg)

	if m.itemList.Quitting() {
		return m, tea.Quit
	}

	if m.itemList.GoBack() {
		m.screen = screenVaultPicker
		m.loading = true
		return m, m.loadVaults()
	}

	if item := m.itemList.Selected(); item != nil {
		m.currentItem = item
		m.screen = screenFieldEditor
		m.loading = true
		return m, m.loadFields(item.ID)
	}

	return m, cmd
}

func (m Model) updateFieldEditor(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.fieldEditor, cmd = m.fieldEditor.Update(msg)

	if m.fieldEditor.Quitting() {
		changes := m.fieldEditor.PendingChanges()
		if len(changes) > 0 {
			return m, m.saveChanges(changes)
		}
		return m, tea.Quit
	}

	if m.fieldEditor.GoBack() {
		changes := m.fieldEditor.PendingChanges()
		if len(changes) > 0 {
			m.loading = true
			return m, m.saveChanges(changes)
		}
		m.screen = screenItemList
		m.loading = true
		return m, m.loadItems(m.currentVault.ID)
	}

	return m, cmd
}

// View renders the active screen, or a loading/error state.
func (m Model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress q to quit.", m.err)
	}

	if m.loading {
		return "Loading..."
	}

	switch m.screen {
	case screenVaultPicker:
		return m.vaultPicker.View()
	case screenItemList:
		return m.itemList.View()
	case screenFieldEditor:
		return m.fieldEditor.View()
	}

	return ""
}

// saveChanges returns a command that applies pending changes via the editor.
func (m Model) saveChanges(changes []PendingChange) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		for _, c := range changes {
			var err error
			switch c.Type {
			case "update":
				err = m.editor.UpdateField(ctx, m.currentItem.ID, c.Field)
			case "delete":
				err = m.editor.DeleteField(ctx, m.currentItem.ID, c.Field.Key)
			case "rename":
				err = m.editor.RenameField(ctx, m.currentItem.ID, c.OldKey, c.Field.Key)
			case "set_type":
				if fte, ok := m.editor.(secrets.FieldTypeEditor); ok {
					err = fte.SetFieldType(ctx, m.currentItem.ID, c.Field.Key, c.NewType)
				}
			}
			if err != nil {
				return saveCompleteMsg{err: err}
			}
		}
		return saveCompleteMsg{}
	}
}

// loadVaults returns a command that fetches vaults from the editor.
func (m Model) loadVaults() tea.Cmd {
	return func() tea.Msg {
		vaults, err := m.editor.ListVaults(context.Background())
		if err != nil {
			return errMsg{err: err}
		}
		return vaultsLoadedMsg{vaults: vaults}
	}
}

// loadItems returns a command that fetches items for a vault.
func (m Model) loadItems(vault string) tea.Cmd {
	return func() tea.Msg {
		items, err := m.editor.ListItems(context.Background(), vault)
		if err != nil {
			return errMsg{err: err}
		}
		return itemsLoadedMsg{items: items}
	}
}

// loadFields returns a command that fetches fields for an item.
func (m Model) loadFields(itemRef string) tea.Cmd {
	return func() tea.Msg {
		fields, err := m.editor.GetFields(context.Background(), itemRef)
		if err != nil {
			return errMsg{err: err}
		}
		return fieldsLoadedMsg{
			fields:   fields,
			itemRef:  itemRef,
			itemName: itemRef, // Use ref as name when we don't have a separate name
		}
	}
}

// vaultName returns the display name for the current vault.
func (m Model) vaultName() string {
	if m.currentVault != nil {
		return m.currentVault.Name
	}
	return ""
}

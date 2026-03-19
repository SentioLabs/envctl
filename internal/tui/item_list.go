package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sentiolabs/envctl/internal/secrets"
)

// itemListItem adapts secrets.Item for the bubbles list component.
type itemListItem struct {
	item secrets.Item
}

func (i itemListItem) Title() string       { return i.item.Name }
func (i itemListItem) Description() string { return i.item.ID }
func (i itemListItem) FilterValue() string { return i.item.Name }

// ItemList is a TUI screen that displays items in a vault.
// It supports selecting an item, creating a new item, navigating back,
// and quitting.
type ItemList struct {
	list     list.Model
	items    []secrets.Item
	vault    string
	selected *secrets.Item
	creating bool // in create-item mode
	input    textinput.Model
	newName  string // completed new item name
	back     bool
	quitting bool
}

// NewItemList creates an ItemList with the given vault name and items.
// The vault name is used as the list title.
func NewItemList(vault string, items []secrets.Item) ItemList {
	listItems := make([]list.Item, len(items))
	for i, item := range items {
		listItems[i] = itemListItem{item: item}
	}

	l := list.New(listItems, list.NewDefaultDelegate(), 80, 20)
	l.Title = fmt.Sprintf("Items in %s", vault)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)
	// Disable built-in quit key so we handle it ourselves.
	l.DisableQuitKeybindings()

	ti := textinput.New()
	ti.Placeholder = "Enter item name..."
	ti.CharLimit = 256

	return ItemList{
		list:  l,
		items: items,
		vault: vault,
		input: ti,
	}
}

// Init implements tea.Model. Returns nil.
func (m ItemList) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model. Handles key messages for item selection,
// create mode, back navigation, and quitting.
func (m ItemList) Update(msg tea.Msg) (ItemList, tea.Cmd) {
	if m.creating {
		return m.updateCreateMode(msg)
	}
	return m.updateNormalMode(msg)
}

func (m ItemList) updateNormalMode(msg tea.Msg) (ItemList, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Don't intercept keys when the list is filtering.
		if m.list.FilterState() == list.Filtering {
			break
		}
		switch msg.Type {
		case tea.KeyEnter:
			if item, ok := m.list.SelectedItem().(itemListItem); ok {
				selected := item.item
				m.selected = &selected
			}
			return m, nil
		case tea.KeyEscape:
			m.back = true
			return m, nil
		case tea.KeyRunes:
			switch string(msg.Runes) {
			case "q":
				m.quitting = true
				return m, tea.Quit
			case "n":
				m.creating = true
				m.input.Focus()
				return m, textinput.Blink
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m ItemList) updateCreateMode(msg tea.Msg) (ItemList, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			m.newName = m.input.Value()
			m.creating = false
			m.input.Reset()
			return m, nil
		case tea.KeyEscape:
			m.creating = false
			m.input.Reset()
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// View implements tea.Model. Renders the list or the create-item input.
func (m ItemList) View() string {
	if m.creating {
		return fmt.Sprintf("New item name:\n\n%s\n\n(enter to confirm, esc to cancel)", m.input.View())
	}
	return m.list.View()
}

// Selected returns the selected item, or nil if no item has been selected.
func (m ItemList) Selected() *secrets.Item {
	return m.selected
}

// GoBack returns true if the user pressed Esc to go back to the vault picker.
func (m ItemList) GoBack() bool {
	return m.back
}

// NewItemName returns the name entered by the user in create mode,
// or an empty string if the create flow was not completed.
func (m ItemList) NewItemName() string {
	return m.newName
}

// Quitting returns true if the user pressed q to quit.
func (m ItemList) Quitting() bool {
	return m.quitting
}

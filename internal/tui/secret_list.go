// Package tui provides terminal user interface components for envctl.
package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

// sourceItem adapts Source for the bubbles list.Model.
type sourceItem struct{ source Source }

func (i sourceItem) Title() string       { return i.source.Name }
func (i sourceItem) Description() string { return "[" + i.source.Backend + "]" }
func (i sourceItem) FilterValue() string { return i.source.Name }

// SecretList presents a filterable list of secret sources for the user to select.
type SecretList struct {
	list     list.Model
	sources  []Source
	selected *Source
	back     bool
	browse   bool // true when user pressed 'b'
	quitting bool
	appEnv   string // display string like "core-api / local"
}

// NewSecretList creates a SecretList initialized with the given sources.
func NewSecretList(appEnv string, sources []Source) SecretList {
	items := make([]list.Item, len(sources))
	for i, s := range sources {
		items[i] = sourceItem{source: s}
	}

	const (
		defaultWidth  = 40
		defaultHeight = 20
	)

	l := list.New(items, list.NewDefaultDelegate(), defaultWidth, defaultHeight)
	l.Title = fmt.Sprintf("Secrets: %s", appEnv)
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)

	return SecretList{
		list:    l,
		sources: sources,
		appEnv:  appEnv,
	}
}

// Init implements tea.Model. Returns nil (no initial command).
func (m SecretList) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model. Handles key events for selection, navigation, and quitting.
func (m SecretList) Update(msg tea.Msg) (SecretList, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Don't intercept keys when the list is filtering.
		if m.list.FilterState() == list.Filtering {
			break
		}

		switch msg.Type {
		case tea.KeyEnter:
			if item, ok := m.list.SelectedItem().(sourceItem); ok {
				src := item.source
				m.selected = &src
			}
			return m, nil
		case tea.KeyEsc:
			m.back = true
			return m, nil
		}

		switch msg.String() {
		case "q":
			m.quitting = true
			return m, nil
		case "b":
			m.browse = true
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// View implements tea.Model. Renders the secret source list.
func (m SecretList) View() string {
	return m.list.View()
}

// Selected returns the selected source, or nil if none has been selected.
func (m SecretList) Selected() *Source {
	return m.selected
}

// GoBack returns true if the user has signaled they want to go back.
func (m SecretList) GoBack() bool {
	return m.back
}

// BrowseMode returns true if the user has signaled they want to switch to browse mode.
func (m SecretList) BrowseMode() bool {
	return m.browse
}

// Quitting returns true if the user has signaled they want to quit.
func (m SecretList) Quitting() bool {
	return m.quitting
}

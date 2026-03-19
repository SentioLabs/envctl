// Package tui provides terminal user interface components for envctl.
package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

// appItem adapts a string app name for the bubbles list.Model.
type appItem struct{ name string }

func (i appItem) Title() string       { return i.name }
func (i appItem) Description() string { return "" }
func (i appItem) FilterValue() string { return i.name }

// AppPicker presents a filterable list of application names for the user to select.
type AppPicker struct {
	list       list.Model
	apps       []string
	selected   string
	quitting   bool
	autoSelect bool
}

// NewAppPicker creates an AppPicker initialized with the given app names.
// If there is only one app, it is auto-selected immediately.
// If defaultApp matches one of the apps, the cursor is moved to it.
func NewAppPicker(apps []string, defaultApp string) AppPicker {
	// Auto-select when there is exactly one app.
	if len(apps) == 1 {
		return AppPicker{
			apps:       apps,
			selected:   apps[0],
			autoSelect: true,
		}
	}

	items := make([]list.Item, len(apps))
	for i, a := range apps {
		items[i] = appItem{name: a}
	}

	const (
		defaultWidth  = 40
		defaultHeight = 20
	)

	l := list.New(items, list.NewDefaultDelegate(), defaultWidth, defaultHeight)
	l.Title = "Select application:"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)

	// Move cursor to defaultApp if it matches.
	if defaultApp != "" {
		for i, a := range apps {
			if a == defaultApp {
				l.Select(i)
				break
			}
		}
	}

	return AppPicker{
		list: l,
		apps: apps,
	}
}

// Init implements tea.Model. Returns nil (no initial command).
func (m AppPicker) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model. Handles key events for selection and quitting.
func (m AppPicker) Update(msg tea.Msg) (AppPicker, tea.Cmd) {
	// If auto-selected, nothing to do.
	if m.autoSelect {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Don't intercept keys when the list is filtering.
		if m.list.FilterState() == list.Filtering {
			break
		}

		switch msg.Type {
		case tea.KeyEnter:
			if item, ok := m.list.SelectedItem().(appItem); ok {
				m.selected = item.name
			}
			return m, nil
		}

		switch msg.String() {
		case "q":
			m.quitting = true
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// View implements tea.Model. Renders the app list.
func (m AppPicker) View() string {
	if m.autoSelect {
		return fmt.Sprintf("Auto-selected application: %s", m.selected)
	}
	return m.list.View()
}

// Selected returns the selected app name, or empty string if none has been selected.
func (m AppPicker) Selected() string {
	return m.selected
}

// Quitting returns true if the user has signaled they want to quit.
func (m AppPicker) Quitting() bool {
	return m.quitting
}

// Package tui provides terminal user interface components for envctl.
package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

// envItem adapts a string env name for the bubbles list.Model.
type envItem struct{ name string }

func (i envItem) Title() string       { return i.name }
func (i envItem) Description() string { return "" }
func (i envItem) FilterValue() string { return i.name }

// EnvPicker presents a filterable list of environment names for the user to select.
type EnvPicker struct {
	list     list.Model
	envs     []string
	appName  string
	selected string
	back     bool
	quitting bool
}

// NewEnvPicker creates an EnvPicker initialized with the given environment names.
// The title includes the app name (e.g., "core-api — Select environment:").
// If defaultEnv matches one of the envs, the cursor is moved to it.
func NewEnvPicker(appName string, envs []string, defaultEnv string) EnvPicker {
	items := make([]list.Item, len(envs))
	for i, e := range envs {
		items[i] = envItem{name: e}
	}

	const (
		defaultWidth  = 40
		defaultHeight = 20
	)

	l := list.New(items, list.NewDefaultDelegate(), defaultWidth, defaultHeight)
	l.Title = fmt.Sprintf("%s — Select environment:", appName)
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)

	// Move cursor to defaultEnv if it matches.
	if defaultEnv != "" {
		for i, e := range envs {
			if e == defaultEnv {
				l.Select(i)
				break
			}
		}
	}

	return EnvPicker{
		list:    l,
		envs:    envs,
		appName: appName,
	}
}

// Init implements tea.Model. Returns nil (no initial command).
func (m EnvPicker) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model. Handles key events for selection, back, and quitting.
func (m EnvPicker) Update(msg tea.Msg) (EnvPicker, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Don't intercept keys when the list is filtering.
		if m.list.FilterState() == list.Filtering {
			break
		}

		switch msg.Type {
		case tea.KeyEnter:
			if item, ok := m.list.SelectedItem().(envItem); ok {
				m.selected = item.name
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
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// View implements tea.Model. Renders the environment list.
func (m EnvPicker) View() string {
	return m.list.View()
}

// Selected returns the selected environment name, or empty string if none has been selected.
func (m EnvPicker) Selected() string {
	return m.selected
}

// GoBack returns true if the user has signaled they want to go back.
func (m EnvPicker) GoBack() bool {
	return m.back
}

// Quitting returns true if the user has signaled they want to quit.
func (m EnvPicker) Quitting() bool {
	return m.quitting
}

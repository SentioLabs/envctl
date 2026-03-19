package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// Confirm is a small overlay model that asks the user to confirm or dismiss
// a destructive action by pressing y or n/esc.
type Confirm struct {
	Message   string
	confirmed bool
	dismissed bool
}

// NewConfirm creates a new confirmation overlay with the given message.
func NewConfirm(message string) Confirm {
	return Confirm{
		Message: message,
	}
}

// Update handles key messages: y confirms, n or esc dismisses.
func (m Confirm) Update(msg tea.Msg) (Confirm, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.Type {
		case tea.KeyEscape:
			m.dismissed = true
		case tea.KeyRunes:
			switch string(msg.Runes) {
			case "y":
				m.confirmed = true
			case "n":
				m.dismissed = true
			}
		}
	}
	return m, nil
}

// View renders the confirmation prompt.
func (m Confirm) View() string {
	return fmt.Sprintf("\n  %s (y/n)\n", m.Message)
}

// Confirmed returns true if the user pressed y.
func (m Confirm) Confirmed() bool {
	return m.confirmed
}

// Dismissed returns true if the user pressed n or esc.
func (m Confirm) Dismissed() bool {
	return m.dismissed
}

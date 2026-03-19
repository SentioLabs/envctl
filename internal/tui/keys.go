package tui

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines the key bindings used across all TUI screens.
type KeyMap struct {
	Navigate   key.Binding
	Select     key.Binding
	Back       key.Binding
	Quit       key.Binding
	Filter     key.Binding
	Edit       key.Binding
	Delete     key.Binding
	Rename     key.Binding
	ToggleType key.Binding
	NewField   key.Binding
	NewItem    key.Binding
	Save       key.Binding
}

// Keys is the default set of key bindings for all TUI screens.
var Keys = KeyMap{
	Navigate:   key.NewBinding(key.WithKeys("up", "down"), key.WithHelp("↑/↓", "navigate")),
	Select:     key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
	Back:       key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	Quit:       key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
	Filter:     key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
	Edit:       key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
	Delete:     key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
	Rename:     key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "rename")),
	ToggleType: key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "toggle type")),
	NewField:   key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new field")),
	NewItem:    key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new item")),
	Save:       key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "save")),
}

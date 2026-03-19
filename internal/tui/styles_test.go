package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
)

func TestStyles_RenderNonEmpty(t *testing.T) {
	tests := []struct {
		name  string
		style lipgloss.Style
	}{
		{"Title", Title},
		{"Subtitle", Subtitle},
		{"Help", Help},
		{"Selected", Selected},
		{"Error", Error},
		{"StatusBar", StatusBar},
		{"Concealed", Concealed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.style.Render("test")
			assert.NotEmpty(t, result, "style %s should render non-empty string", tt.name)
		})
	}
}

func TestKeyBindings_HaveKeysAndHelp(t *testing.T) {
	bindings := []struct {
		name    string
		binding key.Binding
	}{
		{"Navigate", Keys.Navigate},
		{"Select", Keys.Select},
		{"Back", Keys.Back},
		{"Quit", Keys.Quit},
		{"Filter", Keys.Filter},
		{"Edit", Keys.Edit},
		{"Delete", Keys.Delete},
		{"Rename", Keys.Rename},
		{"ToggleType", Keys.ToggleType},
		{"NewField", Keys.NewField},
		{"NewItem", Keys.NewItem},
		{"Save", Keys.Save},
	}

	for _, tt := range bindings {
		t.Run(tt.name, func(t *testing.T) {
			help := tt.binding.Help()
			assert.NotEmpty(t, help.Key, "binding %s should have non-empty key help", tt.name)
			assert.NotEmpty(t, help.Desc, "binding %s should have non-empty description", tt.name)
		})
	}
}

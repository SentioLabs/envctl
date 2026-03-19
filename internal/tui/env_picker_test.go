package tui_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sentiolabs/envctl/internal/tui"
)

func testEnvs() []string {
	return []string{"dev", "staging", "production"}
}

func TestNewEnvPicker(t *testing.T) {
	t.Run("initializes with envs and app name", func(t *testing.T) {
		envs := testEnvs()
		picker := tui.NewEnvPicker("core-api", envs, "")

		if picker.Selected() != "" {
			t.Error("expected no env selected initially")
		}
		if picker.Quitting() {
			t.Error("expected quitting to be false initially")
		}
		if picker.GoBack() {
			t.Error("expected back to be false initially")
		}
	})

	t.Run("initializes with empty envs", func(t *testing.T) {
		picker := tui.NewEnvPicker("core-api", nil, "")

		if picker.Selected() != "" {
			t.Error("expected no env selected initially")
		}
	})
}

func TestEnvPickerUpdate(t *testing.T) {
	t.Run("pressing Enter selects the current env", func(t *testing.T) {
		envs := testEnvs()
		picker := tui.NewEnvPicker("core-api", envs, "")

		updated, _ := picker.Update(tea.KeyMsg{Type: tea.KeyEnter})

		if updated.Selected() != "dev" {
			t.Errorf("expected selected env %q, got %q", "dev", updated.Selected())
		}
	})

	t.Run("pressing Esc signals back", func(t *testing.T) {
		envs := testEnvs()
		picker := tui.NewEnvPicker("core-api", envs, "")

		updated, _ := picker.Update(tea.KeyMsg{Type: tea.KeyEsc})

		if !updated.GoBack() {
			t.Error("expected back to be true after pressing Esc")
		}
		if updated.Selected() != "" {
			t.Error("expected no env selected after going back")
		}
	})

	t.Run("pressing q signals quit", func(t *testing.T) {
		envs := testEnvs()
		picker := tui.NewEnvPicker("core-api", envs, "")

		updated, _ := picker.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

		if !updated.Quitting() {
			t.Error("expected quitting to be true after pressing q")
		}
		if updated.Selected() != "" {
			t.Error("expected no env selected after quitting")
		}
	})

	t.Run("navigating down then pressing Enter selects second env", func(t *testing.T) {
		envs := testEnvs()
		picker := tui.NewEnvPicker("core-api", envs, "")

		updated, _ := picker.Update(tea.KeyMsg{Type: tea.KeyDown})
		updated, _ = updated.Update(tea.KeyMsg{Type: tea.KeyEnter})

		if updated.Selected() != "staging" {
			t.Errorf("expected selected env %q, got %q", "staging", updated.Selected())
		}
	})
}

func TestEnvPickerView(t *testing.T) {
	t.Run("includes env names in output", func(t *testing.T) {
		envs := testEnvs()
		picker := tui.NewEnvPicker("core-api", envs, "")

		view := picker.View()

		for _, env := range envs {
			if !strings.Contains(view, env) {
				t.Errorf("expected view to contain env name %q, got:\n%s", env, view)
			}
		}
	})

	t.Run("includes app name in title", func(t *testing.T) {
		picker := tui.NewEnvPicker("core-api", testEnvs(), "")

		view := picker.View()

		if !strings.Contains(view, "core-api") {
			t.Errorf("expected view to contain app name %q, got:\n%s", "core-api", view)
		}
	})
}

func TestEnvPickerInit(t *testing.T) {
	t.Run("returns nil command", func(t *testing.T) {
		picker := tui.NewEnvPicker("core-api", testEnvs(), "")

		cmd := picker.Init()
		if cmd != nil {
			t.Error("expected Init to return nil command")
		}
	})
}

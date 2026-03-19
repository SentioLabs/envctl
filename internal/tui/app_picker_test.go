package tui_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sentiolabs/envctl/internal/tui"
)

func testApps() []string {
	return []string{"core-api", "web-frontend", "worker"}
}

func TestNewAppPicker(t *testing.T) {
	t.Run("initializes with multiple apps", func(t *testing.T) {
		apps := testApps()
		picker := tui.NewAppPicker(apps, "")

		if picker.Selected() != "" {
			t.Error("expected no app selected initially")
		}
		if picker.Quitting() {
			t.Error("expected quitting to be false initially")
		}
	})

	t.Run("single app auto-selects immediately", func(t *testing.T) {
		picker := tui.NewAppPicker([]string{"only-app"}, "")

		if picker.Selected() != "only-app" {
			t.Errorf("expected single app to be auto-selected, got %q", picker.Selected())
		}
	})

	t.Run("initializes with empty apps", func(t *testing.T) {
		picker := tui.NewAppPicker(nil, "")

		if picker.Selected() != "" {
			t.Error("expected no app selected initially")
		}
	})
}

func TestAppPickerUpdate(t *testing.T) {
	t.Run("pressing Enter selects the current app", func(t *testing.T) {
		apps := testApps()
		picker := tui.NewAppPicker(apps, "")

		updated, _ := picker.Update(tea.KeyMsg{Type: tea.KeyEnter})

		if updated.Selected() != "core-api" {
			t.Errorf("expected selected app %q, got %q", "core-api", updated.Selected())
		}
	})

	t.Run("pressing q signals quit", func(t *testing.T) {
		apps := testApps()
		picker := tui.NewAppPicker(apps, "")

		updated, _ := picker.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

		if !updated.Quitting() {
			t.Error("expected quitting to be true after pressing q")
		}
		if updated.Selected() != "" {
			t.Error("expected no app selected after quitting")
		}
	})

	t.Run("navigating down then pressing Enter selects second app", func(t *testing.T) {
		apps := testApps()
		picker := tui.NewAppPicker(apps, "")

		updated, _ := picker.Update(tea.KeyMsg{Type: tea.KeyDown})
		updated, _ = updated.Update(tea.KeyMsg{Type: tea.KeyEnter})

		if updated.Selected() != "web-frontend" {
			t.Errorf("expected selected app %q, got %q", "web-frontend", updated.Selected())
		}
	})
}

func TestAppPickerView(t *testing.T) {
	t.Run("includes app names in output", func(t *testing.T) {
		apps := testApps()
		picker := tui.NewAppPicker(apps, "")

		view := picker.View()

		for _, app := range apps {
			if !strings.Contains(view, app) {
				t.Errorf("expected view to contain app name %q, got:\n%s", app, view)
			}
		}
	})
}

func TestAppPickerInit(t *testing.T) {
	t.Run("returns nil command", func(t *testing.T) {
		picker := tui.NewAppPicker(testApps(), "")

		cmd := picker.Init()
		if cmd != nil {
			t.Error("expected Init to return nil command")
		}
	})
}

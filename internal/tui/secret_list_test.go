package tui_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sentiolabs/envctl/internal/tui"
)

func testSources() []tui.Source {
	return []tui.Source{
		{Name: "myapp/dev/db", Backend: "aws"},
		{Name: "BACstack Local - Core API", Backend: "1pass"},
		{Name: "myapp/dev/cache", Backend: "aws"},
	}
}

func TestNewSecretList(t *testing.T) {
	t.Run("initializes with sources and appEnv in title", func(t *testing.T) {
		sources := testSources()
		sl := tui.NewSecretList("core-api / local", sources)

		if sl.Selected() != nil {
			t.Error("expected no source selected initially")
		}
		if sl.GoBack() {
			t.Error("expected GoBack to be false initially")
		}
		if sl.BrowseMode() {
			t.Error("expected BrowseMode to be false initially")
		}
		if sl.Quitting() {
			t.Error("expected Quitting to be false initially")
		}

		view := sl.View()
		if !strings.Contains(view, "core-api / local") {
			t.Errorf("expected view to contain appEnv in title, got:\n%s", view)
		}
	})

	t.Run("initializes with empty sources", func(t *testing.T) {
		sl := tui.NewSecretList("app / env", nil)

		if sl.Selected() != nil {
			t.Error("expected no source selected initially")
		}
	})
}

func TestSecretListUpdate(t *testing.T) {
	t.Run("pressing Enter selects the current source", func(t *testing.T) {
		sources := testSources()
		sl := tui.NewSecretList("core-api / local", sources)

		updated, _ := sl.Update(tea.KeyMsg{Type: tea.KeyEnter})

		if updated.Selected() == nil {
			t.Fatal("expected a source to be selected after pressing Enter")
		}
		if updated.Selected().Name != "myapp/dev/db" {
			t.Errorf("expected selected source name %q, got %q", "myapp/dev/db", updated.Selected().Name)
		}
		if updated.Selected().Backend != "aws" {
			t.Errorf("expected selected source backend %q, got %q", "aws", updated.Selected().Backend)
		}
	})

	t.Run("pressing Esc signals GoBack", func(t *testing.T) {
		sources := testSources()
		sl := tui.NewSecretList("core-api / local", sources)

		updated, _ := sl.Update(tea.KeyMsg{Type: tea.KeyEsc})

		if !updated.GoBack() {
			t.Error("expected GoBack to be true after pressing Esc")
		}
		if updated.Selected() != nil {
			t.Error("expected no source selected after going back")
		}
	})

	t.Run("pressing b signals BrowseMode", func(t *testing.T) {
		sources := testSources()
		sl := tui.NewSecretList("core-api / local", sources)

		updated, _ := sl.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})

		if !updated.BrowseMode() {
			t.Error("expected BrowseMode to be true after pressing b")
		}
	})

	t.Run("pressing q signals quit", func(t *testing.T) {
		sources := testSources()
		sl := tui.NewSecretList("core-api / local", sources)

		updated, _ := sl.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

		if !updated.Quitting() {
			t.Error("expected Quitting to be true after pressing q")
		}
		if updated.Selected() != nil {
			t.Error("expected no source selected after quitting")
		}
	})

	t.Run("navigating down then pressing Enter selects second source", func(t *testing.T) {
		sources := testSources()
		sl := tui.NewSecretList("core-api / local", sources)

		updated, _ := sl.Update(tea.KeyMsg{Type: tea.KeyDown})
		updated, _ = updated.Update(tea.KeyMsg{Type: tea.KeyEnter})

		if updated.Selected() == nil {
			t.Fatal("expected a source to be selected")
		}
		if updated.Selected().Name != "BACstack Local - Core API" {
			t.Errorf("expected selected source name %q, got %q", "BACstack Local - Core API", updated.Selected().Name)
		}
	})
}

func TestSecretListView(t *testing.T) {
	t.Run("includes source names and backend badges in output", func(t *testing.T) {
		sources := testSources()
		sl := tui.NewSecretList("core-api / local", sources)

		view := sl.View()

		for _, s := range sources {
			if !strings.Contains(view, s.Name) {
				t.Errorf("expected view to contain source name %q, got:\n%s", s.Name, view)
			}
			badge := "[" + s.Backend + "]"
			if !strings.Contains(view, badge) {
				t.Errorf("expected view to contain backend badge %q, got:\n%s", badge, view)
			}
		}
	})
}

func TestSecretListInit(t *testing.T) {
	t.Run("returns nil command", func(t *testing.T) {
		sl := tui.NewSecretList("core-api / local", testSources())

		cmd := sl.Init()
		if cmd != nil {
			t.Error("expected Init to return nil command")
		}
	})
}

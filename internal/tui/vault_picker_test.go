package tui_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sentiolabs/envctl/internal/secrets"
	"github.com/sentiolabs/envctl/internal/tui"
)

func testVaults() []secrets.Vault {
	return []secrets.Vault{
		{ID: "vault-1", Name: "Development"},
		{ID: "vault-2", Name: "Staging"},
		{ID: "vault-3", Name: "Production"},
	}
}

func TestNewVaultPicker(t *testing.T) {
	t.Run("initializes with a list of vaults", func(t *testing.T) {
		vaults := testVaults()
		picker := tui.NewVaultPicker(vaults)

		if picker.Selected() != nil {
			t.Error("expected no vault selected initially")
		}
		if picker.Quitting() {
			t.Error("expected quitting to be false initially")
		}
	})

	t.Run("initializes with empty vaults", func(t *testing.T) {
		picker := tui.NewVaultPicker(nil)

		if picker.Selected() != nil {
			t.Error("expected no vault selected initially")
		}
	})
}

func TestVaultPickerUpdate(t *testing.T) {
	t.Run("pressing Enter selects the current vault", func(t *testing.T) {
		vaults := testVaults()
		picker := tui.NewVaultPicker(vaults)

		// Press Enter to select the first vault (default cursor position).
		updated, _ := picker.Update(tea.KeyMsg{Type: tea.KeyEnter})

		if updated.Selected() == nil {
			t.Fatal("expected a vault to be selected after pressing Enter")
		}
		if updated.Selected().Name != "Development" {
			t.Errorf("expected selected vault name %q, got %q", "Development", updated.Selected().Name)
		}
		if updated.Selected().ID != "vault-1" {
			t.Errorf("expected selected vault ID %q, got %q", "vault-1", updated.Selected().ID)
		}
	})

	t.Run("pressing q signals quit", func(t *testing.T) {
		vaults := testVaults()
		picker := tui.NewVaultPicker(vaults)

		updated, _ := picker.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

		if !updated.Quitting() {
			t.Error("expected quitting to be true after pressing q")
		}
		if updated.Selected() != nil {
			t.Error("expected no vault selected after quitting")
		}
	})

	t.Run("navigating down then pressing Enter selects second vault", func(t *testing.T) {
		vaults := testVaults()
		picker := tui.NewVaultPicker(vaults)

		// Move down one item.
		updated, _ := picker.Update(tea.KeyMsg{Type: tea.KeyDown})
		// Press Enter.
		updated, _ = updated.Update(tea.KeyMsg{Type: tea.KeyEnter})

		if updated.Selected() == nil {
			t.Fatal("expected a vault to be selected")
		}
		if updated.Selected().Name != "Staging" {
			t.Errorf("expected selected vault name %q, got %q", "Staging", updated.Selected().Name)
		}
	})
}

func TestVaultPickerView(t *testing.T) {
	t.Run("includes vault names in output", func(t *testing.T) {
		vaults := testVaults()
		picker := tui.NewVaultPicker(vaults)

		view := picker.View()

		for _, v := range vaults {
			if !strings.Contains(view, v.Name) {
				t.Errorf("expected view to contain vault name %q, got:\n%s", v.Name, view)
			}
		}
	})
}

func TestVaultPickerInit(t *testing.T) {
	t.Run("returns nil command", func(t *testing.T) {
		picker := tui.NewVaultPicker(testVaults())

		cmd := picker.Init()
		if cmd != nil {
			t.Error("expected Init to return nil command")
		}
	})
}

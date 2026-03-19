// Package tui provides terminal user interface components for envctl.
package tui

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sentiolabs/envctl/internal/secrets"
)

// vaultItem adapts secrets.Vault for the bubbles list.Model.
type vaultItem struct{ vault secrets.Vault }

func (i vaultItem) Title() string       { return i.vault.Name }
func (i vaultItem) Description() string { return i.vault.ID }
func (i vaultItem) FilterValue() string { return i.vault.Name }

// VaultPicker presents a filterable list of vaults for the user to select.
type VaultPicker struct {
	list     list.Model
	vaults   []secrets.Vault
	selected *secrets.Vault
	quitting bool
}

// NewVaultPicker creates a VaultPicker initialized with the given vaults.
func NewVaultPicker(vaults []secrets.Vault) VaultPicker {
	items := make([]list.Item, len(vaults))
	for i, v := range vaults {
		items[i] = vaultItem{vault: v}
	}

	const (
		defaultWidth  = 40
		defaultHeight = 20
	)

	l := list.New(items, list.NewDefaultDelegate(), defaultWidth, defaultHeight)
	l.Title = "Select a Vault"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)

	return VaultPicker{
		list:   l,
		vaults: vaults,
	}
}

// Init implements tea.Model. Returns nil (no initial command).
func (v VaultPicker) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model. Handles key events for selection and quitting.
func (v VaultPicker) Update(msg tea.Msg) (VaultPicker, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Don't intercept keys when the list is filtering.
		if v.list.FilterState() == list.Filtering {
			break
		}

		switch msg.Type {
		case tea.KeyEnter:
			if item, ok := v.list.SelectedItem().(vaultItem); ok {
				vault := item.vault
				v.selected = &vault
			}
			return v, nil
		}

		switch msg.String() {
		case "q":
			v.quitting = true
			return v, nil
		}
	}

	var cmd tea.Cmd
	v.list, cmd = v.list.Update(msg)
	return v, cmd
}

// View implements tea.Model. Renders the vault list.
func (v VaultPicker) View() string {
	return v.list.View()
}

// Selected returns the selected vault, or nil if none has been selected.
func (v VaultPicker) Selected() *secrets.Vault {
	return v.selected
}

// Quitting returns true if the user has signaled they want to quit.
func (v VaultPicker) Quitting() bool {
	return v.quitting
}

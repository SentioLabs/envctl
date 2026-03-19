package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sentiolabs/envctl/internal/secrets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testItems() []secrets.Item {
	return []secrets.Item{
		{ID: "item-1", Name: "database-creds"},
		{ID: "item-2", Name: "api-keys"},
		{ID: "item-3", Name: "tls-cert"},
	}
}

func TestNewItemList_InitializesWithItemsAndVaultTitle(t *testing.T) {
	items := testItems()
	il := NewItemList("my-vault", items)

	assert.Equal(t, "my-vault", il.vault)
	assert.Equal(t, items, il.items)
	assert.Nil(t, il.selected)
	assert.False(t, il.creating)
	assert.False(t, il.back)
	assert.False(t, il.quitting)
	assert.Empty(t, il.newName)
}

func TestItemList_Init_ReturnsNil(t *testing.T) {
	il := NewItemList("vault", testItems())
	cmd := il.Init()
	assert.Nil(t, cmd)
}

func TestItemList_EnterSelectsItem(t *testing.T) {
	items := testItems()
	il := NewItemList("vault", items)

	// The list should have items; press Enter to select the first one.
	updated, _ := il.Update(tea.KeyMsg{Type: tea.KeyEnter})

	require.NotNil(t, updated.Selected())
	assert.Equal(t, "database-creds", updated.Selected().Name)
	assert.Equal(t, "item-1", updated.Selected().ID)
}

func TestItemList_PressN_EntersCreateMode(t *testing.T) {
	il := NewItemList("vault", testItems())

	updated, _ := il.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})

	assert.True(t, updated.creating)
	assert.Empty(t, updated.NewItemName())
}

func TestItemList_CreateMode_EnterConfirmsName(t *testing.T) {
	il := NewItemList("vault", testItems())

	// Enter create mode
	il, _ = il.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	require.True(t, il.creating)

	// Type a name
	for _, r := range "new-secret" {
		il, _ = il.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	// Confirm with Enter
	il, _ = il.Update(tea.KeyMsg{Type: tea.KeyEnter})

	assert.False(t, il.creating)
	assert.Equal(t, "new-secret", il.NewItemName())
}

func TestItemList_CreateMode_EscCancels(t *testing.T) {
	il := NewItemList("vault", testItems())

	// Enter create mode
	il, _ = il.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	require.True(t, il.creating)

	// Type something
	for _, r := range "partial" {
		il, _ = il.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	// Cancel with Esc
	il, _ = il.Update(tea.KeyMsg{Type: tea.KeyEscape})

	assert.False(t, il.creating)
	assert.Empty(t, il.NewItemName())
}

func TestItemList_EscSignalsBack(t *testing.T) {
	il := NewItemList("vault", testItems())

	updated, _ := il.Update(tea.KeyMsg{Type: tea.KeyEscape})

	assert.True(t, updated.GoBack())
}

func TestItemList_QSignalsQuit(t *testing.T) {
	il := NewItemList("vault", testItems())

	updated, _ := il.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

	assert.True(t, updated.Quitting())
}

func TestItemList_ViewIncludesItemNames(t *testing.T) {
	il := NewItemList("vault", testItems())
	view := il.View()

	assert.Contains(t, view, "database-creds")
	assert.Contains(t, view, "api-keys")
	assert.Contains(t, view, "tls-cert")
}

func TestItemList_ViewInCreateMode_ShowsInput(t *testing.T) {
	il := NewItemList("vault", testItems())
	il, _ = il.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})

	view := il.View()

	assert.Contains(t, view, "New item name")
}

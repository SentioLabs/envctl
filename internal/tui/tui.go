package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sentiolabs/envctl/internal/secrets"
)

type screen int

const (
	screenAppPicker screen = iota
	screenEnvPicker
	screenSecretList
	screenVaultPicker
	screenItemList
	screenFieldEditor
)

// Options configures the root TUI model.
type Options struct {
	Editor        secrets.Editor
	EditorFactory func(ctx context.Context, backend string) (secrets.Editor, error)
	Config        *ConfigContext
	Vault         string // pre-select vault (skip picker)
	Item          string // pre-select item (skip to field editor)
	App           string // pre-select app (skip app picker)
	Env           string // pre-select env (skip env picker)
}

// Model is the root Bubble Tea model that orchestrates screen transitions
// between VaultPicker, ItemList, and FieldEditor, and calls secrets.Editor
// methods for loading data and applying changes.
type Model struct {
	editor        secrets.Editor
	editorFactory func(ctx context.Context, backend string) (secrets.Editor, error)
	configCtx     *ConfigContext
	screen        screen
	vaultPicker   VaultPicker
	itemList      ItemList
	fieldEditor   FieldEditor
	appPicker     AppPicker
	envPicker     EnvPicker
	secretList    SecretList
	currentVault  *secrets.Vault
	currentItem   *secrets.Item
	currentApp       string
	currentEnv       string
	currentSourceRef string // secret ref being edited in config mode
	loading          bool
	err           error
	width         int
	height        int
	presetVault   string
	presetItem    string
}

// Message types for async operations.
type vaultsLoadedMsg struct{ vaults []secrets.Vault }
type itemsLoadedMsg struct{ items []secrets.Item }
type fieldsLoadedMsg struct {
	fields   []secrets.Field
	itemRef  string
	itemName string
}
type saveCompleteMsg struct{ err error }
type errMsg struct{ err error }

// editorCreatedMsg is sent when an EditorFactory call completes.
type editorCreatedMsg struct {
	editor secrets.Editor
	source Source
}

// New creates a new root Model from the given Options. If Config is set, uses
// config-driven flow (app picker -> env picker -> secret list -> field editor).
// If Config is nil, uses browse flow (vault picker -> item list -> field editor).
func New(opts Options) Model {
	m := Model{
		editor:        opts.Editor,
		editorFactory: opts.EditorFactory,
		configCtx:     opts.Config,
		presetVault:   opts.Vault,
		presetItem:    opts.Item,
	}

	if opts.Config != nil {
		return newConfigMode(m, opts)
	}

	return newBrowseMode(m, opts)
}

// newConfigMode initializes the model for config-driven flow.
func newConfigMode(m Model, opts Options) Model {
	switch {
	case opts.App != "" && opts.Env != "":
		m.currentApp = opts.App
		m.currentEnv = opts.Env
		key := opts.App + "/" + opts.Env
		sources := opts.Config.Sources[key]
		appEnv := opts.App + " / " + opts.Env
		m.secretList = NewSecretList(appEnv, sources)
		m.screen = screenSecretList

	case opts.App != "":
		m.currentApp = opts.App
		envs := opts.Config.Envs[opts.App]
		m.envPicker = NewEnvPicker(opts.App, envs, opts.Config.DefaultEnv)
		m.screen = screenEnvPicker

	case len(opts.Config.Apps) == 1:
		m.currentApp = opts.Config.Apps[0]
		envs := opts.Config.Envs[m.currentApp]
		m.envPicker = NewEnvPicker(m.currentApp, envs, opts.Config.DefaultEnv)
		m.screen = screenEnvPicker

	default:
		m.appPicker = NewAppPicker(opts.Config.Apps, opts.Config.DefaultApp)
		m.screen = screenAppPicker
	}

	return m
}

// newBrowseMode initializes the model for browse flow.
func newBrowseMode(m Model, opts Options) Model {
	m.loading = true

	switch {
	case opts.Vault != "" && opts.Item != "":
		m.screen = screenFieldEditor
	case opts.Vault != "":
		m.screen = screenItemList
		m.currentVault = &secrets.Vault{ID: opts.Vault, Name: opts.Vault}
	default:
		m.screen = screenVaultPicker
	}

	return m
}

// Init returns the command to load initial data for the starting screen.
func (m Model) Init() tea.Cmd {
	switch m.screen {
	case screenAppPicker, screenEnvPicker, screenSecretList:
		return nil
	case screenVaultPicker:
		return m.loadVaults()
	case screenItemList:
		return m.loadItems(m.presetVault)
	case screenFieldEditor:
		return m.loadFields(m.presetItem)
	}
	return nil
}

// Update processes messages and delegates to the active screen.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case vaultsLoadedMsg:
		m.loading = false
		m.vaultPicker = NewVaultPicker(msg.vaults)
		return m, nil

	case itemsLoadedMsg:
		m.loading = false
		m.itemList = NewItemList(m.vaultName(), msg.items)
		return m, nil

	case fieldsLoadedMsg:
		m.loading = false
		_, hasTypeEditor := m.editor.(secrets.FieldTypeEditor)
		m.fieldEditor = NewFieldEditor(msg.itemRef, msg.itemName, msg.fields, hasTypeEditor)
		return m, nil

	case saveCompleteMsg:
		if msg.err != nil {
			m.err = msg.err
			m.loading = false
			return m, nil
		}
		// Reload fields after successful save.
		m.loading = true
		ref := m.currentSourceRef
		if ref == "" && m.currentItem != nil {
			ref = m.currentItem.ID
		}
		return m, m.loadFields(ref)

	case editorCreatedMsg:
		m.editor = msg.editor
		if msg.source.Name == "__browse__" {
			m.screen = screenVaultPicker
			m.loading = true
			return m, m.loadVaults()
		}
		// Config mode: load fields for the selected source
		m.currentSourceRef = msg.source.Name
		m.screen = screenFieldEditor
		m.loading = true
		return m, m.loadFields(msg.source.Name)

	case errMsg:
		m.err = msg.err
		m.loading = false
		return m, nil
	}

	if m.loading {
		return m, nil
	}

	switch m.screen {
	case screenAppPicker:
		return m.updateAppPicker(msg)
	case screenEnvPicker:
		return m.updateEnvPicker(msg)
	case screenSecretList:
		return m.updateSecretList(msg)
	case screenVaultPicker:
		return m.updateVaultPicker(msg)
	case screenItemList:
		return m.updateItemList(msg)
	case screenFieldEditor:
		return m.updateFieldEditor(msg)
	}

	return m, nil
}

func (m Model) updateAppPicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.appPicker, _ = m.appPicker.Update(msg)

	if m.appPicker.Quitting() {
		return m, tea.Quit
	}

	if app := m.appPicker.Selected(); app != "" {
		m.currentApp = app
		envs := m.configCtx.Envs[app]
		m.envPicker = NewEnvPicker(app, envs, m.configCtx.DefaultEnv)
		m.screen = screenEnvPicker
	}

	return m, nil
}

func (m Model) updateEnvPicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.envPicker, _ = m.envPicker.Update(msg)

	if m.envPicker.Quitting() {
		return m, tea.Quit
	}

	if m.envPicker.GoBack() {
		m.screen = screenAppPicker
		m.appPicker = NewAppPicker(m.configCtx.Apps, m.configCtx.DefaultApp)
		return m, nil
	}

	if env := m.envPicker.Selected(); env != "" {
		m.currentEnv = env
		key := m.currentApp + "/" + env
		sources := m.configCtx.Sources[key]
		appEnv := m.currentApp + " / " + env
		if m.currentApp == "" {
			appEnv = env
		}
		m.secretList = NewSecretList(appEnv, sources)
		m.screen = screenSecretList
	}

	return m, nil
}

func (m Model) updateSecretList(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.secretList, _ = m.secretList.Update(msg)

	if m.secretList.Quitting() {
		return m, tea.Quit
	}

	if m.secretList.GoBack() {
		m.screen = screenEnvPicker
		envs := m.configCtx.Envs[m.currentApp]
		m.envPicker = NewEnvPicker(m.currentApp, envs, m.configCtx.DefaultEnv)
		return m, nil
	}

	if m.secretList.BrowseMode() {
		m.loading = true
		return m, m.createEditorForBrowse()
	}

	if src := m.secretList.Selected(); src != nil {
		m.loading = true
		return m, m.createEditorForSource(*src)
	}

	return m, nil
}

func (m Model) updateVaultPicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.vaultPicker, cmd = m.vaultPicker.Update(msg)

	if m.vaultPicker.Quitting() {
		return m, tea.Quit
	}

	if v := m.vaultPicker.Selected(); v != nil {
		m.currentVault = v
		m.screen = screenItemList
		m.loading = true
		return m, m.loadItems(v.ID)
	}

	return m, cmd
}

func (m Model) updateItemList(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.itemList, cmd = m.itemList.Update(msg)

	if m.itemList.Quitting() {
		return m, tea.Quit
	}

	if m.itemList.GoBack() {
		m.screen = screenVaultPicker
		m.loading = true
		return m, m.loadVaults()
	}

	if item := m.itemList.Selected(); item != nil {
		m.currentItem = item
		m.screen = screenFieldEditor
		m.loading = true
		return m, m.loadFields(item.ID)
	}

	return m, cmd
}

func (m Model) updateFieldEditor(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.fieldEditor, cmd = m.fieldEditor.Update(msg)

	if m.fieldEditor.Quitting() {
		changes := m.fieldEditor.PendingChanges()
		if len(changes) > 0 {
			return m, m.saveChanges(changes)
		}
		return m, tea.Quit
	}

	if m.fieldEditor.Saving() {
		changes := m.fieldEditor.PendingChanges()
		m.loading = true
		return m, m.saveChanges(changes)
	}

	if m.fieldEditor.GoBack() {
		changes := m.fieldEditor.PendingChanges()
		if len(changes) > 0 {
			m.loading = true
			return m, m.saveChanges(changes)
		}
		// Config mode: return to secret list
		if m.configCtx != nil {
			m.screen = screenSecretList
			key := m.currentApp + "/" + m.currentEnv
			sources := m.configCtx.Sources[key]
			appEnv := m.currentApp + " / " + m.currentEnv
			if m.currentApp == "" {
				appEnv = m.currentEnv
			}
			m.secretList = NewSecretList(appEnv, sources)
			m.currentSourceRef = ""
			return m, nil
		}
		// Browse mode: return to item list
		m.screen = screenItemList
		m.loading = true
		return m, m.loadItems(m.currentVault.ID)
	}

	return m, cmd
}

// View renders the active screen, or a loading/error state.
func (m Model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress q to quit.", m.err)
	}

	if m.loading {
		return "Loading..."
	}

	switch m.screen {
	case screenAppPicker:
		return m.appPicker.View()
	case screenEnvPicker:
		return m.envPicker.View()
	case screenSecretList:
		return m.secretList.View()
	case screenVaultPicker:
		return m.vaultPicker.View()
	case screenItemList:
		return m.itemList.View()
	case screenFieldEditor:
		return m.fieldEditor.View()
	}

	return ""
}

// createEditorForSource returns a command that creates an editor for the given source.
func (m Model) createEditorForSource(src Source) tea.Cmd {
	return func() tea.Msg {
		editor, err := m.editorFactory(context.Background(), src.Backend)
		if err != nil {
			return errMsg{err: err}
		}
		return editorCreatedMsg{editor: editor, source: src}
	}
}

// createEditorForBrowse returns a command that creates an editor for browse mode.
func (m Model) createEditorForBrowse() tea.Cmd {
	return func() tea.Msg {
		backend := "aws"
		if m.configCtx != nil {
			for _, sources := range m.configCtx.Sources {
				if len(sources) > 0 {
					backend = sources[0].Backend
					break
				}
			}
		}
		editor, err := m.editorFactory(context.Background(), backend)
		if err != nil {
			return errMsg{err: err}
		}
		return editorCreatedMsg{editor: editor, source: Source{Name: "__browse__"}}
	}
}

// saveChanges returns a command that applies pending changes via the editor.
func (m Model) saveChanges(changes []PendingChange) tea.Cmd {
	// Determine the secret reference for API calls.
	ref := m.currentSourceRef
	if ref == "" && m.currentItem != nil {
		ref = m.currentItem.ID
	}

	return func() tea.Msg {
		ctx := context.Background()
		for _, c := range changes {
			var err error
			switch c.Type {
			case "update":
				err = m.editor.UpdateField(ctx, ref, c.Field)
			case "delete":
				err = m.editor.DeleteField(ctx, ref, c.Field.Key)
			case "rename":
				err = m.editor.RenameField(ctx, ref, c.OldKey, c.Field.Key)
			case "set_type":
				if fte, ok := m.editor.(secrets.FieldTypeEditor); ok {
					err = fte.SetFieldType(ctx, ref, c.Field.Key, c.NewType)
				}
			}
			if err != nil {
				return saveCompleteMsg{err: err}
			}
		}
		return saveCompleteMsg{}
	}
}

// loadVaults returns a command that fetches vaults from the editor.
func (m Model) loadVaults() tea.Cmd {
	return func() tea.Msg {
		vaults, err := m.editor.ListVaults(context.Background())
		if err != nil {
			return errMsg{err: err}
		}
		return vaultsLoadedMsg{vaults: vaults}
	}
}

// loadItems returns a command that fetches items for a vault.
func (m Model) loadItems(vault string) tea.Cmd {
	return func() tea.Msg {
		items, err := m.editor.ListItems(context.Background(), vault)
		if err != nil {
			return errMsg{err: err}
		}
		return itemsLoadedMsg{items: items}
	}
}

// loadFields returns a command that fetches fields for an item.
func (m Model) loadFields(itemRef string) tea.Cmd {
	return func() tea.Msg {
		fields, err := m.editor.GetFields(context.Background(), itemRef)
		if err != nil {
			return errMsg{err: err}
		}
		return fieldsLoadedMsg{
			fields:   fields,
			itemRef:  itemRef,
			itemName: itemRef, // Use ref as name when we don't have a separate name
		}
	}
}

// vaultName returns the display name for the current vault.
func (m Model) vaultName() string {
	if m.currentVault != nil {
		return m.currentVault.Name
	}
	return ""
}

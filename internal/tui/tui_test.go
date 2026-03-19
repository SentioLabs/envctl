package tui

import (
	"context"
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sentiolabs/envctl/internal/secrets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockEditor implements secrets.Editor and optionally secrets.FieldTypeEditor.
type mockEditor struct {
	vaults   []secrets.Vault
	items    []secrets.Item
	fields   []secrets.Field
	calls    []string // records method names
	hasTypes bool     // if true, also implement FieldTypeEditor via mockFieldTypeEditor
}

func (m *mockEditor) GetSecret(_ context.Context, _ string) (map[string]string, error) {
	m.calls = append(m.calls, "GetSecret")
	return nil, nil
}

func (m *mockEditor) GetSecretKey(_ context.Context, _, _ string) (string, error) {
	m.calls = append(m.calls, "GetSecretKey")
	return "", nil
}

func (m *mockEditor) Name() string { return "mock" }

func (m *mockEditor) ListVaults(_ context.Context) ([]secrets.Vault, error) {
	m.calls = append(m.calls, "ListVaults")
	return m.vaults, nil
}

func (m *mockEditor) ListItems(_ context.Context, _ string) ([]secrets.Item, error) {
	m.calls = append(m.calls, "ListItems")
	return m.items, nil
}

func (m *mockEditor) GetFields(_ context.Context, _ string) ([]secrets.Field, error) {
	m.calls = append(m.calls, "GetFields")
	return m.fields, nil
}

func (m *mockEditor) UpdateField(_ context.Context, _ string, _ secrets.Field) error {
	m.calls = append(m.calls, "UpdateField")
	return nil
}

func (m *mockEditor) DeleteField(_ context.Context, _ string, _ secrets.Field) error {
	m.calls = append(m.calls, "DeleteField")
	return nil
}

func (m *mockEditor) RenameField(_ context.Context, _ string, _ secrets.Field, _ string) error {
	m.calls = append(m.calls, "RenameField")
	return nil
}

func (m *mockEditor) CreateItem(_ context.Context, _, _ string, _ []secrets.Field) error {
	m.calls = append(m.calls, "CreateItem")
	return nil
}

// mockFieldTypeEditor extends mockEditor with FieldTypeEditor support.
type mockFieldTypeEditor struct {
	mockEditor
}

func (m *mockFieldTypeEditor) SetFieldType(_ context.Context, _ string, _ secrets.Field, _ secrets.FieldType) error {
	m.calls = append(m.calls, "SetFieldType")
	return nil
}

func newMockEditor() *mockEditor {
	return &mockEditor{
		vaults: []secrets.Vault{
			{ID: "vault-1", Name: "Development"},
			{ID: "vault-2", Name: "Staging"},
		},
		items: []secrets.Item{
			{ID: "item-1", Name: "db-creds", Vault: "vault-1"},
			{ID: "item-2", Name: "api-keys", Vault: "vault-1"},
		},
		fields: []secrets.Field{
			{ID: "f1", Key: "DB_HOST", Value: "localhost", Type: secrets.FieldText},
			{ID: "f2", Key: "DB_PASS", Value: "secret", Type: secrets.FieldConcealed},
		},
	}
}

func newMockFieldTypeEditor() *mockFieldTypeEditor {
	me := newMockEditor()
	return &mockFieldTypeEditor{mockEditor: *me}
}

// mockEditorFactory tracks EditorFactory calls and returns pre-configured editors.
type mockEditorFactory struct {
	editors map[string]*mockEditor // backend -> editor
	calls   []string
}

func (f *mockEditorFactory) create(_ context.Context, backend string) (secrets.Editor, error) {
	f.calls = append(f.calls, backend)
	if e, ok := f.editors[backend]; ok {
		return e, nil
	}
	return nil, fmt.Errorf("unknown backend: %s", backend)
}

// executeCmd runs a tea.Cmd synchronously and returns the resulting message.
func executeCmd(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	return cmd()
}

// updateModel is a helper that calls Update and asserts the result is a Model.
func updateModel(t *testing.T, m Model, msg tea.Msg) (Model, tea.Cmd) {
	t.Helper()
	updated, cmd := m.Update(msg)
	model, ok := updated.(Model)
	require.True(t, ok, "expected Update to return a Model")
	return model, cmd
}

func TestNew_DefaultStartsAtVaultPicker(t *testing.T) {
	editor := newMockEditor()
	m := New(Options{Editor: editor})

	assert.Equal(t, screenVaultPicker, m.screen)
	assert.True(t, m.loading)
}

func TestNew_WithVaultSkipsToItemList(t *testing.T) {
	editor := newMockEditor()
	m := New(Options{Editor: editor, Vault: "vault-1"})

	assert.Equal(t, screenItemList, m.screen)
	assert.True(t, m.loading)
}

func TestNew_WithVaultAndItemSkipsToFieldEditor(t *testing.T) {
	editor := newMockEditor()
	m := New(Options{Editor: editor, Vault: "vault-1", Item: "item-1"})

	assert.Equal(t, screenFieldEditor, m.screen)
	assert.True(t, m.loading)
}

func TestInit_VaultPicker_LoadsVaults(t *testing.T) {
	editor := newMockEditor()
	m := New(Options{Editor: editor})

	cmd := m.Init()
	require.NotNil(t, cmd)

	msg := executeCmd(cmd)
	loaded, ok := msg.(vaultsLoadedMsg)
	require.True(t, ok)
	assert.Len(t, loaded.vaults, 2)
	assert.Contains(t, editor.calls, "ListVaults")
}

func TestInit_ItemList_LoadsItems(t *testing.T) {
	editor := newMockEditor()
	m := New(Options{Editor: editor, Vault: "vault-1"})

	cmd := m.Init()
	require.NotNil(t, cmd)

	msg := executeCmd(cmd)
	loaded, ok := msg.(itemsLoadedMsg)
	require.True(t, ok)
	assert.Len(t, loaded.items, 2)
	assert.Contains(t, editor.calls, "ListItems")
}

func TestInit_FieldEditor_LoadsFields(t *testing.T) {
	editor := newMockEditor()
	m := New(Options{Editor: editor, Vault: "vault-1", Item: "item-1"})

	cmd := m.Init()
	require.NotNil(t, cmd)

	msg := executeCmd(cmd)
	loaded, ok := msg.(fieldsLoadedMsg)
	require.True(t, ok)
	assert.Len(t, loaded.fields, 2)
	assert.Contains(t, editor.calls, "GetFields")
}

func TestUpdate_VaultsLoaded_TransitionsToVaultPicker(t *testing.T) {
	editor := newMockEditor()
	m := New(Options{Editor: editor})

	model, _ := updateModel(t, m, vaultsLoadedMsg{vaults: editor.vaults})

	assert.False(t, model.loading)
	assert.Equal(t, screenVaultPicker, model.screen)
}

func TestUpdate_VaultSelected_TransitionsToItemList(t *testing.T) {
	editor := newMockEditor()
	m := New(Options{Editor: editor})

	// Load vaults first
	m, _ = updateModel(t, m, vaultsLoadedMsg{vaults: editor.vaults})

	// Simulate selecting a vault by pressing Enter on the vault picker
	model, cmd := updateModel(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	assert.Equal(t, screenItemList, model.screen)
	assert.True(t, model.loading)
	assert.NotNil(t, model.currentVault)
	assert.Equal(t, "Development", model.currentVault.Name)

	// The command should load items
	require.NotNil(t, cmd)
	msg := executeCmd(cmd)
	_, ok := msg.(itemsLoadedMsg)
	assert.True(t, ok)
}

func TestUpdate_ItemsLoaded_StopsLoading(t *testing.T) {
	editor := newMockEditor()
	m := New(Options{Editor: editor, Vault: "vault-1"})

	model, _ := updateModel(t, m, itemsLoadedMsg{items: editor.items})

	assert.False(t, model.loading)
	assert.Equal(t, screenItemList, model.screen)
}

func TestUpdate_ItemSelected_TransitionsToFieldEditor(t *testing.T) {
	editor := newMockEditor()
	m := New(Options{Editor: editor, Vault: "vault-1"})

	// Load items first
	m, _ = updateModel(t, m, itemsLoadedMsg{items: editor.items})

	// Press Enter to select item
	model, cmd := updateModel(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	assert.Equal(t, screenFieldEditor, model.screen)
	assert.True(t, model.loading)
	assert.NotNil(t, model.currentItem)

	// The command should load fields
	require.NotNil(t, cmd)
	msg := executeCmd(cmd)
	_, ok := msg.(fieldsLoadedMsg)
	assert.True(t, ok)
}

func TestUpdate_BackFromItemList_ReturnsToVaultPicker(t *testing.T) {
	editor := newMockEditor()
	m := New(Options{Editor: editor})

	// Load vaults
	m, _ = updateModel(t, m, vaultsLoadedMsg{vaults: editor.vaults})
	// Select vault -> item list
	m, _ = updateModel(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	// Load items
	m, _ = updateModel(t, m, itemsLoadedMsg{items: editor.items})

	// Press Esc to go back
	model, cmd := updateModel(t, m, tea.KeyMsg{Type: tea.KeyEscape})

	assert.Equal(t, screenVaultPicker, model.screen)
	assert.True(t, model.loading)
	// Should reload vaults
	require.NotNil(t, cmd)
	msg := executeCmd(cmd)
	_, ok := msg.(vaultsLoadedMsg)
	assert.True(t, ok)
}

func TestUpdate_BackFromFieldEditor_ReturnsToItemList(t *testing.T) {
	editor := newMockEditor()
	m := New(Options{Editor: editor, Vault: "vault-1"})

	// Load items
	m, _ = updateModel(t, m, itemsLoadedMsg{items: editor.items})
	// Select item -> field editor
	m, _ = updateModel(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	// Load fields
	m, _ = updateModel(t, m, fieldsLoadedMsg{
		fields:   editor.fields,
		itemRef:  "item-1",
		itemName: "db-creds",
	})

	// Press Esc to go back
	model, cmd := updateModel(t, m, tea.KeyMsg{Type: tea.KeyEscape})

	assert.Equal(t, screenItemList, model.screen)
	assert.True(t, model.loading)
	// Should reload items
	require.NotNil(t, cmd)
	msg := executeCmd(cmd)
	_, ok := msg.(itemsLoadedMsg)
	assert.True(t, ok)
}

func TestUpdate_SaveChanges_CallsEditorMethods(t *testing.T) {
	editor := newMockEditor()
	m := New(Options{Editor: editor, Vault: "vault-1", Item: "item-1"})
	m.currentItem = &secrets.Item{ID: "item-1", Name: "db-creds", Vault: "vault-1"}

	changes := []PendingChange{
		{Type: "update", Field: secrets.Field{Key: "DB_HOST", Value: "newhost"}},
		{Type: "delete", Field: secrets.Field{Key: "OLD_KEY"}},
		{Type: "rename", Field: secrets.Field{Key: "NEW_KEY"}, OldKey: "OLD_KEY"},
	}

	cmd := m.saveChanges(changes)
	msg := executeCmd(cmd)

	result, ok := msg.(saveCompleteMsg)
	require.True(t, ok)
	assert.Nil(t, result.err)

	assert.Contains(t, editor.calls, "UpdateField")
	assert.Contains(t, editor.calls, "DeleteField")
	assert.Contains(t, editor.calls, "RenameField")
}

func TestUpdate_SaveChanges_SetTypeCallsFieldTypeEditor(t *testing.T) {
	editor := newMockFieldTypeEditor()
	m := New(Options{Editor: editor, Vault: "vault-1", Item: "item-1"})
	m.currentItem = &secrets.Item{ID: "item-1", Name: "db-creds", Vault: "vault-1"}

	changes := []PendingChange{
		{Type: "set_type", Field: secrets.Field{Key: "DB_HOST"}, NewType: secrets.FieldConcealed},
	}

	cmd := m.saveChanges(changes)
	msg := executeCmd(cmd)

	result, ok := msg.(saveCompleteMsg)
	require.True(t, ok)
	assert.Nil(t, result.err)
	assert.Contains(t, editor.calls, "SetFieldType")
}

func TestUpdate_SetType_NoFieldTypeEditor_Skips(t *testing.T) {
	editor := newMockEditor() // does NOT implement FieldTypeEditor
	m := New(Options{Editor: editor, Vault: "vault-1", Item: "item-1"})
	m.currentItem = &secrets.Item{ID: "item-1", Name: "db-creds", Vault: "vault-1"}

	changes := []PendingChange{
		{Type: "set_type", Field: secrets.Field{Key: "DB_HOST"}, NewType: secrets.FieldConcealed},
	}

	cmd := m.saveChanges(changes)
	msg := executeCmd(cmd)

	result, ok := msg.(saveCompleteMsg)
	require.True(t, ok)
	assert.Nil(t, result.err)
	assert.NotContains(t, editor.calls, "SetFieldType")
}

func TestUpdate_Quit_ReturnsQuitCmd(t *testing.T) {
	editor := newMockEditor()
	m := New(Options{Editor: editor})

	// Load vaults
	m, _ = updateModel(t, m, vaultsLoadedMsg{vaults: editor.vaults})

	// Press q to quit
	_, cmd := updateModel(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

	// The quit should be detected
	assert.NotNil(t, cmd)
}

func TestUpdate_WindowSizeMsg(t *testing.T) {
	editor := newMockEditor()
	m := New(Options{Editor: editor})

	model, _ := updateModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})

	assert.Equal(t, 120, model.width)
	assert.Equal(t, 40, model.height)
}

func TestUpdate_ErrorMsg(t *testing.T) {
	editor := newMockEditor()
	m := New(Options{Editor: editor})

	testErr := assert.AnError
	model, _ := updateModel(t, m, errMsg{err: testErr})

	assert.Equal(t, testErr, model.err)
	assert.False(t, model.loading)
}

func TestView_Loading(t *testing.T) {
	editor := newMockEditor()
	m := New(Options{Editor: editor})

	view := m.View()
	assert.Contains(t, view, "Loading")
}

func TestView_Error(t *testing.T) {
	editor := newMockEditor()
	m := New(Options{Editor: editor})
	m.err = assert.AnError
	m.loading = false

	view := m.View()
	assert.Contains(t, view, "Error")
}

func TestView_DelegatesToActiveScreen(t *testing.T) {
	editor := newMockEditor()
	m := New(Options{Editor: editor})

	// Load vaults to stop loading
	m, _ = updateModel(t, m, vaultsLoadedMsg{vaults: editor.vaults})

	view := m.View()
	// Vault picker should show vault names
	assert.Contains(t, view, "Development")
}

func TestSaveComplete_ReloadsFields(t *testing.T) {
	editor := newMockEditor()
	m := New(Options{Editor: editor, Vault: "vault-1", Item: "item-1"})
	m.currentItem = &secrets.Item{ID: "item-1", Name: "db-creds", Vault: "vault-1"}
	m.loading = false
	m.screen = screenFieldEditor
	m.fieldEditor = NewFieldEditor("item-1", "db-creds", editor.fields, false)

	model, cmd := updateModel(t, m, saveCompleteMsg{})

	assert.True(t, model.loading)
	require.NotNil(t, cmd)
	msg := executeCmd(cmd)
	_, ok := msg.(fieldsLoadedMsg)
	assert.True(t, ok)
}

// --- Config mode tests ---

func newTestConfigContext() *ConfigContext {
	return &ConfigContext{
		Apps: []string{"core-api", "web-app"},
		Envs: map[string][]string{
			"core-api": {"local", "staging"},
			"web-app":  {"local", "production"},
		},
		Sources: map[string][]Source{
			"core-api/local":      {{Name: "core-api-local-secrets", Backend: "1pass"}},
			"core-api/staging":    {{Name: "core-api-staging-secrets", Backend: "aws"}},
			"web-app/local":       {{Name: "web-app-local-secrets", Backend: "1pass"}},
			"web-app/production":  {{Name: "web-app-prod-secrets", Backend: "aws"}},
		},
		DefaultApp: "",
		DefaultEnv: "",
	}
}

func newTestEditorFactory() *mockEditorFactory {
	return &mockEditorFactory{
		editors: map[string]*mockEditor{
			"1pass": newMockEditor(),
			"aws":   newMockEditor(),
		},
	}
}

func TestNew_ConfigMode_StartsAtAppPicker(t *testing.T) {
	cfg := newTestConfigContext()
	factory := newTestEditorFactory()
	m := New(Options{
		Config:        cfg,
		EditorFactory: factory.create,
	})

	assert.Equal(t, screenAppPicker, m.screen)
	assert.False(t, m.loading, "app picker does not need async loading")
	assert.NotNil(t, m.configCtx)
}

func TestNew_ConfigMode_SingleApp_AutoSkipsToEnvPicker(t *testing.T) {
	cfg := &ConfigContext{
		Apps: []string{"only-app"},
		Envs: map[string][]string{
			"only-app": {"local", "staging"},
		},
		Sources: map[string][]Source{
			"only-app/local":   {{Name: "secrets", Backend: "aws"}},
			"only-app/staging": {{Name: "secrets", Backend: "aws"}},
		},
	}
	factory := newTestEditorFactory()
	m := New(Options{
		Config:        cfg,
		EditorFactory: factory.create,
	})

	assert.Equal(t, screenEnvPicker, m.screen)
	assert.Equal(t, "only-app", m.currentApp)
	assert.False(t, m.loading)
}

func TestNew_ConfigMode_AppFlag_SkipsAppPicker(t *testing.T) {
	cfg := newTestConfigContext()
	factory := newTestEditorFactory()
	m := New(Options{
		Config:        cfg,
		EditorFactory: factory.create,
		App:           "core-api",
	})

	assert.Equal(t, screenEnvPicker, m.screen)
	assert.Equal(t, "core-api", m.currentApp)
	assert.False(t, m.loading)
}

func TestNew_ConfigMode_AppAndEnvFlags_SkipToSecretList(t *testing.T) {
	cfg := newTestConfigContext()
	factory := newTestEditorFactory()
	m := New(Options{
		Config:        cfg,
		EditorFactory: factory.create,
		App:           "core-api",
		Env:           "local",
	})

	assert.Equal(t, screenSecretList, m.screen)
	assert.Equal(t, "core-api", m.currentApp)
	assert.Equal(t, "local", m.currentEnv)
	assert.False(t, m.loading)
}

func TestConfigMode_SelectApp_TransitionsToEnvPicker(t *testing.T) {
	cfg := newTestConfigContext()
	factory := newTestEditorFactory()
	m := New(Options{
		Config:        cfg,
		EditorFactory: factory.create,
	})

	// Simulate pressing Enter to select the first app
	model, _ := updateModel(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	assert.Equal(t, screenEnvPicker, model.screen)
	assert.NotEmpty(t, model.currentApp)
}

func TestConfigMode_SelectEnv_TransitionsToSecretList(t *testing.T) {
	cfg := newTestConfigContext()
	factory := newTestEditorFactory()
	m := New(Options{
		Config:        cfg,
		EditorFactory: factory.create,
		App:           "core-api",
	})

	// At env picker, press Enter to select first env
	model, _ := updateModel(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	assert.Equal(t, screenSecretList, model.screen)
	assert.NotEmpty(t, model.currentEnv)
}

func TestConfigMode_SelectSource_CallsEditorFactory(t *testing.T) {
	cfg := newTestConfigContext()
	factory := newTestEditorFactory()
	m := New(Options{
		Config:        cfg,
		EditorFactory: factory.create,
		App:           "core-api",
		Env:           "local",
	})

	// At secret list, press Enter to select the source
	model, cmd := updateModel(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	assert.True(t, model.loading)
	require.NotNil(t, cmd)

	// Execute the command — should call EditorFactory
	msg := executeCmd(cmd)
	created, ok := msg.(editorCreatedMsg)
	require.True(t, ok)
	assert.NotNil(t, created.editor)
	assert.Equal(t, "core-api-local-secrets", created.source.Name)
	assert.Contains(t, factory.calls, "1pass")

	// Feed the editorCreatedMsg back to transition to field editor
	model, cmd = updateModel(t, model, created)
	assert.Equal(t, screenFieldEditor, model.screen)
	assert.True(t, model.loading) // loading fields now
	require.NotNil(t, cmd)

	// The cmd should load fields
	msg = executeCmd(cmd)
	_, ok = msg.(fieldsLoadedMsg)
	assert.True(t, ok)
}

func TestConfigMode_BrowseMode_SwitchesToVaultPicker(t *testing.T) {
	cfg := newTestConfigContext()
	factory := newTestEditorFactory()
	m := New(Options{
		Config:        cfg,
		EditorFactory: factory.create,
		App:           "core-api",
		Env:           "local",
	})

	// At secret list, press 'b' for browse mode
	model, cmd := updateModel(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})

	assert.True(t, model.loading)
	require.NotNil(t, cmd)

	// Execute the command — should call EditorFactory for browse
	msg := executeCmd(cmd)
	created, ok := msg.(editorCreatedMsg)
	require.True(t, ok)
	assert.Equal(t, "__browse__", created.source.Name)

	// Feed the editorCreatedMsg — should transition to vault picker
	model, cmd = updateModel(t, model, created)
	assert.Equal(t, screenVaultPicker, model.screen)
	assert.True(t, model.loading) // loading vaults now
	require.NotNil(t, cmd)

	// The cmd should load vaults
	msg = executeCmd(cmd)
	_, ok = msg.(vaultsLoadedMsg)
	assert.True(t, ok)
}

func TestBrowseMode_WithoutConfigContext_UsesExistingFlow(t *testing.T) {
	editor := newMockEditor()
	m := New(Options{Editor: editor})

	// Should start at vault picker in browse mode
	assert.Equal(t, screenVaultPicker, m.screen)
	assert.Nil(t, m.configCtx)

	// Init should load vaults
	cmd := m.Init()
	require.NotNil(t, cmd)
	msg := executeCmd(cmd)
	_, ok := msg.(vaultsLoadedMsg)
	assert.True(t, ok)
}

func TestInit_ConfigMode_AppPicker_ReturnsNil(t *testing.T) {
	cfg := newTestConfigContext()
	factory := newTestEditorFactory()
	m := New(Options{
		Config:        cfg,
		EditorFactory: factory.create,
	})

	cmd := m.Init()
	assert.Nil(t, cmd, "app picker needs no async init")
}

func TestInit_ConfigMode_EnvPicker_ReturnsNil(t *testing.T) {
	cfg := newTestConfigContext()
	factory := newTestEditorFactory()
	m := New(Options{
		Config:        cfg,
		EditorFactory: factory.create,
		App:           "core-api",
	})

	cmd := m.Init()
	assert.Nil(t, cmd, "env picker needs no async init")
}

func TestInit_ConfigMode_SecretList_ReturnsNil(t *testing.T) {
	cfg := newTestConfigContext()
	factory := newTestEditorFactory()
	m := New(Options{
		Config:        cfg,
		EditorFactory: factory.create,
		App:           "core-api",
		Env:           "local",
	})

	cmd := m.Init()
	assert.Nil(t, cmd, "secret list needs no async init")
}

func TestView_ConfigMode_AppPicker(t *testing.T) {
	cfg := newTestConfigContext()
	factory := newTestEditorFactory()
	m := New(Options{
		Config:        cfg,
		EditorFactory: factory.create,
	})

	view := m.View()
	assert.Contains(t, view, "application")
}

func TestView_ConfigMode_EnvPicker(t *testing.T) {
	cfg := newTestConfigContext()
	factory := newTestEditorFactory()
	m := New(Options{
		Config:        cfg,
		EditorFactory: factory.create,
		App:           "core-api",
	})

	view := m.View()
	assert.Contains(t, view, "environment")
}

func TestView_ConfigMode_SecretList(t *testing.T) {
	cfg := newTestConfigContext()
	factory := newTestEditorFactory()
	m := New(Options{
		Config:        cfg,
		EditorFactory: factory.create,
		App:           "core-api",
		Env:           "local",
	})

	view := m.View()
	assert.Contains(t, view, "Secrets")
}

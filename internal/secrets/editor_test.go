package secrets_test

import (
	"context"
	"testing"

	"github.com/sentiolabs/envctl/internal/config"
	"github.com/sentiolabs/envctl/internal/secrets"
)

// mockEditor implements secrets.Editor for compile-time interface checks.
type mockEditor struct{}

func (m *mockEditor) GetSecret(_ context.Context, _ string) (map[string]string, error) {
	return nil, nil
}
func (m *mockEditor) GetSecretKey(_ context.Context, _, _ string) (string, error) { return "", nil }
func (m *mockEditor) Name() string                                                { return "mock" }
func (m *mockEditor) ListVaults(_ context.Context) ([]secrets.Vault, error)       { return nil, nil }
func (m *mockEditor) ListItems(_ context.Context, _ string) ([]secrets.Item, error) {
	return nil, nil
}

func (m *mockEditor) GetFields(_ context.Context, _ string) ([]secrets.Field, error) {
	return nil, nil
}
func (m *mockEditor) UpdateField(_ context.Context, _ string, _ secrets.Field) error { return nil }
func (m *mockEditor) DeleteField(_ context.Context, _, _ string) error               { return nil }
func (m *mockEditor) RenameField(_ context.Context, _, _, _ string) error            { return nil }
func (m *mockEditor) CreateItem(_ context.Context, _ string, _ string, _ []secrets.Field) error {
	return nil
}

// mockFieldTypeEditor extends mockEditor with FieldTypeEditor support.
type mockFieldTypeEditor struct {
	mockEditor
}

func (m *mockFieldTypeEditor) SetFieldType(_ context.Context, _, _ string, _ secrets.FieldType) error {
	return nil
}

func TestEditorImplementsClient(t *testing.T) {
	// Compile-time check: Editor must embed Client.
	var e secrets.Editor = &mockEditor{}
	var _ secrets.Client = e
}

func TestFieldTypeEditorAssertion(t *testing.T) {
	// A FieldTypeEditor can be type-asserted from an Editor.
	var e secrets.Editor = &mockFieldTypeEditor{}

	fte, ok := e.(secrets.FieldTypeEditor)
	if !ok {
		t.Fatal("expected mockFieldTypeEditor to satisfy FieldTypeEditor interface")
	}
	if fte == nil {
		t.Fatal("expected non-nil FieldTypeEditor")
	}
}

func TestFieldTypeConstants(t *testing.T) {
	if secrets.FieldText != "text" {
		t.Errorf("expected FieldText to be %q, got %q", "text", secrets.FieldText)
	}
	if secrets.FieldConcealed != "concealed" {
		t.Errorf("expected FieldConcealed to be %q, got %q", "concealed", secrets.FieldConcealed)
	}
}

func TestNewEditorReturnsNotImplemented(t *testing.T) {
	t.Run("default backend returns not implemented", func(t *testing.T) {
		opts := secrets.EditorOptions{
			Config: nil,
			Env:    nil,
		}
		_, err := secrets.NewEditor(t.Context(), opts)
		if err == nil {
			t.Fatal("expected error for unimplemented AWS editor")
		}
	})

	t.Run("1password backend creates editor", func(t *testing.T) {
		cfg := &config.Config{
			Version: 1,
			OnePass: &config.OnePassConfig{Vault: "TestVault"},
		}
		opts := secrets.EditorOptions{
			Config: cfg,
			Env:    nil,
		}
		editor, err := secrets.NewEditor(t.Context(), opts)
		if err != nil {
			t.Skipf("skipping: op CLI not available: %v", err)
		}
		if editor == nil {
			t.Fatal("expected non-nil editor for 1password backend")
		}

		// Verify the editor also satisfies FieldTypeEditor.
		if _, ok := editor.(secrets.FieldTypeEditor); !ok {
			t.Fatal("expected 1password editor to satisfy FieldTypeEditor")
		}
	})
}

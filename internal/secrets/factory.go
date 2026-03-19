package secrets

import (
	"context"
	"fmt"

	"github.com/sentiolabs/envctl/internal/aws"
	"github.com/sentiolabs/envctl/internal/cache"
	"github.com/sentiolabs/envctl/internal/config"
	"github.com/sentiolabs/envctl/internal/onepassword"
)

// Compile-time interface checks for the adapters.
var (
	_ Editor          = (*opEditorAdapter)(nil)
	_ FieldTypeEditor = (*opEditorAdapter)(nil)
	_ BatchSaver      = (*opEditorAdapter)(nil)
	_ Editor          = (*awsEditorAdapter)(nil)
)

// Options configures the secrets client factory.
type Options struct {
	Config  *config.Config
	Env     *config.Environment
	Cache   *cache.Manager
	NoCache bool
	Refresh bool
}

// NewClient creates a secrets client based on the resolved environment's backend.
// Uses config.ResolveBackend to determine which backend to use, with precedence:
// environment block > global block > default (aws).
func NewClient(ctx context.Context, opts Options) (Client, error) {
	backend := config.BackendAWS
	if opts.Config != nil {
		backend = opts.Config.ResolveBackend(opts.Env)
	}

	switch backend {
	case config.Backend1Pass:
		opCfg := config.OnePassConfig{}
		if opts.Config != nil {
			opCfg = opts.Config.ResolveOnePassConfig(opts.Env)
		}
		return newOnePasswordClient(opCfg, opts)
	default:
		awsCfg := config.AWSConfig{}
		if opts.Config != nil {
			awsCfg = opts.Config.ResolveAWSConfig(opts.Env)
		}
		return newAWSClient(ctx, awsCfg, opts)
	}
}

// newAWSClient creates an AWS Secrets Manager client using resolved AWS config.
func newAWSClient(ctx context.Context, awsCfg config.AWSConfig, opts Options) (Client, error) {
	return aws.NewSecretsClientWithOptions(ctx, aws.ClientOptions{
		Region:  awsCfg.Region,
		Profile: awsCfg.Profile,
		Cache:   opts.Cache,
		NoCache: opts.NoCache,
		Refresh: opts.Refresh,
	})
}

// newOnePasswordClient creates a 1Password client using resolved OnePass config.
func newOnePasswordClient(opCfg config.OnePassConfig, opts Options) (Client, error) {
	return onepassword.NewClient(onepassword.ClientOptions{
		DefaultVault: opCfg.Vault,
		Account:      opCfg.Account,
	})
}

// EditorOptions configures the editor factory.
type EditorOptions struct {
	Config *config.Config
	Env    *config.Environment
}

// NewEditor creates a secrets editor based on the resolved environment's backend.
func NewEditor(ctx context.Context, opts EditorOptions) (Editor, error) {
	backend := config.BackendAWS
	if opts.Config != nil {
		backend = opts.Config.ResolveBackend(opts.Env)
	}

	switch backend {
	case config.Backend1Pass:
		opCfg := config.OnePassConfig{}
		if opts.Config != nil {
			opCfg = opts.Config.ResolveOnePassConfig(opts.Env)
		}
		return newOnePasswordEditor(opCfg)
	default:
		awsCfg := config.AWSConfig{}
		if opts.Config != nil {
			awsCfg = opts.Config.ResolveAWSConfig(opts.Env)
		}
		return newAWSEditor(ctx, awsCfg)
	}
}

// NewEditorForBackend creates an editor for a specific backend.
// Used by the TUI's EditorFactory to create editors on demand for config-driven mode.
func NewEditorForBackend(ctx context.Context, cfg *config.Config, backend string) (Editor, error) {
	switch backend {
	case config.Backend1Pass:
		opCfg := config.OnePassConfig{}
		if cfg != nil {
			opCfg = cfg.ResolveOnePassConfig(nil)
		}
		return newOnePasswordEditor(opCfg)
	default:
		awsCfg := config.AWSConfig{}
		if cfg != nil {
			awsCfg = cfg.ResolveAWSConfig(nil)
		}
		return newAWSEditor(ctx, awsCfg)
	}
}

// newAWSEditor creates an AWS editor that adapts the aws.AWSEditor
// to the secrets.Editor interface.
func newAWSEditor(ctx context.Context, awsCfg config.AWSConfig) (Editor, error) {
	awsEditor, err := aws.NewEditor(ctx, aws.EditorOptions{
		Region:  awsCfg.Region,
		Profile: awsCfg.Profile,
	})
	if err != nil {
		return nil, err
	}
	return &awsEditorAdapter{awsEditor: awsEditor}, nil
}

// awsEditorAdapter wraps aws.AWSEditor to satisfy secrets.Editor.
// This adapter is necessary to avoid an import cycle between the secrets and aws packages.
type awsEditorAdapter struct {
	awsEditor *aws.AWSEditor
}

// GetSecret delegates to the underlying AWS editor.
func (a *awsEditorAdapter) GetSecret(ctx context.Context, secretRef string) (map[string]string, error) {
	return a.awsEditor.GetSecret(ctx, secretRef)
}

// GetSecretKey delegates to the underlying AWS editor.
func (a *awsEditorAdapter) GetSecretKey(ctx context.Context, secretRef, key string) (string, error) {
	return a.awsEditor.GetSecretKey(ctx, secretRef, key)
}

// Name returns the backend name from the underlying AWS editor.
func (a *awsEditorAdapter) Name() string {
	return a.awsEditor.Name()
}

// ListVaults converts aws.EditorVault to secrets.Vault.
func (a *awsEditorAdapter) ListVaults(ctx context.Context) ([]Vault, error) {
	vaults, err := a.awsEditor.ListEditorVaults(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]Vault, len(vaults))
	for i, v := range vaults {
		result[i] = Vault{ID: v.ID, Name: v.Name}
	}
	return result, nil
}

// ListItems converts aws.EditorItem to secrets.Item.
func (a *awsEditorAdapter) ListItems(ctx context.Context, vault string) ([]Item, error) {
	items, err := a.awsEditor.ListEditorItems(ctx, vault)
	if err != nil {
		return nil, err
	}
	result := make([]Item, len(items))
	for i, item := range items {
		result[i] = Item{ID: item.ID, Name: item.Name, Vault: item.Vault}
	}
	return result, nil
}

// GetFields converts aws.EditorField to secrets.Field.
func (a *awsEditorAdapter) GetFields(ctx context.Context, ref string) ([]Field, error) {
	fields, err := a.awsEditor.GetEditorFields(ctx, ref)
	if err != nil {
		return nil, err
	}
	result := make([]Field, len(fields))
	for i, f := range fields {
		result[i] = Field{
			ID:      f.ID,
			Key:     f.Key,
			Value:   f.Value,
			Type:    FieldType(f.Type),
			Section: f.Section,
		}
	}
	return result, nil
}

// UpdateField delegates field update to the AWS editor.
func (a *awsEditorAdapter) UpdateField(ctx context.Context, ref string, field Field) error {
	return a.awsEditor.UpdateEditorField(ctx, ref, field.Key, field.Value)
}

// DeleteField delegates field deletion to the AWS editor.
func (a *awsEditorAdapter) DeleteField(ctx context.Context, ref string, field Field) error {
	return a.awsEditor.DeleteEditorField(ctx, ref, field.Key)
}

// RenameField delegates field rename to the AWS editor.
func (a *awsEditorAdapter) RenameField(ctx context.Context, ref string, field Field, newKey string) error {
	return a.awsEditor.RenameEditorField(ctx, ref, field.Key, newKey)
}

// CreateItem converts secrets.Field to aws.EditorFieldPair and delegates item creation.
func (a *awsEditorAdapter) CreateItem(ctx context.Context, vault, name string, fields []Field) error {
	awsFields := make([]aws.EditorFieldPair, len(fields))
	for i, f := range fields {
		awsFields[i] = aws.EditorFieldPair{Key: f.Key, Value: f.Value}
	}
	return a.awsEditor.CreateEditorItem(ctx, vault, name, awsFields)
}

// newOnePasswordEditor creates a 1Password editor that adapts the onepassword.OPEditor
// to the secrets.Editor and secrets.FieldTypeEditor interfaces.
func newOnePasswordEditor(opCfg config.OnePassConfig) (Editor, error) {
	opEditor, err := onepassword.NewEditor(onepassword.ClientOptions{
		DefaultVault: opCfg.Vault,
		Account:      opCfg.Account,
	})
	if err != nil {
		return nil, err
	}
	return &opEditorAdapter{opEditor: opEditor}, nil
}

// opEditorAdapter wraps onepassword.OPEditor to satisfy secrets.Editor and secrets.FieldTypeEditor.
// This adapter is necessary to avoid an import cycle between the secrets and onepassword packages.
type opEditorAdapter struct {
	opEditor *onepassword.OPEditor
}

// GetSecret delegates to the underlying 1Password client.
func (a *opEditorAdapter) GetSecret(ctx context.Context, secretRef string) (map[string]string, error) {
	return a.opEditor.GetSecret(ctx, secretRef)
}

// GetSecretKey delegates to the underlying 1Password client.
func (a *opEditorAdapter) GetSecretKey(ctx context.Context, secretRef, key string) (string, error) {
	return a.opEditor.GetSecretKey(ctx, secretRef, key)
}

// Name returns the backend name from the underlying 1Password client.
func (a *opEditorAdapter) Name() string {
	return a.opEditor.Name()
}

// ListVaults converts onepassword.EditorVault to secrets.Vault.
func (a *opEditorAdapter) ListVaults(ctx context.Context) ([]Vault, error) {
	vaults, err := a.opEditor.ListEditorVaults(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]Vault, len(vaults))
	for i, v := range vaults {
		result[i] = Vault{ID: v.ID, Name: v.Name}
	}
	return result, nil
}

// ListItems converts onepassword.EditorItem to secrets.Item.
func (a *opEditorAdapter) ListItems(ctx context.Context, vault string) ([]Item, error) {
	items, err := a.opEditor.ListEditorItems(ctx, vault)
	if err != nil {
		return nil, err
	}
	result := make([]Item, len(items))
	for i, item := range items {
		result[i] = Item{ID: item.ID, Name: item.Name, Vault: item.Vault}
	}
	return result, nil
}

// GetFields converts onepassword.EditorField to secrets.Field with type mapping.
func (a *opEditorAdapter) GetFields(ctx context.Context, ref string) ([]Field, error) {
	fields, err := a.opEditor.GetEditorFields(ctx, ref)
	if err != nil {
		return nil, err
	}
	result := make([]Field, len(fields))
	for i, f := range fields {
		result[i] = Field{
			ID:      f.ID,
			Key:     f.Key,
			Value:   f.Value,
			Type:    FieldType(f.Type),
			Section: f.Section,
		}
	}
	return result, nil
}

// UpdateField delegates field update to the 1Password editor.
func (a *opEditorAdapter) UpdateField(ctx context.Context, ref string, field Field) error {
	return a.opEditor.UpdateField(ctx, ref, field.Key, field.Value, field.Section)
}

// DeleteField delegates field deletion to the 1Password editor.
func (a *opEditorAdapter) DeleteField(ctx context.Context, ref string, field Field) error {
	return a.opEditor.DeleteField(ctx, ref, field.Key, field.Section)
}

// RenameField delegates field rename to the 1Password editor.
func (a *opEditorAdapter) RenameField(ctx context.Context, ref string, field Field, newKey string) error {
	return a.opEditor.RenameField(ctx, ref, field.Key, newKey, field.Section)
}

// CreateItem converts secrets.Field to onepassword.FieldPair and delegates item creation.
func (a *opEditorAdapter) CreateItem(ctx context.Context, vault, name string, fields []Field) error {
	opFields := make([]onepassword.FieldPair, len(fields))
	for i, f := range fields {
		opFields[i] = onepassword.FieldPair{Key: f.Key, Value: f.Value}
	}
	return a.opEditor.CreateEditorItem(ctx, vault, name, opFields)
}

// SetFieldType delegates field type change to the 1Password editor.
func (a *opEditorAdapter) SetFieldType(ctx context.Context, ref string, field Field, ft FieldType) error {
	return a.opEditor.SetEditorFieldType(ctx, ref, field.Key, field.Section, onepassword.FieldType(ft))
}

// BatchSave batches all changes into minimal op item edit calls.
// Regular changes (update, delete, rename) are combined into one call.
// Type changes use different syntax and go in a second call.
func (a *opEditorAdapter) BatchSave(ctx context.Context, ref string, changes []Change) error {
	var assignments []string
	var typeAssignments []string

	for _, c := range changes {
		fieldRef := c.Field.Key
		if c.Field.Section != "" {
			fieldRef = c.Field.Section + "." + c.Field.Key
		}

		switch c.Type {
		case "update":
			assignments = append(assignments, fmt.Sprintf("%s=%s", fieldRef, c.Field.Value))
		case "delete":
			assignments = append(assignments, fieldRef+"[delete]")
		case "rename":
			oldRef := c.OldKey
			if c.Field.Section != "" {
				oldRef = c.Field.Section + "." + c.OldKey
			}
			// Rename = delete old + create new (in same batch)
			assignments = append(assignments, oldRef+"[delete]")
			assignments = append(assignments, fmt.Sprintf("%s=%s", fieldRef, c.Field.Value))
		case "set_type":
			opType := "text"
			if c.NewType == FieldConcealed {
				opType = "password"
			}
			typeAssignments = append(typeAssignments, fmt.Sprintf("%s[%s]=%s", fieldRef, opType, c.Field.Value))
		}
	}

	// First batch: regular changes
	if err := a.opEditor.BatchEdit(ctx, ref, assignments); err != nil {
		return err
	}

	// Second batch: type changes (different syntax)
	return a.opEditor.BatchEdit(ctx, ref, typeAssignments)
}

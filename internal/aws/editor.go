package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

// smClient defines the AWS Secrets Manager operations used by AWSEditor.
// This interface enables dependency injection for testing.
type smClient interface {
	ListSecrets(
		ctx context.Context,
		params *secretsmanager.ListSecretsInput,
		optFns ...func(*secretsmanager.Options),
	) (*secretsmanager.ListSecretsOutput, error)

	GetSecretValue(
		ctx context.Context,
		params *secretsmanager.GetSecretValueInput,
		optFns ...func(*secretsmanager.Options),
	) (*secretsmanager.GetSecretValueOutput, error)

	PutSecretValue(
		ctx context.Context,
		params *secretsmanager.PutSecretValueInput,
		optFns ...func(*secretsmanager.Options),
	) (*secretsmanager.PutSecretValueOutput, error)

	CreateSecret(
		ctx context.Context,
		params *secretsmanager.CreateSecretInput,
		optFns ...func(*secretsmanager.Options),
	) (*secretsmanager.CreateSecretOutput, error)
}

// EditorFieldType indicates whether a field's value is visible or hidden.
type EditorFieldType string

const (
	// EditorFieldText indicates a visible text field.
	EditorFieldText EditorFieldType = "text"
)

// EditorVault represents a secret vault grouping (path prefix in AWS).
type EditorVault struct {
	ID   string
	Name string
}

// EditorItem represents a secret item within a vault.
type EditorItem struct {
	ID    string
	Name  string
	Vault string
}

// EditorField represents a single key-value field within a secret.
type EditorField struct {
	ID      string
	Key     string
	Value   string
	Type    EditorFieldType
	Section string
}

// EditorFieldPair holds a key-value pair for creating items.
type EditorFieldPair struct {
	Key   string
	Value string
}

// AWSEditor provides read-write access to AWS Secrets Manager.
//
//nolint:revive // AWSEditor matches project convention (cf. OPEditor)
type AWSEditor struct {
	client smClient
	region string
}

// EditorOptions configures the AWS editor.
type EditorOptions struct {
	Region  string
	Profile string
}

// NewEditor creates a new AWS Secrets Manager editor.
func NewEditor(ctx context.Context, opts EditorOptions) (*AWSEditor, error) {
	var loadOpts []func(*config.LoadOptions) error

	if opts.Region != "" {
		loadOpts = append(loadOpts, config.WithRegion(opts.Region))
	}
	if opts.Profile != "" {
		loadOpts = append(loadOpts, config.WithSharedConfigProfile(opts.Profile))
	}

	cfg, err := config.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("aws editor: failed to load config: %w", err)
	}

	client := secretsmanager.NewFromConfig(cfg)

	return &AWSEditor{
		client: client,
		region: opts.Region,
	}, nil
}

// GetSecret retrieves all key-value pairs from a secret.
func (e *AWSEditor) GetSecret(ctx context.Context, secretName string) (map[string]string, error) {
	return e.getSecretJSON(ctx, secretName)
}

// GetSecretKey retrieves a specific key from a secret.
func (e *AWSEditor) GetSecretKey(ctx context.Context, secretName, key string) (string, error) {
	m, err := e.getSecretJSON(ctx, secretName)
	if err != nil {
		return "", err
	}

	value, ok := m[key]
	if !ok {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		return "", fmt.Errorf(
			"key %q not found in secret %q (available: %s)",
			key, secretName, strings.Join(keys, ", "),
		)
	}

	return value, nil
}

// Name returns the backend name.
func (e *AWSEditor) Name() string {
	return "aws"
}

// ListEditorVaults returns unique path prefixes from all secrets as vaults.
// Secrets without a "/" are treated as their own vault.
func (e *AWSEditor) ListEditorVaults(ctx context.Context) ([]EditorVault, error) {
	names, err := e.listAllSecretNames(ctx)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	for _, name := range names {
		prefix := name
		if idx := strings.Index(name, "/"); idx >= 0 {
			prefix = name[:idx]
		}
		seen[prefix] = true
	}

	vaults := make([]EditorVault, 0, len(seen))
	for prefix := range seen {
		vaults = append(vaults, EditorVault{ID: prefix, Name: prefix})
	}
	sort.Slice(vaults, func(i, j int) bool {
		return vaults[i].Name < vaults[j].Name
	})

	return vaults, nil
}

// ListEditorItems returns secrets matching the given vault prefix.
func (e *AWSEditor) ListEditorItems(ctx context.Context, vault string) ([]EditorItem, error) {
	names, err := e.listAllSecretNames(ctx)
	if err != nil {
		return nil, err
	}

	prefix := vault + "/"
	var items []EditorItem
	for _, name := range names {
		if strings.HasPrefix(name, prefix) {
			items = append(items, EditorItem{
				ID:    name,
				Name:  name,
				Vault: vault,
			})
		}
	}

	return items, nil
}

// GetEditorFields retrieves and parses a secret's JSON into fields.
func (e *AWSEditor) GetEditorFields(ctx context.Context, ref string) ([]EditorField, error) {
	m, err := e.getSecretJSON(ctx, ref)
	if err != nil {
		return nil, err
	}

	fields := make([]EditorField, 0, len(m))
	for k, v := range m {
		fields = append(fields, EditorField{
			Key:   k,
			Value: v,
			Type:  EditorFieldText,
		})
	}
	sort.Slice(fields, func(i, j int) bool {
		return fields[i].Key < fields[j].Key
	})

	return fields, nil
}

// UpdateEditorField performs a read-modify-write to update a field value.
func (e *AWSEditor) UpdateEditorField(ctx context.Context, ref, key, value string) error {
	m, err := e.getSecretJSON(ctx, ref)
	if err != nil {
		return err
	}

	m[key] = value
	return e.putSecretJSON(ctx, ref, m)
}

// DeleteEditorField performs a read-modify-write to remove a field.
func (e *AWSEditor) DeleteEditorField(ctx context.Context, ref, key string) error {
	m, err := e.getSecretJSON(ctx, ref)
	if err != nil {
		return err
	}

	delete(m, key)
	return e.putSecretJSON(ctx, ref, m)
}

// RenameEditorField performs a read-modify-write to rename a field key.
func (e *AWSEditor) RenameEditorField(ctx context.Context, ref, oldKey, newKey string) error {
	m, err := e.getSecretJSON(ctx, ref)
	if err != nil {
		return err
	}

	m[newKey] = m[oldKey]
	delete(m, oldKey)
	return e.putSecretJSON(ctx, ref, m)
}

// CreateEditorItem creates a new secret with the given name and initial fields.
func (e *AWSEditor) CreateEditorItem(ctx context.Context, vault, name string, fields []EditorFieldPair) error {
	secretName := name
	if vault != "" {
		secretName = vault + "/" + name
	}

	m := make(map[string]string, len(fields))
	for _, f := range fields {
		m[f.Key] = f.Value
	}

	body, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("failed to marshal fields: %w", err)
	}

	_, err = e.client.CreateSecret(ctx, &secretsmanager.CreateSecretInput{
		Name:         awssdk.String(secretName),
		SecretString: awssdk.String(string(body)),
	})
	if err != nil {
		return fmt.Errorf("failed to create secret %q: %w", secretName, err)
	}

	return nil
}

// listAllSecretNames retrieves all secret names, handling pagination.
func (e *AWSEditor) listAllSecretNames(ctx context.Context) ([]string, error) {
	var names []string
	var nextToken *string

	for {
		output, err := e.client.ListSecrets(ctx, &secretsmanager.ListSecretsInput{
			NextToken: nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list secrets: %w", err)
		}

		for _, entry := range output.SecretList {
			if entry.Name != nil {
				names = append(names, *entry.Name)
			}
		}

		if output.NextToken == nil || *output.NextToken == "" {
			break
		}
		nextToken = output.NextToken
	}

	return names, nil
}

// getSecretJSON retrieves a secret and parses it as JSON key-value pairs.
func (e *AWSEditor) getSecretJSON(ctx context.Context, ref string) (map[string]string, error) {
	output, err := e.client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: awssdk.String(ref),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get secret %q: %w", ref, err)
	}

	if output.SecretString == nil {
		return nil, fmt.Errorf("secret %q has no string value", ref)
	}

	var m map[string]string
	if err := json.Unmarshal([]byte(*output.SecretString), &m); err != nil {
		return nil, fmt.Errorf("failed to parse secret %q as JSON: %w", ref, err)
	}

	return m, nil
}

// putSecretJSON marshals data as JSON and writes it back to a secret.
func (e *AWSEditor) putSecretJSON(ctx context.Context, ref string, data map[string]string) error {
	body, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal secret %q: %w", ref, err)
	}

	_, err = e.client.PutSecretValue(ctx, &secretsmanager.PutSecretValueInput{
		SecretId:     awssdk.String(ref),
		SecretString: awssdk.String(string(body)),
	})
	if err != nil {
		return fmt.Errorf("failed to put secret %q: %w", ref, err)
	}

	return nil
}

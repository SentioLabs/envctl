//nolint:testpackage // Testing internal functions requires same package
package env

import (
	"context"
	"testing"

	"github.com/sentiolabs/envctl/internal/config"
	"github.com/sentiolabs/envctl/internal/errors"
	"github.com/sentiolabs/envctl/internal/mocks"
	"github.com/sentiolabs/envctl/internal/secrets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func boolPtr(v bool) *bool { return &v }

func TestBuilder_Build_LegacyMode_IncludeAll(t *testing.T) {
	ctx := t.Context()
	mockClient := mocks.NewMockClient(t)

	cfg := &config.Config{
		Version:            1,
		DefaultEnvironment: "dev",
		IncludeAll:         boolPtr(true),
		Environments: map[string]config.Environment{
			"dev": config.NewEnvironment(config.IncludeEntry{Secret: "my-app/dev"}),
		},
	}

	mockClient.On("GetSecret", mock.Anything, "my-app/dev").Return(map[string]string{
		"DB_HOST":     "localhost",
		"DB_USER":     "admin",
		"DB_PASSWORD": "secret123",
	}, nil)

	builder := NewBuilder(mockClient, cfg, "", "dev")
	entries, err := builder.Build(ctx, nil)

	require.NoError(t, err)
	assert.Len(t, entries, 3)

	entryMap := ToMap(entries)
	assert.Equal(t, "localhost", entryMap["DB_HOST"])
	assert.Equal(t, "admin", entryMap["DB_USER"])
	assert.Equal(t, "secret123", entryMap["DB_PASSWORD"])
}

func TestBuilder_Build_LegacyMode_MappingsOnly(t *testing.T) {
	ctx := t.Context()
	mockClient := mocks.NewMockClient(t)

	cfg := &config.Config{
		Version:            1,
		DefaultEnvironment: "dev",
		IncludeAll:         boolPtr(false),
		Environments: map[string]config.Environment{
			"dev": config.NewEnvironment(config.IncludeEntry{Secret: "my-app/dev"}),
		},
		Mapping: map[string]string{
			"DATABASE_URL": "my-app/dev#connection_string",
		},
	}

	mockClient.On("GetSecretKey", mock.Anything, "my-app/dev", "connection_string").
		Return("postgres://localhost:5432/mydb", nil)

	builder := NewBuilder(mockClient, cfg, "", "dev")
	entries, err := builder.Build(ctx, nil)

	require.NoError(t, err)
	assert.Len(t, entries, 1)

	entryMap := ToMap(entries)
	assert.Equal(t, "postgres://localhost:5432/mydb", entryMap["DATABASE_URL"])
}

func TestBuilder_Build_WithSources_SpecificKey(t *testing.T) {
	// Environment sources include entries with specific keys
	ctx := t.Context()
	mockClient := mocks.NewMockClient(t)

	cfg := &config.Config{
		Version:            1,
		DefaultEnvironment: "dev",
		IncludeAll:         boolPtr(false),
		Environments: map[string]config.Environment{
			"dev": {
				Sources: []config.IncludeEntry{
					{Secret: "my-app/dev"},
					{Secret: "shared/datadog", Key: "api_key", As: "DD_API_KEY"},
					{Secret: "shared/stripe", Key: "secret_key"},
				},
			},
		},
	}

	mockClient.On("GetSecretKey", mock.Anything, "shared/datadog", "api_key").
		Return("dd-api-key-12345", nil)
	mockClient.On("GetSecretKey", mock.Anything, "shared/stripe", "secret_key").
		Return("sk_live_12345", nil)

	builder := NewBuilder(mockClient, cfg, "", "dev")
	entries, err := builder.Build(ctx, nil)

	require.NoError(t, err)
	assert.Len(t, entries, 2)

	entryMap := ToMap(entries)
	assert.Equal(t, "dd-api-key-12345", entryMap["DD_API_KEY"])
	assert.Equal(t, "sk_live_12345", entryMap["secret_key"])
}

func TestBuilder_Build_WithSources_AllKeys(t *testing.T) {
	// Source entries that include all keys from a secret (requires include_all)
	ctx := t.Context()
	mockClient := mocks.NewMockClient(t)

	cfg := &config.Config{
		Version:            1,
		DefaultEnvironment: "dev",
		IncludeAll:         boolPtr(true),
		Environments: map[string]config.Environment{
			"dev": {
				Sources: []config.IncludeEntry{
					{Secret: "my-app/dev"},
					{Secret: "shared/common"}, // No key specified - includes all
				},
			},
		},
	}

	mockClient.On("GetSecret", mock.Anything, "my-app/dev").Return(map[string]string{
		"APP_SECRET": "app-secret-value",
	}, nil)

	mockClient.On("GetSecret", mock.Anything, "shared/common").Return(map[string]string{
		"LOG_LEVEL": "debug",
		"NODE_ENV":  "development",
	}, nil)

	builder := NewBuilder(mockClient, cfg, "", "dev")
	entries, err := builder.Build(ctx, nil)

	require.NoError(t, err)
	assert.Len(t, entries, 3)

	entryMap := ToMap(entries)
	assert.Equal(t, "app-secret-value", entryMap["APP_SECRET"])
	assert.Equal(t, "debug", entryMap["LOG_LEVEL"])
	assert.Equal(t, "development", entryMap["NODE_ENV"])
}

func TestBuilder_Build_SourceWithoutKey_RequiresIncludeAll(t *testing.T) {
	// Source entries without a key fail when include_all is disabled
	ctx := t.Context()
	mockClient := mocks.NewMockClient(t)

	cfg := &config.Config{
		Version:            1,
		DefaultEnvironment: "dev",
		IncludeAll:         boolPtr(false),
		Environments: map[string]config.Environment{
			"dev": {
				Sources: []config.IncludeEntry{
					{Secret: "my-app/dev"},
					{Secret: "shared/common"}, // No key - should fail
				},
			},
		},
	}

	builder := NewBuilder(mockClient, cfg, "", "dev")
	_, err := builder.Build(ctx, nil)

	require.Error(t, err)
	var includeErr *errors.IncludeAllRequiredError
	require.ErrorAs(t, err, &includeErr)
	assert.Equal(t, "shared/common", includeErr.Secret)
}

func TestBuilder_Build_WithOverrides(t *testing.T) {
	ctx := t.Context()
	mockClient := mocks.NewMockClient(t)

	cfg := &config.Config{
		Version:            1,
		DefaultEnvironment: "dev",
		IncludeAll:         boolPtr(true),
		Environments: map[string]config.Environment{
			"dev": config.NewEnvironment(config.IncludeEntry{Secret: "my-app/dev"}),
		},
	}

	mockClient.On("GetSecret", mock.Anything, "my-app/dev").Return(map[string]string{
		"DB_HOST": "from-secret",
		"DB_PORT": "5432",
	}, nil)

	builder := NewBuilder(mockClient, cfg, "", "dev")
	entries, err := builder.Build(ctx, map[string]string{
		"DB_HOST": "override-host",
		"NEW_VAR": "new-value",
	})

	require.NoError(t, err)
	assert.Len(t, entries, 3)

	entryMap := ToMap(entries)
	assert.Equal(t, "override-host", entryMap["DB_HOST"])
	assert.Equal(t, "5432", entryMap["DB_PORT"])
	assert.Equal(t, "new-value", entryMap["NEW_VAR"])
}

func TestBuilder_Build_CLIIncludeAllOverride(t *testing.T) {
	ctx := t.Context()
	mockClient := mocks.NewMockClient(t)

	cfg := &config.Config{
		Version:            1,
		DefaultEnvironment: "dev",
		IncludeAll:         boolPtr(false),
		Environments: map[string]config.Environment{
			"dev": config.NewEnvironment(config.IncludeEntry{Secret: "my-app/dev"}),
		},
	}

	mockClient.On("GetSecret", mock.Anything, "my-app/dev").Return(map[string]string{
		"SECRET_KEY": "value",
	}, nil)

	builder := NewBuilder(mockClient, cfg, "", "dev").
		WithIncludeAll(boolPtr(true))

	entries, err := builder.Build(ctx, nil)

	require.NoError(t, err)
	assert.Len(t, entries, 1)
	entryMap := ToMap(entries)
	assert.Equal(t, "value", entryMap["SECRET_KEY"])
}

func TestBuilder_Build_ApplicationMode(t *testing.T) {
	ctx := t.Context()
	mockClient := mocks.NewMockClient(t)

	cfg := &config.Config{
		Version:            1,
		DefaultApplication: "api",
		Applications: map[string]*config.Application{
			"api": {
				Environments: map[string]config.Environment{
					"dev": config.NewEnvironment(config.IncludeEntry{Secret: "api/dev"}),
				},
				IncludeAll: boolPtr(true),
			},
		},
	}

	mockClient.On("GetSecret", mock.Anything, "api/dev").Return(map[string]string{
		"API_KEY": "api-key-value",
	}, nil)

	builder := NewBuilder(mockClient, cfg, "api", "dev")
	entries, err := builder.Build(ctx, nil)

	require.NoError(t, err)
	assert.Len(t, entries, 1)
	entryMap := ToMap(entries)
	assert.Equal(t, "api-key-value", entryMap["API_KEY"])
}

func TestBuilder_Build_ErrorFromSecretClient(t *testing.T) {
	ctx := t.Context()
	mockClient := mocks.NewMockClient(t)

	cfg := &config.Config{
		Version:            1,
		DefaultEnvironment: "dev",
		IncludeAll:         boolPtr(true),
		Environments: map[string]config.Environment{
			"dev": config.NewEnvironment(config.IncludeEntry{Secret: "my-app/dev"}),
		},
	}

	expectedErr := &errors.SecretNotFoundError{SecretName: "my-app/dev"}
	mockClient.On("GetSecret", mock.Anything, "my-app/dev").Return(nil, expectedErr)

	builder := NewBuilder(mockClient, cfg, "", "dev")
	_, err := builder.Build(ctx, nil)

	require.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
}

func TestBuilder_Build_ApplicationMode_1PassBackend(t *testing.T) {
	ctx := t.Context()
	mockClient := mocks.NewMockClient(t)

	cfg := &config.Config{
		Version:            1,
		DefaultApplication: "api",
		Applications: map[string]*config.Application{
			"api": {
				Environments: map[string]config.Environment{
					"local": config.NewEnvironment(config.IncludeEntry{
						Secret:  "My App Local",
						OnePass: &config.OnePassConfig{Vault: "Development"},
					}),
				},
				IncludeAll: boolPtr(true),
			},
		},
	}

	mockClient.On("GetSecret", mock.Anything, "My App Local").Return(map[string]string{
		"API_KEY": "local-key",
	}, nil)

	builder := NewBuilder(mockClient, cfg, "api", "local")
	entries, err := builder.Build(ctx, nil)

	require.NoError(t, err)
	assert.Len(t, entries, 1)
	entryMap := ToMap(entries)
	assert.Equal(t, "local-key", entryMap["API_KEY"])
}

func TestBuilder_Build_LegacyMode_AWSConfig(t *testing.T) {
	ctx := t.Context()
	mockClient := mocks.NewMockClient(t)

	cfg := &config.Config{
		Version:            1,
		DefaultEnvironment: "staging",
		IncludeAll:         boolPtr(true),
		Environments: map[string]config.Environment{
			"staging": config.NewEnvironment(config.IncludeEntry{
				Secret: "myapp/staging",
				AWS:    &config.AWSConfig{Region: "us-west-2", Profile: "staging"},
			}),
		},
	}

	mockClient.On("GetSecret", mock.Anything, "myapp/staging").Return(map[string]string{
		"DB_HOST": "staging-db.example.com",
	}, nil)

	builder := NewBuilder(mockClient, cfg, "", "staging")
	entries, err := builder.Build(ctx, nil)

	require.NoError(t, err)
	assert.Len(t, entries, 1)
	entryMap := ToMap(entries)
	assert.Equal(t, "staging-db.example.com", entryMap["DB_HOST"])
}

func TestBuilder_Build_SourcesOnlyProcessActiveEnv(t *testing.T) {
	// Building for "dev" should only process dev sources, not staging
	ctx := t.Context()
	mockClient := mocks.NewMockClient(t)

	cfg := &config.Config{
		Version:            1,
		DefaultEnvironment: "dev",
		IncludeAll:         boolPtr(false),
		Environments: map[string]config.Environment{
			"dev": {
				Sources: []config.IncludeEntry{
					{Secret: "my-app/dev"},
					{Secret: "shared/dev-tools", Key: "api_key", As: "DEV_API_KEY"},
				},
			},
			"staging": {
				Sources: []config.IncludeEntry{
					{Secret: "my-app/staging"},
					{Secret: "shared/staging-monitor", Key: "token", As: "MONITOR_TOKEN"},
				},
			},
		},
	}

	// Only dev sources should be called
	mockClient.On("GetSecretKey", mock.Anything, "shared/dev-tools", "api_key").
		Return("dev-key-123", nil)

	builder := NewBuilder(mockClient, cfg, "", "dev")
	entries, err := builder.Build(ctx, nil)

	require.NoError(t, err)
	assert.Len(t, entries, 1)

	entryMap := ToMap(entries)
	assert.Equal(t, "dev-key-123", entryMap["DEV_API_KEY"])

	// Verify staging sources were NOT called
	mockClient.AssertNotCalled(t, "GetSecretKey", mock.Anything, "shared/staging-monitor", "token")
}

func TestBuilder_Build_CrossBackendSource(t *testing.T) {
	// Primary env uses 1pass; one source has aws config (different backend)
	ctx := t.Context()
	primaryClient := mocks.NewMockClient(t)
	awsClient := mocks.NewMockClient(t)

	cfg := &config.Config{
		Version:            1,
		DefaultEnvironment: "dev",
		IncludeAll:         boolPtr(false),
		OnePass:            &config.OnePassConfig{Vault: "Dev"},
		Environments: map[string]config.Environment{
			"dev": {
				Sources: []config.IncludeEntry{
					{Secret: "My App Dev", OnePass: &config.OnePassConfig{Vault: "Development"}},
					{
						Secret: "aws-shared/datadog",
						Key:    "api_key",
						As:     "DD_API_KEY",
						AWS:    &config.AWSConfig{Region: "us-east-1"},
					},
				},
				OnePass: &config.OnePassConfig{Vault: "Development"},
			},
		},
	}

	awsClient.On("GetSecretKey", mock.Anything, "aws-shared/datadog", "api_key").
		Return("dd-key-from-aws", nil)

	builder := NewBuilder(primaryClient, cfg, "", "dev")
	builder.newClient = func(
		_ context.Context, opts secrets.Options,
	) (secrets.Client, error) {
		assert.NotNil(t, opts.Env)
		assert.NotNil(t, opts.Env.AWS)
		assert.Equal(t, "us-east-1", opts.Env.AWS.Region)
		return awsClient, nil
	}

	entries, err := builder.Build(ctx, nil)

	require.NoError(t, err)
	assert.Len(t, entries, 1)

	entryMap := ToMap(entries)
	assert.Equal(t, "dd-key-from-aws", entryMap["DD_API_KEY"])
}

func TestBuilder_Build_SourceWithoutBackendQualifier(t *testing.T) {
	// Source entry has no aws/1pass fields — should use primary client
	ctx := t.Context()
	primaryClient := mocks.NewMockClient(t)

	cfg := &config.Config{
		Version:            1,
		DefaultEnvironment: "dev",
		IncludeAll:         boolPtr(false),
		Environments: map[string]config.Environment{
			"dev": {
				Sources: []config.IncludeEntry{
					{Secret: "my-app/dev"},
					{Secret: "shared/common", Key: "api_key", As: "COMMON_API_KEY"},
				},
			},
		},
	}

	primaryClient.On("GetSecretKey", mock.Anything, "shared/common", "api_key").
		Return("common-key-value", nil)

	newClientCalled := false
	builder := NewBuilder(primaryClient, cfg, "", "dev")
	builder.newClient = func(
		_ context.Context, _ secrets.Options,
	) (secrets.Client, error) {
		newClientCalled = true
		return nil, nil //nolint:nilnil // Test stub intentionally unused
	}

	entries, err := builder.Build(ctx, nil)

	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.False(t, newClientCalled, "newClient should not be called for sources without backend qualifier")

	entryMap := ToMap(entries)
	assert.Equal(t, "common-key-value", entryMap["COMMON_API_KEY"])
}

func TestBuilder_Build_SourceWithKeys(t *testing.T) {
	// Extract multiple keys from the same secret using the keys: field
	ctx := t.Context()
	mockClient := mocks.NewMockClient(t)

	cfg := &config.Config{
		Version:            1,
		DefaultEnvironment: "dev",
		IncludeAll:         boolPtr(false),
		Environments: map[string]config.Environment{
			"dev": {
				Sources: []config.IncludeEntry{
					{Secret: "my-app/dev"},
					{
						Secret: "shared/database",
						Keys: []config.KeyMapping{
							{Key: "database_host", As: "DATABASE_HOST"},
							{Key: "database_password", As: "DATABASE_PASSWORD"},
							{Key: "database_name", As: "DATABASE_NAME"},
						},
					},
				},
			},
		},
	}

	mockClient.On("GetSecretKey", mock.Anything, "shared/database", "database_host").
		Return("db.example.com", nil)
	mockClient.On("GetSecretKey", mock.Anything, "shared/database", "database_password").
		Return("s3cret", nil)
	mockClient.On("GetSecretKey", mock.Anything, "shared/database", "database_name").
		Return("mydb", nil)

	builder := NewBuilder(mockClient, cfg, "", "dev")
	entries, err := builder.Build(ctx, nil)

	require.NoError(t, err)
	assert.Len(t, entries, 3)

	entryMap := ToMap(entries)
	assert.Equal(t, "db.example.com", entryMap["DATABASE_HOST"])
	assert.Equal(t, "s3cret", entryMap["DATABASE_PASSWORD"])
	assert.Equal(t, "mydb", entryMap["DATABASE_NAME"])
}

func TestBuilder_Build_SourceWithKeysCrossBackend(t *testing.T) {
	// keys: with a cross-backend qualifier (primary=1pass, source=aws)
	ctx := t.Context()
	primaryClient := mocks.NewMockClient(t)
	awsClient := mocks.NewMockClient(t)

	cfg := &config.Config{
		Version:            1,
		DefaultEnvironment: "dev",
		IncludeAll:         boolPtr(false),
		OnePass:            &config.OnePassConfig{Vault: "Dev"},
		Environments: map[string]config.Environment{
			"dev": {
				Sources: []config.IncludeEntry{
					{Secret: "My App Dev", OnePass: &config.OnePassConfig{Vault: "Development"}},
					{
						Secret: "dev/app/secrets",
						AWS:    &config.AWSConfig{Region: "us-east-1"},
						Keys: []config.KeyMapping{
							{Key: "database_host", As: "DATABASE_HOST"},
							{Key: "database_password", As: "DATABASE_PASSWORD"},
						},
					},
				},
				OnePass: &config.OnePassConfig{Vault: "Development"},
			},
		},
	}

	awsClient.On("GetSecretKey", mock.Anything, "dev/app/secrets", "database_host").
		Return("aws-db-host", nil)
	awsClient.On("GetSecretKey", mock.Anything, "dev/app/secrets", "database_password").
		Return("aws-db-pass", nil)

	builder := NewBuilder(primaryClient, cfg, "", "dev")
	builder.newClient = func(
		_ context.Context, opts secrets.Options,
	) (secrets.Client, error) {
		assert.NotNil(t, opts.Env)
		assert.NotNil(t, opts.Env.AWS)
		assert.Equal(t, "us-east-1", opts.Env.AWS.Region)
		return awsClient, nil
	}

	entries, err := builder.Build(ctx, nil)

	require.NoError(t, err)
	assert.Len(t, entries, 2)

	entryMap := ToMap(entries)
	assert.Equal(t, "aws-db-host", entryMap["DATABASE_HOST"])
	assert.Equal(t, "aws-db-pass", entryMap["DATABASE_PASSWORD"])
}

func TestBuilder_Build_SourceWithKeysAsOptional(t *testing.T) {
	// When 'as' is omitted in keys entries, the key name is used as-is
	ctx := t.Context()
	mockClient := mocks.NewMockClient(t)

	cfg := &config.Config{
		Version:            1,
		DefaultEnvironment: "dev",
		IncludeAll:         boolPtr(false),
		Environments: map[string]config.Environment{
			"dev": {
				Sources: []config.IncludeEntry{
					{Secret: "my-app/dev"},
					{
						Secret: "shared/config",
						Keys: []config.KeyMapping{
							{Key: "log_level"},          // No 'as' — should use "log_level"
							{Key: "api_url", As: "URL"}, // With 'as'
						},
					},
				},
			},
		},
	}

	mockClient.On("GetSecretKey", mock.Anything, "shared/config", "log_level").
		Return("debug", nil)
	mockClient.On("GetSecretKey", mock.Anything, "shared/config", "api_url").
		Return("https://api.example.com", nil)

	builder := NewBuilder(mockClient, cfg, "", "dev")
	entries, err := builder.Build(ctx, nil)

	require.NoError(t, err)
	assert.Len(t, entries, 2)

	entryMap := ToMap(entries)
	assert.Equal(t, "debug", entryMap["log_level"])
	assert.Equal(t, "https://api.example.com", entryMap["URL"])
}

func TestToMap(t *testing.T) {
	entries := []Entry{
		{Key: "KEY1", Value: "value1", Source: "secret1"},
		{Key: "KEY2", Value: "value2", Source: "secret2"},
		{Key: "KEY3", Value: "value3", Source: "override"},
	}

	result := ToMap(entries)

	assert.Len(t, result, 3)
	assert.Equal(t, "value1", result["KEY1"])
	assert.Equal(t, "value2", result["KEY2"])
	assert.Equal(t, "value3", result["KEY3"])
}

func TestToMap_EmptyInput(t *testing.T) {
	result := ToMap([]Entry{})
	assert.Empty(t, result)

	result = ToMap(nil)
	assert.Empty(t, result)
}

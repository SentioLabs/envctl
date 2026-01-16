//nolint:testpackage // Testing internal functions requires same package
package env

import (
	"context"
	"testing"

	"github.com/sentiolabs/envctl/internal/config"
	"github.com/sentiolabs/envctl/internal/errors"
	"github.com/sentiolabs/envctl/internal/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func boolPtr(b bool) *bool {
	return &b
}

func TestBuilder_Build_LegacyMode_IncludeAll(t *testing.T) {
	// Test building environment variables in legacy mode with include_all enabled
	ctx := context.Background()
	mockClient := mocks.NewMockClient(t)

	cfg := &config.Config{
		Version:            1,
		DefaultEnvironment: "dev",
		IncludeAll:         boolPtr(true),
		Environments: map[string]config.Environment{
			"dev": {Secret: "my-app/dev"},
		},
	}

	// Mock the primary secret retrieval
	mockClient.On("GetSecret", mock.Anything, "my-app/dev").Return(map[string]string{
		"DB_HOST":     "localhost",
		"DB_USER":     "admin",
		"DB_PASSWORD": "secret123",
	}, nil)

	builder := NewBuilder(mockClient, cfg, "", "dev")
	entries, err := builder.Build(ctx, nil)

	require.NoError(t, err)
	assert.Len(t, entries, 3)

	// Convert to map for easier assertions
	entryMap := ToMap(entries)
	assert.Equal(t, "localhost", entryMap["DB_HOST"])
	assert.Equal(t, "admin", entryMap["DB_USER"])
	assert.Equal(t, "secret123", entryMap["DB_PASSWORD"])
}

func TestBuilder_Build_LegacyMode_MappingsOnly(t *testing.T) {
	// Test building environment variables with explicit mappings only (include_all disabled)
	ctx := context.Background()
	mockClient := mocks.NewMockClient(t)

	cfg := &config.Config{
		Version:            1,
		DefaultEnvironment: "dev",
		IncludeAll:         boolPtr(false), // Explicitly disabled
		Environments: map[string]config.Environment{
			"dev": {Secret: "my-app/dev"},
		},
		Mapping: map[string]string{
			"DATABASE_URL": "my-app/dev#connection_string",
		},
	}

	// Mock the mapping retrieval - only the specific key is requested
	mockClient.On("GetSecretKey", mock.Anything, "my-app/dev", "connection_string").
		Return("postgres://localhost:5432/mydb", nil)

	builder := NewBuilder(mockClient, cfg, "", "dev")
	entries, err := builder.Build(ctx, nil)

	require.NoError(t, err)
	assert.Len(t, entries, 1)

	entryMap := ToMap(entries)
	assert.Equal(t, "postgres://localhost:5432/mydb", entryMap["DATABASE_URL"])
}

func TestBuilder_Build_WithIncludes_SpecificKey(t *testing.T) {
	// Test include entries that specify a specific key
	ctx := context.Background()
	mockClient := mocks.NewMockClient(t)

	cfg := &config.Config{
		Version:            1,
		DefaultEnvironment: "dev",
		IncludeAll:         boolPtr(false),
		Environments: map[string]config.Environment{
			"dev": {Secret: "my-app/dev"},
		},
		Include: []config.IncludeEntry{
			{Secret: "shared/datadog", Key: "api_key", As: "DD_API_KEY"},
			{Secret: "shared/stripe", Key: "secret_key"}, // Uses original key name
		},
	}

	// Mock the include retrievals
	mockClient.On("GetSecretKey", mock.Anything, "shared/datadog", "api_key").
		Return("dd-api-key-12345", nil)
	mockClient.On("GetSecretKey", mock.Anything, "shared/stripe", "secret_key").
		Return("sk_live_12345", nil)

	builder := NewBuilder(mockClient, cfg, "", "dev")
	entries, err := builder.Build(ctx, nil)

	require.NoError(t, err)
	assert.Len(t, entries, 2)

	entryMap := ToMap(entries)
	assert.Equal(t, "dd-api-key-12345", entryMap["DD_API_KEY"]) // Renamed with 'as'
	assert.Equal(t, "sk_live_12345", entryMap["secret_key"])    // Original key name
}

func TestBuilder_Build_WithIncludes_AllKeys(t *testing.T) {
	// Test include entries that include all keys from a secret (requires include_all)
	ctx := context.Background()
	mockClient := mocks.NewMockClient(t)

	cfg := &config.Config{
		Version:            1,
		DefaultEnvironment: "dev",
		IncludeAll:         boolPtr(true),
		Environments: map[string]config.Environment{
			"dev": {Secret: "my-app/dev"},
		},
		Include: []config.IncludeEntry{
			{Secret: "shared/common"}, // No key specified - includes all
		},
	}

	// Mock the primary secret
	mockClient.On("GetSecret", mock.Anything, "my-app/dev").Return(map[string]string{
		"APP_SECRET": "app-secret-value",
	}, nil)

	// Mock the include secret (all keys)
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

func TestBuilder_Build_IncludeWithoutKey_RequiresIncludeAll(t *testing.T) {
	// Test that include entries without a key fail when include_all is disabled
	ctx := context.Background()
	mockClient := mocks.NewMockClient(t)

	cfg := &config.Config{
		Version:            1,
		DefaultEnvironment: "dev",
		IncludeAll:         boolPtr(false),
		Environments: map[string]config.Environment{
			"dev": {Secret: "my-app/dev"},
		},
		Include: []config.IncludeEntry{
			{Secret: "shared/common"}, // No key - should fail
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
	// Test that overrides take precedence over all other sources
	ctx := context.Background()
	mockClient := mocks.NewMockClient(t)

	cfg := &config.Config{
		Version:            1,
		DefaultEnvironment: "dev",
		IncludeAll:         boolPtr(true),
		Environments: map[string]config.Environment{
			"dev": {Secret: "my-app/dev"},
		},
	}

	mockClient.On("GetSecret", mock.Anything, "my-app/dev").Return(map[string]string{
		"DB_HOST": "from-secret",
		"DB_PORT": "5432",
	}, nil)

	builder := NewBuilder(mockClient, cfg, "", "dev")
	entries, err := builder.Build(ctx, map[string]string{
		"DB_HOST": "override-host", // Override from secret
		"NEW_VAR": "new-value",     // Additional variable
	})

	require.NoError(t, err)
	assert.Len(t, entries, 3)

	entryMap := ToMap(entries)
	assert.Equal(t, "override-host", entryMap["DB_HOST"]) // Overridden
	assert.Equal(t, "5432", entryMap["DB_PORT"])          // From secret
	assert.Equal(t, "new-value", entryMap["NEW_VAR"])     // From override
}

func TestBuilder_Build_CLIIncludeAllOverride(t *testing.T) {
	// Test that CLI --include-all flag overrides config setting
	ctx := context.Background()
	mockClient := mocks.NewMockClient(t)

	cfg := &config.Config{
		Version:            1,
		DefaultEnvironment: "dev",
		IncludeAll:         boolPtr(false), // Config says false
		Environments: map[string]config.Environment{
			"dev": {Secret: "my-app/dev"},
		},
	}

	// When CLI override is true, GetSecret should be called
	mockClient.On("GetSecret", mock.Anything, "my-app/dev").Return(map[string]string{
		"SECRET_KEY": "value",
	}, nil)

	builder := NewBuilder(mockClient, cfg, "", "dev").
		WithIncludeAll(boolPtr(true)) // CLI override

	entries, err := builder.Build(ctx, nil)

	require.NoError(t, err)
	assert.Len(t, entries, 1)
	entryMap := ToMap(entries)
	assert.Equal(t, "value", entryMap["SECRET_KEY"])
}

func TestBuilder_Build_ApplicationMode(t *testing.T) {
	// Test building environment in application mode
	ctx := context.Background()
	mockClient := mocks.NewMockClient(t)

	cfg := &config.Config{
		Version:            1,
		DefaultApplication: "api",
		Applications: map[string]*config.Application{
			"api": {
				Environments: map[string]config.Environment{
					"dev": {Secret: "api/dev"},
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
	// Test that errors from the secrets client are propagated
	ctx := context.Background()
	mockClient := mocks.NewMockClient(t)

	cfg := &config.Config{
		Version:            1,
		DefaultEnvironment: "dev",
		IncludeAll:         boolPtr(true),
		Environments: map[string]config.Environment{
			"dev": {Secret: "my-app/dev"},
		},
	}

	expectedErr := &errors.SecretNotFoundError{SecretName: "my-app/dev"}
	mockClient.On("GetSecret", mock.Anything, "my-app/dev").Return(nil, expectedErr)

	builder := NewBuilder(mockClient, cfg, "", "dev")
	_, err := builder.Build(ctx, nil)

	require.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
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

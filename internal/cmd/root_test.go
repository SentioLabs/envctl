//nolint:testpackage // Testing internal functions requires same package
package cmd

import (
	"testing"

	"github.com/sentiolabs/envctl/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateSecretsClientSignature(t *testing.T) {
	devEnv := config.NewEnvironment(
		config.IncludeEntry{Secret: "test/secret"},
	)
	cfg := &config.Config{
		Version:      1,
		Environments: map[string]config.Environment{"dev": devEnv},
	}

	ctx := t.Context()
	client, err := createSecretsClient(ctx, cfg, nil)
	require.NoError(t, err)
	assert.NotNil(t, client)
}

func TestCreateSecretsClientWithEnvironment(t *testing.T) {
	devEnv := config.NewEnvironment(
		config.IncludeEntry{Secret: "test/secret"},
	)
	cfg := &config.Config{
		Version:      1,
		Environments: map[string]config.Environment{"dev": devEnv},
	}
	env := config.NewEnvironment(config.IncludeEntry{
		Secret: "test/secret",
		AWS:    &config.AWSConfig{Region: "us-west-2"},
	})
	envConfig := &env

	ctx := t.Context()
	client, err := createSecretsClient(ctx, cfg, envConfig)
	require.NoError(t, err)
	assert.NotNil(t, client)
}

func TestCreateSecretsClientUsesResolveBackend(t *testing.T) {
	devEnv := config.NewEnvironment(
		config.IncludeEntry{Secret: "test/secret"},
	)
	cfg := &config.Config{
		Version:      1,
		Environments: map[string]config.Environment{"dev": devEnv},
	}

	env := config.NewEnvironment(config.IncludeEntry{
		Secret:  "op://vault/item",
		OnePass: &config.OnePassConfig{Vault: "test-vault"},
	})
	envConfig := &env

	backend := cfg.ResolveBackend(envConfig)
	assert.Equal(t, config.Backend1Pass, backend)
}

func TestRootCmdLongDescription(t *testing.T) {
	assert.Contains(t, rootCmd.Long, "secrets backends")
	assert.Contains(t, rootCmd.Long, "1Password")
}

func TestRunCmdLongDescription(t *testing.T) {
	assert.Contains(t, runCmd.Long, "configured backend")
}

func TestValidateCmdDescriptions(t *testing.T) {
	assert.Contains(t, validateCmd.Short, "backend connectivity")
	assert.Contains(t, validateCmd.Long, "backend connectivity")
	assert.Contains(t, validateCmd.Long, "Backend credentials are valid")
}

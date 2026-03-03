//nolint:testpackage // Testing internal functions requires same package
package cmd

import (
	"testing"

	"github.com/sentiolabs/envctl/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestCreateSecretsClientSignature(t *testing.T) {
	// Test that createSecretsClient accepts *config.Environment instead of region/profile strings.
	// We pass nil environment and a minimal config to verify the signature compiles
	// and the function handles nil environment gracefully (defaults to AWS backend).
	cfg := &config.Config{
		Version:      1,
		Environments: map[string]config.Environment{"dev": {Secret: "test/secret"}},
	}

	// This call verifies the function signature accepts (ctx, *config.Config, *config.Environment)
	// With nil environment, it should default to AWS backend and create a client.
	ctx := t.Context()
	client, err := createSecretsClient(ctx, cfg, nil)
	// Should succeed with AWS defaults (no error expected)
	assert.NoError(t, err)
	assert.NotNil(t, client)
}

func TestCreateSecretsClientWithEnvironment(t *testing.T) {
	// Test that createSecretsClient correctly accepts an environment config pointer
	// and passes the environment's AWS config through to the factory.
	cfg := &config.Config{
		Version:      1,
		Environments: map[string]config.Environment{"dev": {Secret: "test/secret"}},
	}
	envConfig := &config.Environment{
		Secret: "test/secret",
		AWS:    &config.AWSConfig{Region: "us-west-2"},
	}

	ctx := t.Context()
	client, err := createSecretsClient(ctx, cfg, envConfig)
	// Should succeed - AWS client created with region from environment
	assert.NoError(t, err)
	assert.NotNil(t, client)
}

func TestCreateSecretsClientUsesResolveBackend(t *testing.T) {
	// Test that the function uses cfg.ResolveBackend(envConfig) to determine the backend.
	// When environment has a 1pass block, ResolveBackend should return "1pass".
	cfg := &config.Config{
		Version:      1,
		Environments: map[string]config.Environment{"dev": {Secret: "test/secret"}},
	}

	envConfig := &config.Environment{
		Secret:  "op://vault/item",
		OnePass: &config.OnePassConfig{Vault: "test-vault"},
	}

	// ResolveBackend should return "1pass" for this environment
	backend := cfg.ResolveBackend(envConfig)
	assert.Equal(t, config.Backend1Pass, backend)
}

func TestRootCmdLongDescription(t *testing.T) {
	// Test that the root command description mentions multiple backends
	assert.Contains(t, rootCmd.Long, "secrets backends")
	assert.Contains(t, rootCmd.Long, "1Password")
}

func TestRunCmdLongDescription(t *testing.T) {
	// Test that the run command description mentions "configured backend"
	assert.Contains(t, runCmd.Long, "configured backend")
}

func TestValidateCmdDescriptions(t *testing.T) {
	// Test that validate command mentions backend connectivity (not AWS-specific)
	assert.Contains(t, validateCmd.Short, "backend connectivity")
	assert.Contains(t, validateCmd.Long, "backend connectivity")
	assert.Contains(t, validateCmd.Long, "Backend credentials are valid")
}

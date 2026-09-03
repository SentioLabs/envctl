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

// spKeyFile and kubeconfigVar avoid re-triggering goconst against the
// matching literals already used in files_unit_test.go.
const (
	spKeyFile     = "sp.key"
	kubeconfigVar = "KUBECONFIG"
)

func TestFileSinkListEntries(t *testing.T) {
	envCfg := config.NewEnvironment(
		config.IncludeEntry{Secret: "app/dev"},
		config.IncludeEntry{Secret: "app/dev/key", File: &config.FileSink{Name: spKeyFile, PathAs: "KEY_FILE"}},
		config.IncludeEntry{
			Secret: "app/dev/kube", Key: "config",
			File: &config.FileSink{Path: "/x/kube", PathAs: kubeconfigVar},
		},
	)
	cfg := &config.Config{Version: 1, FilesDirAs: "FILES_DIR"}

	entries := fileSinkListEntries(cfg, nil, &envCfg)
	require.Len(t, entries, 3)
	assert.Equal(t, "KEY_FILE", entries[0].Key)
	assert.Equal(t, "file:app/dev/key", entries[0].Source)
	assert.Empty(t, entries[0].Value, "list entries carry no value")
	assert.Equal(t, kubeconfigVar, entries[1].Key)
	assert.Equal(t, "FILES_DIR", entries[2].Key)
}

func TestFileSinkListEntries_NoDirVarWithoutEphemeral(t *testing.T) {
	envCfg := config.NewEnvironment(
		config.IncludeEntry{Secret: "app/dev"},
		config.IncludeEntry{Secret: "app/dev/kube", File: &config.FileSink{Path: "/x/kube", PathAs: kubeconfigVar}},
	)
	cfg := &config.Config{Version: 1, FilesDirAs: "FILES_DIR"}
	entries := fileSinkListEntries(cfg, nil, &envCfg)
	require.Len(t, entries, 1)
	assert.Equal(t, kubeconfigVar, entries[0].Key)
}

func TestCountSpecificSourcesSkipsSinks(t *testing.T) {
	sources := []config.IncludeEntry{
		{Secret: "a", Key: "K"},
		{Secret: "b", File: &config.FileSink{Name: "f", PathAs: "F"}},
	}
	assert.Equal(t, 1, countSpecificSources(sources))
	assert.False(t, hasWildcardSources(sources), "a sink without key is not a wildcard source")
}

func TestOutputCommandsMentionFileSinks(t *testing.T) {
	assert.Contains(t, envCmd.Long, "file.path")
	assert.Contains(t, exportCmd.Long, "file.path")
	assert.Contains(t, getCmd.Long, "file.path")
}

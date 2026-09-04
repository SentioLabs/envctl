//nolint:testpackage // Testing internal functions requires same package
package cmd

import (
	"context"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/sentiolabs/envctl/internal/config"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testSecretRef is the secret reference used by the fixtures in this file.
const testSecretRef = "test/secret"

func TestCreateSecretsClientSignature(t *testing.T) {
	devEnv := config.NewEnvironment(
		config.IncludeEntry{Secret: testSecretRef},
	)
	cfg := &config.Config{
		Version:      1,
		Environments: map[string]config.Environment{testDevEnv: devEnv},
	}

	ctx := t.Context()
	client, err := createSecretsClient(ctx, cfg, nil)
	require.NoError(t, err)
	assert.NotNil(t, client)
}

func TestCreateSecretsClientWithEnvironment(t *testing.T) {
	devEnv := config.NewEnvironment(
		config.IncludeEntry{Secret: testSecretRef},
	)
	cfg := &config.Config{
		Version:      1,
		Environments: map[string]config.Environment{testDevEnv: devEnv},
	}
	env := config.NewEnvironment(config.IncludeEntry{
		Secret: testSecretRef,
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
		config.IncludeEntry{Secret: testSecretRef},
	)
	cfg := &config.Config{
		Version:      1,
		Environments: map[string]config.Environment{testDevEnv: devEnv},
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

// TestExecuteRootGivesCommandsACancellableContext guards the ExecuteContext
// wiring. Under plain Execute cobra hands commands context.Background(),
// whose Done channel is nil, and exec.Cmd then skips its cancellation
// watcher, so a cancelled context never stops an install pipeline.
func TestExecuteRootGivesCommandsACancellableContext(t *testing.T) {
	var probeCtx context.Context
	probe := &cobra.Command{
		Use:    "ctxprobe",
		Hidden: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			probeCtx = cmd.Context()
			return nil
		},
	}
	rootCmd.AddCommand(probe)
	rootCmd.SetArgs([]string{"ctxprobe"})
	// executeRoot leaves its cancelled signal context on rootCmd, and cobra
	// reuses a stored context for every later Execute.
	t.Cleanup(func() {
		rootCmd.RemoveCommand(probe)
		rootCmd.SetArgs(nil)
		rootCmd.SetContext(context.Background())
	})

	assert.Zero(t, executeRoot())
	require.NotNil(t, probeCtx)
	assert.NotNil(t, probeCtx.Done(), "commands must get a context that cancels on SIGINT or SIGTERM")
	// executeRoot's deferred stop cancels the context it handed down, so a
	// command that outlives it cannot keep using a dead signal context.
	assert.ErrorIs(t, probeCtx.Err(), context.Canceled)
}

// TestSignalContextCancelsOnSIGINT delivers a real SIGINT to the test
// process. signalContext has a handler installed at that point, so the
// signal cancels the context instead of killing the binary.
func TestSignalContextCancelsOnSIGINT(t *testing.T) {
	ctx, stop := signalContext()
	defer stop()

	require.NoError(t, syscall.Kill(os.Getpid(), syscall.SIGINT))

	select {
	case <-ctx.Done():
		require.ErrorIs(t, ctx.Err(), context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("SIGINT did not cancel the signal context")
	}
}

// TestCommandContextFallsBackToBackground covers the RunE functions the
// tests call directly, where cobra never attached a context.
func TestCommandContextFallsBackToBackground(t *testing.T) {
	assert.NotNil(t, commandContext(&cobra.Command{Use: "bare"}))

	withCtx := &cobra.Command{Use: "withctx"}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	withCtx.SetContext(ctx)
	assert.Equal(t, ctx, commandContext(withCtx))
}

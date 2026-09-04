//nolint:testpackage // Testing internal functions requires same package
package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/sentiolabs/envctl/internal/secrets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	e2eChildEnv   = "ENVCTL_E2E_CHILD"
	e2eSecretsEnv = "ENVCTL_E2E_SECRETS"
	e2eSentinel   = "E2E-SENTINEL-PRIVATE-KEY-51c0"
	rawField      = "_raw"
	e2ePlainField = "PLAIN"
	e2eDevSecret  = "app/dev"
	e2eKubeSecret = "app/dev/kube"
)

// TestMain turns the test binary into envctl when ENVCTL_E2E_CHILD=1.
// The parent test execs os.Args[0] with envctl's own arguments, so the
// child runs the real Execute path, including os.Exit, in its own process.
func TestMain(m *testing.M) {
	if os.Getenv(e2eChildEnv) != "1" {
		os.Exit(m.Run())
	}
	client, err := loadFixtureClient(os.Getenv(e2eSecretsEnv))
	if err != nil {
		fmt.Fprintln(os.Stderr, "e2e child:", err) //nolint:revive // CLI output to stderr always succeeds
		os.Exit(3)
	}
	newSecretsClient = func(context.Context, secrets.Options) (secrets.Client, error) {
		return client, nil
	}
	Execute() // exits non-zero itself on error or child exit code
	os.Exit(0)
}

// fixtureClient serves secrets from a JSON file: secret name -> fields.
// A field named _raw is what GetSecretRaw returns and is hidden from GetSecret.
type fixtureClient struct {
	secrets map[string]map[string]string
}

var (
	_ secrets.Client    = (*fixtureClient)(nil)
	_ secrets.RawReader = (*fixtureClient)(nil)
)

func loadFixtureClient(path string) (*fixtureClient, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is a fixture file this test wrote
	if err != nil {
		return nil, err
	}
	var m map[string]map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &fixtureClient{secrets: m}, nil
}

func (f *fixtureClient) Name() string { return "fixture" }

func (f *fixtureClient) GetSecret(_ context.Context, ref string) (map[string]string, error) {
	fields, ok := f.secrets[ref]
	if !ok {
		return nil, fmt.Errorf("fixture: secret %q not found", ref)
	}
	out := make(map[string]string, len(fields))
	for k, v := range fields {
		if k != rawField {
			out[k] = v
		}
	}
	return out, nil
}

func (f *fixtureClient) GetSecretKey(ctx context.Context, ref, key string) (string, error) {
	fields, err := f.GetSecret(ctx, ref)
	if err != nil {
		return "", err
	}
	v, ok := fields[key]
	if !ok {
		return "", fmt.Errorf("fixture: key %q not in %q", key, ref)
	}
	return v, nil
}

func (f *fixtureClient) GetSecretRaw(_ context.Context, ref string) ([]byte, error) {
	fields, ok := f.secrets[ref]
	if !ok {
		return nil, fmt.Errorf("fixture: secret %q not found", ref)
	}
	raw, ok := fields[rawField]
	if !ok {
		return nil, fmt.Errorf("fixture: secret %q has no %s field", ref, rawField)
	}
	return []byte(raw), nil
}

// e2eFixture writes the secrets JSON and an .envctl.yaml into a temp dir and
// returns their paths. persistentPath, when non-empty, adds a second sink
// with file.path.
func e2eFixture(t *testing.T, persistentPath string) (configPath, secretsPath string) {
	t.Helper()
	dir := t.TempDir()
	secretsPath = filepath.Join(dir, "secrets.json")
	fixture := map[string]map[string]string{
		e2eDevSecret:     {e2ePlainField: "plain-value"},
		"app/dev/sp_key": {rawField: e2eSentinel + "\n"},
		e2eKubeSecret:    {rawField: "apiVersion: v1\n" + e2eSentinel + "\n"},
	}
	data, err := json.Marshal(fixture)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(secretsPath, data, 0o600))

	cfg := `version: 1
files_dir_as: FILES_DIR
environments:
  dev:
    - secret: app/dev
      key: PLAIN
    - secret: app/dev/sp_key
      file:
        name: sp.key
        path_as: KEY_FILE
`
	if persistentPath != "" {
		cfg += `    - secret: app/dev/kube
      file:
        path: ` + persistentPath + `
        path_as: KUBECONFIG
`
	}
	configPath = filepath.Join(dir, ".envctl.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(cfg), 0o600))
	return configPath, secretsPath
}

// envctlCmd builds an exec.Cmd that re-runs this test binary as envctl.
func envctlCmd(
	t *testing.T, secretsPath string, extraEnv []string, args ...string,
) (cmd *exec.Cmd, stdout, stderr *bytes.Buffer) {
	t.Helper()
	cmd = exec.Command(os.Args[0], args...) //nolint:gosec // re-exec of the test binary
	cmd.Env = append(os.Environ(), e2eChildEnv+"=1", e2eSecretsEnv+"="+secretsPath)
	cmd.Env = append(cmd.Env, extraEnv...)
	stdout, stderr = &bytes.Buffer{}, &bytes.Buffer{}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd, stdout, stderr
}

// runEnvctl runs envctl to completion and returns its streams and exit code.
func runEnvctl(
	t *testing.T, secretsPath string, extraEnv []string, args ...string,
) (stdout, stderr string, exitCode int) {
	t.Helper()
	cmd, stdoutBuf, stderrBuf := envctlCmd(t, secretsPath, extraEnv, args...)
	err := cmd.Run()
	if err != nil {
		var exitErr *exec.ExitError
		require.ErrorAs(t, err, &exitErr, "envctl must have exited, got: %v", err)
		exitCode = exitErr.ExitCode()
	}
	return stdoutBuf.String(), stderrBuf.String(), exitCode
}

func readMarker(t *testing.T, dir, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, name))
	require.NoError(t, err, name)
	return strings.TrimSpace(string(b))
}

// e2eChildScript records what the child process observed into OUT. The
// final "ready" marker uses a plain redirection: sh defers SIGINT while it
// waits on any foreground command, so a marker written by an earlier
// command is not proof the shell itself is ready to receive a signal.
func e2eChildScript(tail string) string {
	return `printf %s "$KEY_FILE" > "$OUT/path"; ` +
		`printf %s "$FILES_DIR" > "$OUT/dir"; ` +
		`cp "$KEY_FILE" "$OUT/copy"; ` +
		`ls -ld "$FILES_DIR" | cut -c1-10 > "$OUT/dirmode"; ` +
		`ls -l "$KEY_FILE" | cut -c1-10 > "$OUT/filemode"; ` +
		`: > "$OUT/ready"; ` + tail
}

func TestE2E_RunPropagatesExitCodeAndRemovesRunDir(t *testing.T) {
	configPath, secretsPath := e2eFixture(t, "")
	out := t.TempDir()

	stdout, stderr, code := runEnvctl(t, secretsPath, []string{"OUT=" + out},
		"-c", configPath, "run", "--", "sh", "-c", e2eChildScript("exit 7"))

	assert.Equal(t, 7, code, "child exit code must reach the shell; stderr: %s", stderr)
	assert.NotContains(t, stdout+stderr, e2eSentinel)

	path := readMarker(t, out, "path")
	dir := readMarker(t, out, "dir")
	assert.Equal(t, filepath.Join(dir, "sp.key"), path)
	copied, err := os.ReadFile(filepath.Join(out, "copy"))
	require.NoError(t, err)
	assert.Equal(t, e2eSentinel+"\n", string(copied))
	assert.Equal(t, "drwx------", readMarker(t, out, "dirmode"))
	assert.Equal(t, "-rw-------", readMarker(t, out, "filemode"))

	_, statErr := os.Stat(dir)
	assert.True(t, os.IsNotExist(statErr), "run dir must be gone after the child exits")
}

func TestE2E_RunSuccessExitsZero(t *testing.T) {
	configPath, secretsPath := e2eFixture(t, "")
	_, stderr, code := runEnvctl(t, secretsPath, nil,
		"-c", configPath, "run", "--", "sh", "-c", `test -s "$KEY_FILE" && test -n "$PLAIN"`)
	assert.Equal(t, 0, code, stderr)
}

func TestE2E_RunSIGINTFromOutsideRemovesRunDir(t *testing.T) {
	configPath, secretsPath := e2eFixture(t, "")
	out := t.TempDir()

	cmd, _, stderr := envctlCmd(t, secretsPath, []string{"OUT=" + out},
		"-c", configPath, "run", "--", "sh", "-c", e2eChildScript("exec sleep 30"))
	require.NoError(t, cmd.Start())

	// Wait for the "ready" marker, written after every foreground command in
	// the child script by a plain redirection. sh (the child before it execs
	// into sleep) defers a SIGINT that arrives while it is still waiting on
	// any foreground command, even one that already wrote its own output
	// file; once that command exits normally, the script simply continues to
	// `exec sleep 30` with the signal consumed. Waiting for "ready" closes
	// that window.
	require.Eventually(t, func() bool {
		_, err := os.Stat(filepath.Join(out, "ready"))
		return err == nil
	}, 15*time.Second, 20*time.Millisecond, "child never started; stderr: %s", stderr.String())

	// A real cross-process SIGINT, as a terminal would deliver it. The
	// resend loop below is defense in depth for any scheduling delay left
	// after closing the readiness race above: under heavy CPU contention
	// the OS can take a while to schedule envctl's signal-forwarding
	// goroutine, so retry on a short interval until envctl is observed to
	// exit.
	require.NoError(t, cmd.Process.Signal(syscall.SIGINT))

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	retry := time.NewTicker(500 * time.Millisecond)
	defer retry.Stop()
	timeout := time.After(20 * time.Second)
waitLoop:
	for {
		select {
		case err := <-done:
			require.Error(t, err, "envctl must exit non-zero when its child is interrupted")
			break waitLoop
		case <-retry.C:
			_ = cmd.Process.Signal(syscall.SIGINT)
		case <-timeout:
			_ = cmd.Process.Kill()
			t.Fatal("envctl did not exit after SIGINT")
		}
	}

	_, statErr := os.Stat(readMarker(t, out, "dir"))
	assert.True(t, os.IsNotExist(statErr), "run dir must be removed after SIGINT; stderr: %s", stderr.String())
}

func TestE2E_EnvSkipsEphemeralAndNeverPrintsContent(t *testing.T) {
	configPath, secretsPath := e2eFixture(t, "")
	stdout, stderr, code := runEnvctl(t, secretsPath, nil, "-c", configPath, "env")
	assert.Equal(t, 0, code, stderr)
	assert.Contains(t, stdout, "PLAIN=")
	assert.NotContains(t, stdout, "KEY_FILE")
	assert.NotContains(t, stdout, e2eSentinel)
	assert.Contains(t, stderr, "skipped 1 ephemeral file sink")
}

func TestE2E_ValidateReportsSinkSizeNotContent(t *testing.T) {
	configPath, secretsPath := e2eFixture(t, "")
	stdout, stderr, code := runEnvctl(t, secretsPath, nil, "-c", configPath, "validate")
	assert.Equal(t, 0, code, stderr)
	assert.Contains(t, stdout, "File sink KEY_FILE -> <run dir>/sp.key (0600, 30 bytes)")
	assert.NotContains(t, stdout+stderr, e2eSentinel)
}

func TestE2E_PersistentSinkWrittenByEnvThenCleaned(t *testing.T) {
	target := filepath.Join(t.TempDir(), "state", "kube")
	configPath, secretsPath := e2eFixture(t, target)

	stdout, stderr, code := runEnvctl(t, secretsPath, nil, "-c", configPath, "env")
	require.Equal(t, 0, code, stderr)
	assert.Contains(t, stdout, "KUBECONFIG="+target)
	assert.NotContains(t, stdout, e2eSentinel)

	info, err := os.Stat(target)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	parent, err := os.Stat(filepath.Dir(target))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), parent.Mode().Perm())

	stdout, stderr, code = runEnvctl(t, secretsPath, nil, "-c", configPath, "clean")
	require.Equal(t, 0, code, stderr)
	assert.Contains(t, stdout, "removed "+target)
	_, statErr := os.Stat(target)
	assert.True(t, os.IsNotExist(statErr))

	stdout, _, code = runEnvctl(t, secretsPath, nil, "-c", configPath, "clean")
	assert.Equal(t, 0, code)
	assert.Contains(t, stdout, "no persistent file sinks to remove")
}

func TestE2E_PersistentSinkRefusedInsideUnignoredRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	repo := t.TempDir()
	require.NoError(t, exec.Command("git", "-C", repo, "init", "-q").Run())
	target := filepath.Join(repo, "certs", "kube")
	configPath, secretsPath := e2eFixture(t, target)

	_, stderr, code := runEnvctl(t, secretsPath, nil, "-c", configPath, "env")
	assert.NotEqual(t, 0, code)
	assert.Contains(t, stderr, "not ignored")
	_, statErr := os.Stat(target)
	assert.True(t, os.IsNotExist(statErr))
}

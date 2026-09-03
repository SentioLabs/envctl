//nolint:testpackage // Testing internal functions requires same package
package cmd

import (
	"context"
	stderrors "errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/sentiolabs/envctl/internal/mocks"
	"github.com/sentiolabs/envctl/internal/runner"
	"github.com/sentiolabs/envctl/internal/secrets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// sentinel is the sink content. It must never appear on stdout.
const sentinel = "SENTINEL-PRIVATE-KEY-7f3a9c"

// testDevEnv is the environment name selected by every test config in this file.
const testDevEnv = "dev"

// envProgram is the coreutils env(1) program used to launch the child so it
// inherits its argv[0] cleanly and can be replaced by exec later in the script.
const envProgram = "env"

const ephemeralConfig = `version: 1
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

// persistentConfigTemplate needs the target path substituted for %s.
const persistentConfigTemplate = `version: 1
environments:
  dev:
    - secret: app/dev
      key: PLAIN
    - secret: app/dev/kube
      file:
        path: %s
        path_as: KUBECONFIG
`

// setupSinkConfig writes cfgYAML as .envctl.yaml in a temp dir, points the CLI
// at it, installs a mock backend that returns sentinel for every raw read, and
// restores all package globals on cleanup. It returns the config directory.
func setupSinkConfig(t *testing.T, cfgYAML string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, ".envctl.yaml")
	require.NoError(t, os.WriteFile(path, []byte(cfgYAML), 0o600))

	prevConfig, prevEnv, prevApp := configFile, envName, appName
	prevSet, prevOut, prevFormat := setFlags, outputFile, exportFormat
	prevQuiet, prevAll, prevGet, prevVerbose := listQuiet, cleanAll, getSecret, verbose
	prevFactory := newSecretsClient
	t.Cleanup(func() {
		configFile, envName, appName = prevConfig, prevEnv, prevApp
		setFlags, outputFile, exportFormat = prevSet, prevOut, prevFormat
		listQuiet, cleanAll, getSecret, verbose = prevQuiet, prevAll, prevGet, prevVerbose
		newSecretsClient = prevFactory
	})
	configFile, envName, appName = path, testDevEnv, ""
	setFlags, outputFile, exportFormat = nil, "", "shell"
	listQuiet, cleanAll, getSecret, verbose = false, false, "", false

	plainValues := map[string]string{"PLAIN": "plain-value"}
	client := mocks.NewMockRawClient(t)
	client.Client.On("Name").Return("mock").Maybe()
	client.Client.On("GetSecretKey", mock.Anything, "app/dev", "PLAIN").Return("plain-value", nil).Maybe()
	client.Client.On("GetSecret", mock.Anything, mock.Anything).Return(plainValues, nil).Maybe()
	client.Raw.On("GetSecretRaw", mock.Anything, mock.Anything).Return([]byte(sentinel), nil).Maybe()
	newSecretsClient = func(_ context.Context, _ secrets.Options) (secrets.Client, error) {
		return client, nil
	}
	return dir
}

// capture redirects *target (os.Stdout or os.Stderr) for the duration of fn.
func capture(t *testing.T, target **os.File, fn func() error) (string, error) {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	prev := *target
	*target = w
	var buf strings.Builder
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(&buf, r)
	}()
	fnErr := fn()
	*target = prev
	_ = w.Close()
	wg.Wait()
	_ = r.Close()
	return buf.String(), fnErr
}

func captureStdout(t *testing.T, fn func() error) (string, error) { return capture(t, &os.Stdout, fn) }

func captureStderr(t *testing.T, fn func() error) (string, error) { return capture(t, &os.Stderr, fn) }

// childScript is a portable shell body that records what the child observed.
// OUT is the directory the child writes into. The final "ready" marker uses
// a plain redirection: sh defers SIGINT while it waits on any foreground
// command, so a marker written by an earlier command is not proof the shell
// itself is ready to receive a signal.
func childScript(out, tail string) []string {
	body := `printf %s "$KEY_FILE" > "$OUT/path"; ` +
		`printf %s "$FILES_DIR" > "$OUT/dir"; ` +
		`cp "$KEY_FILE" "$OUT/copy"; ` +
		`ls -ld "$FILES_DIR" | cut -c1-10 > "$OUT/dirmode"; ` +
		`ls -l "$KEY_FILE" | cut -c1-10 > "$OUT/filemode"; ` +
		`: > "$OUT/ready"; ` + tail
	return []string{envProgram, "OUT=" + out, "sh", "-c", body}
}

func readOut(t *testing.T, out, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(out, name))
	require.NoError(t, err, name)
	return strings.TrimSpace(string(b))
}

func TestRun_EphemeralSinkLifecycle(t *testing.T) {
	setupSinkConfig(t, ephemeralConfig)
	out := t.TempDir()

	err := runRun(runCmd, childScript(out, "exit 0"))
	require.NoError(t, err)

	path := readOut(t, out, "path")
	dir := readOut(t, out, "dir")
	assert.Equal(t, filepath.Join(dir, "sp.key"), path)
	assert.True(t, strings.HasPrefix(dir, os.TempDir()), "run dir under os.TempDir()")
	assert.Equal(t, sentinel, readOut(t, out, "copy"), "child read the real content")
	assert.Equal(t, "drwx------", readOut(t, out, "dirmode"))
	assert.Equal(t, "-rw-------", readOut(t, out, "filemode"))

	_, statErr := os.Stat(dir)
	assert.True(t, os.IsNotExist(statErr), "run dir removed after exit 0")
}

func TestRun_NonZeroExitStillCleansUpAndKeepsCode(t *testing.T) {
	setupSinkConfig(t, ephemeralConfig)
	out := t.TempDir()

	err := runRun(runCmd, childScript(out, "exit 3"))
	require.Error(t, err)
	exitErr, ok := stderrors.AsType[*runner.ExitError](err)
	require.True(t, ok, "got %T", err)
	assert.Equal(t, 3, exitErr.Code)

	_, statErr := os.Stat(readOut(t, out, "dir"))
	assert.True(t, os.IsNotExist(statErr), "run dir removed after exit 3")
}

func TestRun_SIGINTCleansUp(t *testing.T) {
	setupSinkConfig(t, ephemeralConfig)
	out := t.TempDir()

	done := make(chan error, 1)
	go func() { done <- runRun(runCmd, childScript(out, "exec sleep 30")) }()

	// Wait for the "ready" marker, written after every foreground command in
	// the child script by a plain redirection, before interrupting
	// ourselves. sh (the child before it execs into sleep) defers a SIGINT
	// that arrives while it is still waiting on any foreground command, even
	// one that already wrote its own output file; once that command exits
	// normally, the script simply continues to `exec sleep 30` with the
	// signal consumed. Waiting for "ready" closes that window. The runner
	// has signal.Notify installed, so the process does not die on SIGINT; it
	// forwards SIGINT to the child (now `sleep`, thanks to exec), Wait
	// returns, and runRun's deferred cleanup runs.
	require.Eventually(t, func() bool {
		_, err := os.Stat(filepath.Join(out, "ready"))
		return err == nil
	}, 10*time.Second, 20*time.Millisecond)

	// The resend loop below is defense in depth for any scheduling delay
	// still left after closing the readiness race: under heavy CPU
	// contention the OS can take a while to schedule the runtime's
	// signal-handling goroutine. Resending is safe because the child only
	// needs to observe one SIGINT to exit, so retry on a short interval
	// until the run completes.
	retry := time.NewTicker(500 * time.Millisecond)
	defer retry.Stop()
	require.NoError(t, syscall.Kill(os.Getpid(), syscall.SIGINT))
	timeout := time.After(20 * time.Second)
waitLoop:
	for {
		select {
		case err := <-done:
			require.Error(t, err, "sleep killed by SIGINT exits non-zero")
			break waitLoop
		case <-retry.C:
			require.NoError(t, syscall.Kill(os.Getpid(), syscall.SIGINT))
		case <-timeout:
			t.Fatal("runRun did not return after SIGINT")
		}
	}

	_, statErr := os.Stat(readOut(t, out, "dir"))
	assert.True(t, os.IsNotExist(statErr), "run dir removed after SIGINT")
}

func TestRun_SetOverrideBeatsPathVar(t *testing.T) {
	setupSinkConfig(t, ephemeralConfig)
	out := t.TempDir()
	setFlags = []string{"KEY_FILE=/custom/override"}

	err := runRun(runCmd, []string{envProgram, "OUT=" + out, "sh", "-c", `printf %s "$KEY_FILE" > "$OUT/path"`})
	require.NoError(t, err)
	assert.Equal(t, "/custom/override", readOut(t, out, "path"))
}

func persistentConfig(t *testing.T) (cfgYAML, target string) {
	t.Helper()
	target = filepath.Join(t.TempDir(), "state", "kube")
	return strings.Replace(persistentConfigTemplate, "%s", target, 1), target
}

func TestStdoutNeverContainsSinkContent(t *testing.T) {
	persistentYAML, _ := persistentConfig(t)
	configs := map[string]string{"ephemeral": ephemeralConfig, "persistent": persistentYAML}

	commands := map[string]func() error{
		"env":          func() error { return runEnv(envCmd, nil) },
		"export shell": func() error { exportFormat = "shell"; return runExport(exportCmd, nil) },
		"export env":   func() error { exportFormat = "env"; return runExport(exportCmd, nil) },
		"export json":  func() error { exportFormat = "json"; return runExport(exportCmd, nil) },
		"get plain":    func() error { return runGet(getCmd, []string{"PLAIN"}) },
		"list":         func() error { return runList(listCmd, nil) },
		"list quiet":   func() error { listQuiet = true; return runList(listCmd, nil) },
		"validate":     func() error { return runValidate(validateCmd, nil) },
	}

	for cfgName, cfgYAML := range configs {
		for cmdName, fn := range commands {
			t.Run(cfgName+"/"+cmdName, func(t *testing.T) {
				setupSinkConfig(t, cfgYAML)
				var stdout string
				stderr, _ := captureStderr(t, func() error {
					var err error
					stdout, err = captureStdout(t, fn)
					return err
				})
				assert.NotContains(t, stdout, sentinel, "stdout leaked sink content")
				assert.NotContains(t, stderr, sentinel, "stderr leaked sink content")
			})
		}
	}
}

func TestEnv_EphemeralSkippedWithWarning(t *testing.T) {
	setupSinkConfig(t, ephemeralConfig)
	var stdout string
	stderr, err := captureStderr(t, func() error {
		var e error
		stdout, e = captureStdout(t, func() error { return runEnv(envCmd, nil) })
		return e
	})
	require.NoError(t, err)
	assert.Contains(t, stdout, "PLAIN=")
	assert.NotContains(t, stdout, "KEY_FILE", "ephemeral path var must not be exported")
	assert.Contains(t, stderr, "skipped 1 ephemeral file sink")
	assert.Contains(t, stderr, "KEY_FILE")
}

func TestList_ShowsSinkVariablesWithoutFetching(t *testing.T) {
	setupSinkConfig(t, ephemeralConfig)
	stdout, err := captureStdout(t, func() error { return runList(listCmd, nil) })
	require.NoError(t, err)
	assert.Contains(t, stdout, "KEY_FILE")
	assert.Contains(t, stdout, "file:app/dev/sp_key")
	assert.Contains(t, stdout, "FILES_DIR")
}

func TestValidate_ReportsSinkWithSizeNotContent(t *testing.T) {
	setupSinkConfig(t, ephemeralConfig)
	stdout, err := captureStdout(t, func() error { return runValidate(validateCmd, nil) })
	require.NoError(t, err)
	assert.Contains(t, stdout, "File sink KEY_FILE -> <run dir>/sp.key (0600, 27 bytes)")
	assert.Contains(t, stdout, "Files dir: FILES_DIR")
}

func TestPersistent_EnvWritesThenCleanRemoves(t *testing.T) {
	cfgYAML, target := persistentConfig(t)
	setupSinkConfig(t, cfgYAML)

	stdout, err := captureStdout(t, func() error { return runEnv(envCmd, nil) })
	require.NoError(t, err)
	assert.Contains(t, stdout, "KUBECONFIG="+target)

	got, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, sentinel, string(got))
	info, err := os.Stat(target)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	parent, err := os.Stat(filepath.Dir(target))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), parent.Mode().Perm())

	stdout, err = captureStdout(t, func() error { return runClean(cleanCmd, nil) })
	require.NoError(t, err)
	assert.Contains(t, stdout, "removed "+target)
	_, statErr := os.Stat(target)
	assert.True(t, os.IsNotExist(statErr))

	stdout, err = captureStdout(t, func() error { return runClean(cleanCmd, nil) })
	require.NoError(t, err)
	assert.Contains(t, stdout, "no persistent file sinks to remove")
}

func TestClean_AllSweepsEveryEnvironment(t *testing.T) {
	base := t.TempDir()
	devTarget := filepath.Join(base, testDevEnv, "kube")
	stagingTarget := filepath.Join(base, "staging", "kube")
	cfgYAML := `version: 1
default_environment: dev
environments:
  dev:
    - secret: app/dev
      key: PLAIN
    - secret: app/dev/kube
      file: { path: ` + devTarget + `, path_as: KUBECONFIG }
  staging:
    - secret: app/staging
      key: PLAIN
    - secret: app/staging/kube
      file: { path: ` + stagingTarget + `, path_as: KUBECONFIG }
`
	setupSinkConfig(t, cfgYAML)
	require.NoError(t, os.MkdirAll(filepath.Dir(devTarget), 0o700))
	require.NoError(t, os.MkdirAll(filepath.Dir(stagingTarget), 0o700))
	require.NoError(t, os.WriteFile(devTarget, []byte("x"), 0o600))
	require.NoError(t, os.WriteFile(stagingTarget, []byte("x"), 0o600))

	// Default scope: only the selected environment (dev).
	stdout, err := captureStdout(t, func() error { return runClean(cleanCmd, nil) })
	require.NoError(t, err)
	assert.Contains(t, stdout, "removed "+devTarget)
	assert.NotContains(t, stdout, stagingTarget)
	_, err = os.Stat(stagingTarget)
	require.NoError(t, err, "staging sink must survive a default clean")

	// --all sweeps the rest.
	cleanAll = true
	stdout, err = captureStdout(t, func() error { return runClean(cleanCmd, nil) })
	require.NoError(t, err)
	assert.Contains(t, stdout, "removed "+stagingTarget)
	_, statErr := os.Stat(stagingTarget)
	assert.True(t, os.IsNotExist(statErr))
}

func TestPersistent_GetPrintsPath(t *testing.T) {
	cfgYAML, target := persistentConfig(t)
	setupSinkConfig(t, cfgYAML)

	stdout, err := captureStdout(t, func() error { return runGet(getCmd, []string{"KUBECONFIG"}) })
	require.NoError(t, err)
	assert.Equal(t, target, strings.TrimSpace(stdout))
	_, statErr := os.Stat(target)
	require.NoError(t, statErr, "get writes the persistent sink like env does")
}

func TestPersistent_RefusedInsideUnignoredRepo(t *testing.T) {
	repo := t.TempDir()
	if err := execGitInit(repo); err != nil {
		t.Skip("git unavailable")
	}
	target := filepath.Join(repo, "certs", "kube")
	cfgYAML := strings.Replace(persistentConfigTemplate, "%s", target, 1)
	setupSinkConfig(t, cfgYAML)

	_, err := captureStdout(t, func() error { return runEnv(envCmd, nil) })
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not ignored")
	_, statErr := os.Stat(target)
	assert.True(t, os.IsNotExist(statErr))
}

func execGitInit(dir string) error {
	cmd := exec.Command("git", "-C", dir, "init", "-q")
	return cmd.Run()
}

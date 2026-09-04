//nolint:testpackage // Testing internal functions requires same package
package runner

import (
	"errors"
	"fmt"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunNoCommand(t *testing.T) {
	r := NewRunner(map[string]string{})
	err := r.Run(t.Context(), []string{})
	require.Error(t, err)
	assert.Equal(t, "no command specified", err.Error())
}

func TestRunInvalidCommand(t *testing.T) {
	r := NewRunner(map[string]string{"FOO": "bar"})
	err := r.Run(t.Context(), []string{"/nonexistent-command-abc123"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to start command")
}

func TestBuildEnvIncludesSecrets(t *testing.T) {
	r := NewRunner(map[string]string{
		"SECRET_A": "value_a",
		"SECRET_B": "value_b",
	})

	env := r.buildEnv()

	found := make(map[string]bool)
	for _, e := range env {
		if e == "SECRET_A=value_a" {
			found["SECRET_A"] = true
		}
		if e == "SECRET_B=value_b" {
			found["SECRET_B"] = true
		}
	}

	assert.True(t, found["SECRET_A"], "SECRET_A should be in environment")
	assert.True(t, found["SECRET_B"], "SECRET_B should be in environment")
}

func TestBuildEnvNoInherit(t *testing.T) {
	r := NewRunner(map[string]string{
		"ONLY_VAR": "only_value",
	})
	r.WithInheritEnv(false)

	env := r.buildEnv()

	assert.Len(t, env, 1)
	assert.Equal(t, "ONLY_VAR=only_value", env[0])
}

func TestErrorsAsTypeExitError(t *testing.T) {
	// Verify that errors.AsType[*exec.ExitError] works the same as errors.As
	// for extracting exit errors. This validates the pattern used in Run().
	baseErr := &exec.ExitError{}
	wrappedErr := fmt.Errorf("command failed: %w", baseErr)

	exitErr, ok := errors.AsType[*exec.ExitError](wrappedErr)
	assert.True(t, ok, "errors.AsType should find *exec.ExitError")
	assert.NotNil(t, exitErr, "extracted error should not be nil")
}

func TestExitErrorDetectionUsesAsType(t *testing.T) {
	// Verifies the errors.AsType[*exec.ExitError] pattern that replaced
	// the removed isExitError helper function.
	baseErr := &exec.ExitError{}
	wrapped := fmt.Errorf("wrapped: %w", baseErr)

	exitErr, ok := errors.AsType[*exec.ExitError](wrapped)
	require.True(t, ok)
	assert.Equal(t, baseErr, exitErr)

	// Non-exit error should not match
	otherErr := errors.New("some other error")
	_, ok = errors.AsType[*exec.ExitError](otherErr)
	assert.False(t, ok)
}

// --- Contract assertions ---
// These verify the file-sink design contracts. Do NOT modify
// without updating the approved plan.

var _ error = (*ExitError)(nil)

func TestExitErrorMessage(t *testing.T) {
	err := &ExitError{Code: 3}
	assert.Equal(t, "command exited with code 3", err.Error())
}

func TestRunReturnsExitErrorInsteadOfExiting(t *testing.T) {
	r := NewRunner(map[string]string{})
	err := r.Run(t.Context(), []string{"sh", "-c", "exit 7"})
	require.Error(t, err)

	exitErr, ok := errors.AsType[*ExitError](err)
	require.True(t, ok, "expected *ExitError, got %T", err)
	assert.Equal(t, 7, exitErr.Code)
}

func TestRunSuccessReturnsNil(t *testing.T) {
	r := NewRunner(map[string]string{"RUNNER_TEST_VAR": "ok"})
	err := r.Run(t.Context(), []string{"sh", "-c", `test "$RUNNER_TEST_VAR" = ok`})
	require.NoError(t, err)
}

func TestRunDeferredCleanupObservable(t *testing.T) {
	// Before this change, a non-zero child called os.Exit inside Run and no
	// code after Run could execute. This test only passes if Run returns.
	cleaned := false
	func() {
		defer func() { cleaned = true }()
		r := NewRunner(map[string]string{})
		_ = r.Run(t.Context(), []string{"sh", "-c", "exit 1"})
	}()
	assert.True(t, cleaned)
}

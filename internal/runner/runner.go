// Package runner handles subprocess execution with environment injection.
package runner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

// Runner executes commands with injected environment variables.
type Runner struct {
	env          map[string]string
	inheritEnv   bool
	inheritPaths bool
}

// NewRunner creates a new runner.
func NewRunner(env map[string]string) *Runner {
	return &Runner{
		env:          env,
		inheritEnv:   true,
		inheritPaths: true,
	}
}

// WithInheritEnv sets whether to inherit the parent environment.
func (r *Runner) WithInheritEnv(inherit bool) *Runner {
	r.inheritEnv = inherit
	return r
}

// Run executes the command with the configured environment.
func (r *Runner) Run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no command specified")
	}

	// Build the command - use exec directly, no shell
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)

	// Set up I/O
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Build environment
	cmd.Env = r.buildEnv()

	// Set up signal forwarding
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	// Start the command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	// Forward signals to child process
	go func() {
		for sig := range sigChan {
			if cmd.Process != nil {
				cmd.Process.Signal(sig)
			}
		}
	}()

	// Wait for command to complete
	err := cmd.Wait()

	// Stop signal forwarding
	signal.Stop(sigChan)
	close(sigChan)

	// If the command exited with a non-zero exit code, exit with that code
	if err != nil {
		var exitErr *exec.ExitError
		if ok := isExitError(err, &exitErr); ok {
			os.Exit(exitErr.ExitCode())
		}
		return err
	}

	return nil
}

// buildEnv builds the environment variable list.
func (r *Runner) buildEnv() []string {
	var env []string

	// Start with parent environment if inheriting
	if r.inheritEnv {
		env = os.Environ()
	}

	// Add our secrets (these override parent env)
	for key, value := range r.env {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}

	return env
}

// isExitError checks if an error is an exec.ExitError and extracts it.
func isExitError(err error, target **exec.ExitError) bool {
	if exitErr, ok := err.(*exec.ExitError); ok {
		*target = exitErr
		return true
	}
	return false
}

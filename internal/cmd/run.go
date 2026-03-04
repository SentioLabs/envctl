// Package cmd implements the CLI commands for envctl.
// This file contains the run command for executing processes with injected secrets.
package cmd

import (
	"context"
	"strings"

	"github.com/sentiolabs/envctl/internal/env"
	"github.com/sentiolabs/envctl/internal/runner"
	"github.com/spf13/cobra"
)

// keyValueParts is the expected number of parts when splitting KEY=VALUE strings.
const keyValueParts = 2

var (
	setFlags []string

	runCmd = &cobra.Command{
		Use:   "run [flags] -- command [args...]",
		Short: "Run a command with secrets injected as environment variables",
		Long: `Run executes a command with secrets from your configured backend injected
into its environment. This is the primary workflow for local development.

Secrets are fetched from your backend, merged with the current environment
(secrets take precedence), and the command is executed with the merged environment.

Example:
  envctl run -- go run ./cmd/server
  envctl run -- npm start
  envctl run -e staging -- make dev
  envctl run --set DEBUG=true -- ./app`,
		Args:               cobra.MinimumNArgs(1),
		RunE:               runRun,
		DisableFlagParsing: false,
	}
)

// init registers the run command with the root command.
func init() {
	runCmd.Flags().StringArrayVar(&setFlags, "set", nil, "Set or override an environment variable (KEY=VALUE)")
	rootCmd.AddCommand(runCmd)
}

// runRun executes a command with secrets injected into its environment.
func runRun(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	overrides := parseOverrides(setFlags)

	entries, _, err := loadAndBuild(ctx, cmd, overrides)
	if err != nil {
		return err
	}

	// Run the command
	envMap := env.ToMap(entries)
	r := runner.NewRunner(envMap)
	return r.Run(ctx, args)
}

// parseOverrides parses KEY=VALUE strings into a map.
func parseOverrides(flags []string) map[string]string {
	overrides := make(map[string]string)
	for _, flag := range flags {
		parts := strings.SplitN(flag, "=", keyValueParts)
		if len(parts) == keyValueParts {
			overrides[parts[0]] = parts[1]
		}
	}
	return overrides
}

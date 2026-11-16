// Package cmd implements the CLI commands for envctl.
// This file contains the export command for outputting secrets in various formats.
package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"

	"github.com/sentiolabs/envctl/internal/env"
	"github.com/sentiolabs/envctl/internal/output"
	"github.com/sentiolabs/envctl/internal/secrets"
	"github.com/spf13/cobra"
)

// Command flags and definition for export command.
var (
	exportFormat string
	exportApps   []string

	// exportCmd outputs secrets in env, shell, or json format.
	exportCmd = &cobra.Command{
		Use:   "export",
		Short: "Output secrets in various formats for shell integration",
		Long: `Output secrets in formats suitable for shell eval or other tools.

Supported formats:
  - env:   KEY=VALUE (default)
  - shell: export KEY="VALUE" (for eval)
  - json:  {"KEY": "VALUE"}

Example:
  eval "$(envctl export)"
  eval "$(envctl export --format shell)"
  envctl export --format json | jq .

--apps exports several applications in one run as {"app": {"KEY": "VALUE"}}.
It needs --format json. A secret that two applications include is read
from the backend once, so a 1Password-backed config authorizes once:
  envctl export --apps api,web --env local --format json

Sources with file.path are written and their path_as variables included.
Sources with file.name are skipped: only envctl run materializes them.`,
		RunE: runExport,
	}
)

// init registers the export command with the root command.
func init() {
	exportCmd.Flags().StringVar(&exportFormat, "format", "shell", "output format (env, shell, json)")
	exportCmd.Flags().StringSliceVar(&exportApps, "apps", nil,
		"export these applications as one JSON object keyed by name (needs --format json)")
	_ = exportCmd.RegisterFlagCompletionFunc("apps", completeApplicationNames)
	rootCmd.AddCommand(exportCmd)
}

// runExport outputs secrets in the requested format.
func runExport(cmd *cobra.Command, args []string) error {
	ctx := commandContext(cmd)

	format, err := output.ParseFormat(exportFormat)
	if err != nil {
		return err
	}

	if len(exportApps) > 0 {
		return runExportApps(ctx, cmd, format)
	}

	entries, _, err := loadAndBuild(ctx, cmd, nil)
	if err != nil {
		return err
	}

	return output.Write(os.Stdout, entries, format)
}

// runExportApps builds each --apps application with one shared memo and
// writes them as a single JSON object. Any application's failure fails the
// whole export, so a caller never gets a partial object.
func runExportApps(ctx context.Context, cmd *cobra.Command, format output.Format) error {
	if format != output.FormatJSON {
		return fmt.Errorf("--apps needs --format json (got %s)", format)
	}
	if appName != "" {
		return errors.New("--apps and --app can't be combined")
	}
	// An empty name would resolve to the default (or a random) application
	// and land in the output under the key "".
	if slices.Contains(exportApps, "") {
		return errors.New("--apps has an empty application name")
	}

	configPath, cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if !cfg.HasApplications() {
		return errors.New("--apps needs a config with applications")
	}

	memo := secrets.NewMemo()
	apps := make(map[string][]env.Entry, len(exportApps))
	for _, application := range slices.Compact(slices.Sorted(slices.Values(exportApps))) {
		out, err := buildApp(ctx, cmd, appBuild{
			cfg: cfg, configPath: configPath, application: application,
			mode: sinkModePersistentOnly, memo: memo,
		})
		if err != nil {
			return fmt.Errorf("application %s: %w", application, err)
		}
		apps[application] = out.entries
	}

	return output.WriteJSONApps(os.Stdout, apps)
}

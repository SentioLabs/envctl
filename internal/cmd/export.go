// Package cmd implements the CLI commands for envctl.
// This file contains the export command for outputting secrets in various formats.
package cmd

import (
	"context"
	"os"

	"github.com/sentiolabs/envctl/internal/output"
	"github.com/spf13/cobra"
)

// Command flags and definition for export command.
var (
	exportFormat string

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
  envctl export --format json | jq .`,
		RunE: runExport,
	}
)

// init registers the export command with the root command.
func init() {
	exportCmd.Flags().StringVar(&exportFormat, "format", "shell", "output format (env, shell, json)")
	rootCmd.AddCommand(exportCmd)
}

// runExport outputs secrets in the requested format.
func runExport(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	format, err := output.ParseFormat(exportFormat)
	if err != nil {
		return err
	}

	entries, _, err := loadAndBuild(ctx, cmd, nil)
	if err != nil {
		return err
	}

	return output.Write(os.Stdout, entries, format)
}

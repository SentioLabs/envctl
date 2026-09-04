// Package cmd implements the CLI commands for envctl.
// This file contains the clean command for removing persistent file sinks.
package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/sentiolabs/envctl/internal/config"
	"github.com/sentiolabs/envctl/internal/filesink"
	"github.com/spf13/cobra"
)

var (
	cleanAll bool

	cleanCmd = &cobra.Command{
		Use:   "clean",
		Short: "Remove persistent file sinks",
		Long: `Remove files written by sources that declare file.path.

Ephemeral sinks (file.name) live in a per-run directory that envctl run
removes on exit, so clean only concerns persistent ones. By default the
selected application and environment are cleaned. Use --all to sweep every
application and environment in the config.

Example:
  envctl clean
  envctl clean -e staging
  envctl clean --all`,
		RunE: runClean,
	}
)

// init registers the clean command with the root command.
func init() {
	cleanCmd.Flags().BoolVar(
		&cleanAll, "all", false, "remove persistent file sinks for every application and environment",
	)
	rootCmd.AddCommand(cleanCmd)
}

// runClean removes persistent sink files and prints each removed path.
//
//nolint:revive // CLI output to stdout always succeeds
func runClean(cmd *cobra.Command, args []string) error {
	configPath, err := resolveConfigPath()
	if err != nil {
		return err
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	configDir := filepath.Dir(configPath)

	// --all sweeps the whole config. Otherwise only the selected app/env.
	var envs []*config.Environment
	if cleanAll {
		envs = allEnvironments(cfg)
	} else {
		envConfig, _, err := resolveEnvironmentConfig(cfg)
		if err != nil {
			return err
		}
		envs = []*config.Environment{envConfig}
	}

	removed := 0
	for _, e := range envs {
		for _, src := range e.Sources {
			// Ephemeral sinks belong to a run directory that envctl run already removed.
			if src.File == nil || !src.File.Persistent() {
				continue
			}
			abs, err := filesink.Expand(src.File.Path, configDir)
			if err != nil {
				return err
			}
			// A sink that was never written is not an error.
			switch err := os.Remove(abs); {
			case err == nil:
				fmt.Fprintf(os.Stdout, "removed %s\n", abs)
				removed++
			case errors.Is(err, fs.ErrNotExist):
				verboseLog("not present: %s", abs)
			default:
				return err
			}
		}
	}

	// Say so explicitly rather than exiting silently on a no-op.
	if removed == 0 {
		fmt.Fprintln(os.Stdout, "no persistent file sinks to remove")
	}
	return nil
}

// allEnvironments returns every environment across legacy and application modes.
func allEnvironments(cfg *config.Config) []*config.Environment {
	var envs []*config.Environment
	for name := range cfg.Environments {
		e := cfg.Environments[name]
		envs = append(envs, &e)
	}
	for _, app := range cfg.Applications {
		for name := range app.Environments {
			e := app.Environments[name]
			envs = append(envs, &e)
		}
	}
	return envs
}

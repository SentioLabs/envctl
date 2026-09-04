package cmd

import (
	"context"
	"os"

	"github.com/sentiolabs/envctl/internal/config"
	"github.com/sentiolabs/envctl/internal/env"
	"github.com/sentiolabs/envctl/internal/output"
	"github.com/spf13/cobra"
)

var (
	listQuiet bool

	listCmd = &cobra.Command{
		Use:   "list",
		Short: "List available secret keys",
		Long: `List the keys that would be injected as environment variables.

This shows key names and their sources, but never shows secret values.

Example:
  envctl list
  envctl list -e staging
  envctl list --quiet`,
		RunE: runList,
	}
)

func init() {
	listCmd.Flags().BoolVarP(&listQuiet, "quiet", "q", false, "show only key names")
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Load config
	configPath, err := resolveConfigPath()
	if err != nil {
		return err
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	// Resolve environment config
	envConfig, app, err := resolveEnvironmentConfig(cfg)
	if err != nil {
		return err
	}

	// Create secrets client with caching
	client, err := createSecretsClient(ctx, cfg, envConfig)
	if err != nil {
		return err
	}

	// Build environment
	builder := env.NewBuilder(client, cfg, appName, envName).
		WithIncludeAll(getIncludeAllOverride(cmd))
	entries, err := builder.Build(ctx, nil)
	if err != nil {
		return err
	}

	// File sinks are listed from config alone; their content is never fetched here.
	entries = append(entries, fileSinkListEntries(cfg, app, envConfig)...)

	// Write list
	return output.WriteList(os.Stdout, entries, listQuiet)
}

// fileSinkListEntries returns one entry per file sink (path_as with a
// file:<secret> source) plus the files_dir_as variable when any sink is
// ephemeral. Values are left empty: list prints names and sources only.
func fileSinkListEntries(cfg *config.Config, app *config.Application, envConfig *config.Environment) []env.Entry {
	var entries []env.Entry
	ephemeral := false
	for _, src := range envConfig.Sources {
		if src.File == nil {
			continue
		}
		if !src.File.Persistent() {
			ephemeral = true
		}
		entries = append(entries, env.Entry{Key: src.File.PathAs, Source: fileSourcePrefix + src.Secret})
	}
	if dirAs := cfg.ResolveFilesDirAs(app); dirAs != "" && ephemeral {
		entries = append(entries, env.Entry{Key: dirAs, Source: "file"})
	}
	return entries
}

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
	configPath := configFile
	if configPath == "" {
		var err error
		configPath, err = config.FindConfig()
		if err != nil {
			return err
		}
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	// Resolve environment config
	envConfig, _, err := resolveEnvironmentConfig(cfg)
	if err != nil {
		return err
	}

	// Create AWS client with caching
	client, err := createSecretsClient(ctx, cfg, envConfig.Region, envConfig.Profile)
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

	// Write list
	return output.WriteList(os.Stdout, entries, listQuiet)
}

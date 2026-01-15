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
	exportFormat string

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

func init() {
	exportCmd.Flags().StringVar(&exportFormat, "format", "shell", "output format (env, shell, json)")
	rootCmd.AddCommand(exportCmd)
}

func runExport(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Parse format
	format, err := output.ParseFormat(exportFormat)
	if err != nil {
		return err
	}

	// Load config
	configPath := configFile
	if configPath == "" {
		var err error
		configPath, err = config.FindConfig()
		if err != nil {
			return err
		}
	}
	verboseLog("Using config: %s", configPath)

	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	// Resolve environment config
	envConfig, _, err := resolveEnvironmentConfig(cfg)
	if err != nil {
		return err
	}
	verboseLog("Using environment: %s (secret: %s)", envName, envConfig.Secret)

	// Create AWS client with caching
	client, err := createSecretsClient(ctx, cfg, envConfig.Region, envConfig.Profile)
	if err != nil {
		return err
	}

	// Build environment
	builder := env.NewBuilder(client, cfg, appName, envName)
	entries, err := builder.Build(ctx, nil)
	if err != nil {
		return err
	}
	verboseLog("Loaded %d environment variables", len(entries))

	// Write output
	return output.Write(os.Stdout, entries, format)
}

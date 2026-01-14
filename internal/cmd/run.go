package cmd

import (
	"context"
	"strings"

	"github.com/sentiolabs/envctl/internal/aws"
	"github.com/sentiolabs/envctl/internal/config"
	"github.com/sentiolabs/envctl/internal/env"
	"github.com/sentiolabs/envctl/internal/runner"
	"github.com/spf13/cobra"
)

var (
	setFlags []string

	runCmd = &cobra.Command{
		Use:   "run [flags] -- command [args...]",
		Short: "Run a command with secrets injected as environment variables",
		Long: `Run executes a command with secrets from AWS Secrets Manager injected
into its environment. This is the primary workflow for local development.

Secrets are fetched from AWS, merged with the current environment (secrets
take precedence), and the command is executed with the merged environment.

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

func init() {
	runCmd.Flags().StringArrayVar(&setFlags, "set", nil, "Set or override an environment variable (KEY=VALUE)")
	rootCmd.AddCommand(runCmd)
}

func runRun(cmd *cobra.Command, args []string) error {
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
	verboseLog("Using config: %s", configPath)

	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	// Get environment config
	envConfig, err := cfg.GetEnvironment(envName)
	if err != nil {
		return err
	}
	verboseLog("Using environment: %s (secret: %s)", envName, envConfig.Secret)

	// Create AWS client
	client, err := aws.NewSecretsClient(ctx, envConfig.Region)
	if err != nil {
		return err
	}

	// Parse overrides from --set flags
	overrides := parseOverrides(setFlags)

	// Build environment
	builder := env.NewBuilder(client, cfg, envName)
	entries, err := builder.Build(ctx, overrides)
	if err != nil {
		return err
	}
	verboseLog("Loaded %d environment variables", len(entries))

	// Run the command
	envMap := env.ToMap(entries)
	r := runner.NewRunner(envMap)
	return r.Run(ctx, args)
}

// parseOverrides parses KEY=VALUE strings into a map.
func parseOverrides(flags []string) map[string]string {
	overrides := make(map[string]string)
	for _, flag := range flags {
		parts := strings.SplitN(flag, "=", 2)
		if len(parts) == 2 {
			overrides[parts[0]] = parts[1]
		}
	}
	return overrides
}

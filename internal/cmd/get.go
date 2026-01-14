package cmd

import (
	"context"
	"fmt"

	"github.com/sentiolabs/envctl/internal/aws"
	"github.com/sentiolabs/envctl/internal/cache"
	"github.com/sentiolabs/envctl/internal/config"
	"github.com/sentiolabs/envctl/internal/env"
	"github.com/spf13/cobra"
)

var (
	getSecret string

	getCmd = &cobra.Command{
		Use:   "get KEY",
		Short: "Get a single secret value",
		Long: `Retrieve a single secret value by key name.

This is useful for scripts that need a specific secret value.

Example:
  envctl get DATABASE_URL
  envctl get -e staging API_KEY
  psql "$(envctl get DATABASE_URL)"

  # Get from specific secret (bypass config)
  envctl get --secret myapp/prod#DATABASE_URL`,
		Args: cobra.ExactArgs(1),
		RunE: runGet,
	}
)

func init() {
	getCmd.Flags().StringVar(&getSecret, "secret", "", "get from specific secret (format: secret_name#key)")
	rootCmd.AddCommand(getCmd)
}

func runGet(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	key := args[0]

	// If --secret flag is provided, bypass config
	if getSecret != "" {
		return getFromSecret(ctx, getSecret)
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

	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	// Get environment config
	envConfig, err := cfg.GetEnvironment(envName)
	if err != nil {
		return err
	}

	// Create AWS client with caching
	client, err := createSecretsClient(ctx, cfg, envConfig.Region)
	if err != nil {
		return err
	}

	// Build environment
	builder := env.NewBuilder(client, cfg, envName)
	entries, err := builder.Build(ctx, nil)
	if err != nil {
		return err
	}

	// Find the key
	for _, e := range entries {
		if e.Key == key {
			fmt.Println(e.Value)
			return nil
		}
	}

	return fmt.Errorf("key not found: %s", key)
}

func getFromSecret(ctx context.Context, ref string) error {
	secretRef, err := config.ParseSecretRef(ref)
	if err != nil {
		return err
	}

	if secretRef.KeyName == "" {
		return fmt.Errorf("key name is required (format: secret_name#key)")
	}

	// Set up cache for direct secret access
	var cacheManager *cache.Manager
	if !noCache {
		cacheOpts := cache.DefaultOptions()
		cacheManager, err = cache.NewManager(cacheOpts)
		if err != nil {
			verboseLog("Cache initialization failed: %v", err)
		}
	}

	// Create AWS client with caching
	client, err := aws.NewSecretsClientWithOptions(ctx, aws.ClientOptions{
		Region:  "",
		Cache:   cacheManager,
		NoCache: noCache,
		Refresh: refresh,
	})
	if err != nil {
		return err
	}

	value, err := client.GetSecretKey(ctx, secretRef.SecretName, secretRef.KeyName)
	if err != nil {
		return err
	}

	fmt.Println(value)
	return nil
}

// Package cmd implements the CLI commands for envctl.
// This file contains the get command for retrieving a single secret value.
package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/sentiolabs/envctl/internal/aws"
	"github.com/sentiolabs/envctl/internal/cache"
	"github.com/sentiolabs/envctl/internal/config"
	"github.com/sentiolabs/envctl/internal/env"
	"github.com/spf13/cobra"
)

// errKeyNameRequired is returned when a key name is not provided in the secret reference.
var errKeyNameRequired = errors.New("key name is required (format: secret_name#key)")

// Command flags for get command.
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

// init registers the get command with the root command.
func init() {
	getCmd.Flags().StringVar(&getSecret, "secret", "", "get from specific secret (format: secret_name#key)")
	rootCmd.AddCommand(getCmd)
}

// runGet retrieves a single secret value by key name from the configured secrets.
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

	// Resolve environment config
	envConfig, _, err := resolveEnvironmentConfig(cfg)
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

	// Find the key in the built environment entries
	for _, e := range entries {
		if e.Key == key {
			fmt.Println(e.Value) //nolint:revive,forbidigo // CLI output to stdout
			return nil
		}
	}

	return fmt.Errorf("key not found: %s", key)
}

// getFromSecret retrieves a secret value directly from AWS using the secret#key syntax.
func getFromSecret(ctx context.Context, ref string) error {
	secretRef, err := config.ParseSecretRef(ref)
	if err != nil {
		return err
	}

	if secretRef.KeyName == "" {
		return errKeyNameRequired
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

	// Create AWS client with caching (direct secret access bypasses config)
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

	fmt.Println(value) //nolint:revive,forbidigo // CLI output to stdout
	return nil
}

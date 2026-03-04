// Package cmd implements the CLI commands for envctl.
// This file contains the validate command for testing configuration and connectivity.
package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/sentiolabs/envctl/internal/config"
	"github.com/sentiolabs/envctl/internal/secrets"
	"github.com/spf13/cobra"
)

// validateCmd tests configuration validity and backend connectivity.
var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configuration and backend connectivity",
	Long: `Validate the configuration file and test backend connectivity.

This checks:
- Config file syntax and required fields
- Backend credentials are valid
- All referenced secrets are accessible
- All mapping references resolve correctly

Example:
  envctl validate
  envctl validate -e staging`,
	RunE: runValidate,
}

// init registers the validate command with the root command.
func init() {
	rootCmd.AddCommand(validateCmd)
}

// runValidate executes the validation logic: loads config, resolves environment,
// creates secrets client, and tests access to all referenced secrets.
// The fmt.Fprintf/Fprintln calls output status to stdout and always succeed.
//
//nolint:revive // Validation requires checking multiple conditions; stdout writes always succeed
func runValidate(cmd *cobra.Command, args []string) error {
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

	fmt.Fprintf(os.Stdout, "✓ Config file: %s\n", configPath)

	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	// Resolve environment config
	envConfig, app, err := resolveEnvironmentConfig(cfg)
	if err != nil {
		return err
	}

	// Display what we're validating
	if cfg.HasApplications() {
		selectedApp := appName
		if selectedApp == "" {
			selectedApp = cfg.DefaultApplication
		}
		fmt.Fprintf(os.Stdout, "✓ Application: %s\n", selectedApp)
	}
	selectedEnv := envName
	if selectedEnv == "" {
		selectedEnv = cfg.DefaultEnvironment
	}
	fmt.Fprintf(os.Stdout, "✓ Environment: %s\n", selectedEnv)

	// Determine include_all mode from flag override or config
	includeAllOverride := getIncludeAllOverride(cmd)
	var includeAllEnabled bool
	if includeAllOverride != nil {
		includeAllEnabled = *includeAllOverride
	} else {
		includeAllEnabled = cfg.ShouldIncludeAll(app, envConfig)
	}

	if includeAllEnabled {
		fmt.Fprintln(os.Stdout, "✓ Mode: include_all (all keys from sources)")
	} else {
		fmt.Fprintln(os.Stdout, "✓ Mode: mappings-only (explicit keys only)")

		// Warn if no mappings or specific sources defined
		totalMappings := len(cfg.Mapping)
		if app != nil {
			totalMappings += len(app.Mapping)
		}
		specificSources := countSpecificSources(envConfig.Sources)

		if totalMappings == 0 && specificSources == 0 {
			fmt.Fprintln(os.Stderr, "⚠ Warning: no mappings or specific sources defined")
			fmt.Fprintln(os.Stderr, "  No environment variables will be injected")
			fmt.Fprintln(os.Stderr, "  Add mappings or set include_all: true in config")
		}

		// Check for source entries without key
		if hasWildcardSources(envConfig.Sources) {
			fmt.Fprintln(os.Stderr, "⚠ Warning: source entries without 'key' will fail")
			fmt.Fprintln(os.Stderr, "  In mappings-only mode, sources must specify a key")
			fmt.Fprintln(os.Stderr, "  Add 'key' to source entries or set include_all: true")
		}
	}

	// Create primary secrets client from first source
	client, err := createSecretsClient(ctx, cfg, envConfig)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "✓ Backend: %s (authenticated)\n", client.Name())

	// Test all sources from the resolved environment
	totalKeys := 0
	totalKeys += validateSources(ctx, cfg, client, envConfig.Sources, selectedEnv)

	// Test global mapping references
	if len(cfg.Mapping) > 0 {
		if err := validateMapping(ctx, client, cfg.Mapping, "global"); err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "✓ Global mapping: %d entries resolved\n", len(cfg.Mapping))
	}

	// Test app-level mapping references
	if app != nil && len(app.Mapping) > 0 {
		if err := validateMapping(ctx, client, app.Mapping, "app"); err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "✓ App mapping: %d entries resolved\n", len(app.Mapping))
	}

	fmt.Fprintln(os.Stdout)
	fmt.Fprintf(os.Stdout, "Total: %d environment variables will be set\n", totalKeys)

	return nil
}

// validateSources tests source secrets and returns count of keys.
//
//nolint:revive // CLI output to stdout always succeeds
func validateSources(
	ctx context.Context,
	cfg *config.Config,
	primaryClient secrets.Client,
	sources []config.IncludeEntry,
	scope string,
) int {
	totalKeys := 0
	for _, src := range sources {
		client, err := clientForValidate(ctx, cfg, primaryClient, src)
		if err != nil {
			fmt.Fprintf(os.Stderr, "✗ Source (%s) '%s': failed to create client: %v\n", scope, src.Secret, err)
			continue
		}

		switch {
		case src.Key != "":
			// Test specific key access
			_, err := client.GetSecretKey(ctx, src.Secret, src.Key)
			if err != nil {
				fmt.Fprintf(os.Stderr, "✗ Source (%s) '%s#%s': %v\n", scope, src.Secret, src.Key, err)
				continue
			}
			fmt.Fprintf(os.Stdout, "✓ Source (%s) '%s#%s': accessible\n", scope, src.Secret, src.Key)
			totalKeys++
		case len(src.Keys) > 0:
			// Test multiple specific keys from same secret
			for _, km := range src.Keys {
				_, err := client.GetSecretKey(ctx, src.Secret, km.Key)
				if err != nil {
					fmt.Fprintf(os.Stderr, "✗ Source (%s) '%s#%s': %v\n", scope, src.Secret, km.Key, err)
					continue
				}
				name := km.Key
				if km.As != "" {
					name = km.As
				}
				fmt.Fprintf(os.Stdout, "✓ Source (%s) '%s#%s' as %s: accessible\n", scope, src.Secret, km.Key, name)
				totalKeys++
			}
		default:
			// Test full secret access
			srcSecrets, err := client.GetSecret(ctx, src.Secret)
			if err != nil {
				fmt.Fprintf(os.Stderr, "✗ Source (%s) '%s': %v\n", scope, src.Secret, err)
				continue
			}
			fmt.Fprintf(os.Stdout, "✓ Source (%s) '%s': accessible (%d keys)\n", scope, src.Secret, len(srcSecrets))
			totalKeys += len(srcSecrets)
		}
	}
	return totalKeys
}

// clientForValidate returns the appropriate secrets client for a source entry.
// If the source has a backend qualifier (aws: or 1pass:) different from the primary,
// a new client is created for that backend.
func clientForValidate(
	ctx context.Context,
	cfg *config.Config,
	primaryClient secrets.Client,
	src config.IncludeEntry,
) (secrets.Client, error) {
	// No backend qualifier — use primary client
	if src.AWS == nil && src.OnePass == nil {
		return primaryClient, nil
	}

	// Build a synthetic environment from the source's backend config
	syntheticEnv := config.NewEnvironment(src)

	return secrets.NewClient(ctx, secrets.Options{
		Config:  cfg,
		Env:     &syntheticEnv,
		NoCache: noCache,
		Refresh: refresh,
	})
}

// validateMapping tests mapping entries.
func validateMapping(
	ctx context.Context,
	client secrets.Client,
	mapping map[string]string,
	scope string,
) error {
	for envVar, ref := range mapping {
		secretRef, err := config.ParseSecretRef(ref)
		if err != nil {
			return err
		}
		_, err = client.GetSecretKey(ctx, secretRef.SecretName, secretRef.KeyName)
		if err != nil {
			return fmt.Errorf("mapping (%s) %s -> %s: %w", scope, envVar, ref, err)
		}
	}
	return nil
}

// countSpecificSources counts source entries that specify a key (or keys).
func countSpecificSources(sources []config.IncludeEntry) int {
	count := 0
	for _, src := range sources {
		if src.Key != "" {
			count++
		} else if len(src.Keys) > 0 {
			count += len(src.Keys)
		}
	}
	return count
}

// hasWildcardSources checks if any source entry doesn't specify a key or keys.
func hasWildcardSources(sources []config.IncludeEntry) bool {
	for _, src := range sources {
		if src.Key == "" && len(src.Keys) == 0 {
			return true
		}
	}
	return false
}

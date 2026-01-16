package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/sentiolabs/envctl/internal/config"
	"github.com/sentiolabs/envctl/internal/secrets"
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configuration and AWS connectivity",
	Long: `Validate the configuration file and test AWS connectivity.

This checks:
- Config file syntax and required fields
- AWS credentials are valid
- All referenced secrets are accessible
- All mapping references resolve correctly

Example:
  envctl validate
  envctl validate -e staging`,
	RunE: runValidate,
}

func init() {
	rootCmd.AddCommand(validateCmd)
}

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

	// Check include_all setting
	includeAllOverride := getIncludeAllOverride(cmd)
	includeAll := false
	if includeAllOverride != nil {
		includeAll = *includeAllOverride
	} else {
		includeAll = cfg.ShouldIncludeAll(app, envConfig)
	}

	if includeAll {
		fmt.Fprintln(os.Stdout, "✓ Mode: include_all (all keys from primary secret)")
	} else {
		fmt.Fprintln(os.Stdout, "✓ Mode: mappings-only (explicit keys only)")

		// Warn if no mappings or specific includes defined
		totalMappings := len(cfg.Mapping)
		if app != nil {
			totalMappings += len(app.Mapping)
		}
		specificIncludes := countSpecificIncludes(cfg.Include)
		if app != nil {
			specificIncludes += countSpecificIncludes(app.Include)
		}

		if totalMappings == 0 && specificIncludes == 0 {
			fmt.Fprintln(os.Stderr, "⚠ Warning: no mappings or specific includes defined")
			fmt.Fprintln(os.Stderr, "  No environment variables will be injected")
			fmt.Fprintln(os.Stderr, "  Add mappings or set include_all: true in config")
		}

		// Check for include entries without key
		hasWildcardIncludes := hasIncludeAllEntries(cfg.Include)
		if app != nil && !hasWildcardIncludes {
			hasWildcardIncludes = hasIncludeAllEntries(app.Include)
		}
		if hasWildcardIncludes {
			fmt.Fprintln(os.Stderr, "⚠ Warning: include entries without 'key' will fail")
			fmt.Fprintln(os.Stderr, "  In mappings-only mode, includes must specify a key")
			fmt.Fprintln(os.Stderr, "  Add 'key' to include entries or set include_all: true")
		}
	}

	// Create secrets client
	client, err := createSecretsClient(ctx, cfg, envConfig.Region, envConfig.Profile)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "✓ Backend: %s (authenticated)\n", client.Name())

	// Test primary secret
	totalKeys := 0
	secrets, err := client.GetSecret(ctx, envConfig.Secret)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "✓ Secret '%s': accessible (%d keys)\n", envConfig.Secret, len(secrets))
	totalKeys += len(secrets)

	// Test global include secrets
	totalKeys += validateIncludes(client, ctx, cfg.Include, "global")

	// Test app-level include secrets
	if app != nil && len(app.Include) > 0 {
		totalKeys += validateIncludes(client, ctx, app.Include, "app")
	}

	// Test global mapping references
	if len(cfg.Mapping) > 0 {
		if err := validateMapping(client, ctx, cfg.Mapping, "global"); err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "✓ Global mapping: %d entries resolved\n", len(cfg.Mapping))
	}

	// Test app-level mapping references
	if app != nil && len(app.Mapping) > 0 {
		if err := validateMapping(client, ctx, app.Mapping, "app"); err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "✓ App mapping: %d entries resolved\n", len(app.Mapping))
	}

	fmt.Fprintln(os.Stdout)
	fmt.Fprintf(os.Stdout, "Total: %d environment variables will be set\n", totalKeys)

	return nil
}

// validateIncludes tests include secrets and returns count of keys.
func validateIncludes(client secrets.Client, ctx context.Context, includes []config.IncludeEntry, scope string) int {
	totalKeys := 0
	for _, inc := range includes {
		if inc.Key != "" {
			// Test specific key access
			_, err := client.GetSecretKey(ctx, inc.Secret, inc.Key)
			if err != nil {
				fmt.Fprintf(os.Stderr, "✗ Include (%s) '%s#%s': %v\n", scope, inc.Secret, inc.Key, err)
				continue
			}
			fmt.Fprintf(os.Stdout, "✓ Include (%s) '%s#%s': accessible\n", scope, inc.Secret, inc.Key)
			totalKeys++
		} else {
			// Test full secret access
			incSecrets, err := client.GetSecret(ctx, inc.Secret)
			if err != nil {
				fmt.Fprintf(os.Stderr, "✗ Include (%s) '%s': %v\n", scope, inc.Secret, err)
				continue
			}
			fmt.Fprintf(os.Stdout, "✓ Include (%s) '%s': accessible (%d keys)\n", scope, inc.Secret, len(incSecrets))
			totalKeys += len(incSecrets)
		}
	}
	return totalKeys
}

// validateMapping tests mapping entries.
func validateMapping(client secrets.Client, ctx context.Context, mapping map[string]string, scope string) error {
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

// countSpecificIncludes counts include entries that specify a key.
func countSpecificIncludes(includes []config.IncludeEntry) int {
	count := 0
	for _, inc := range includes {
		if inc.Key != "" {
			count++
		}
	}
	return count
}

// hasIncludeAllEntries checks if any include entry doesn't specify a key.
func hasIncludeAllEntries(includes []config.IncludeEntry) bool {
	for _, inc := range includes {
		if inc.Key == "" {
			return true
		}
	}
	return false
}

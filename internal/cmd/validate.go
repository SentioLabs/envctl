package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/sentiolabs/envctl/internal/aws"
	"github.com/sentiolabs/envctl/internal/config"
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

	// Create AWS client with caching
	client, err := createSecretsClient(ctx, cfg, envConfig.Region, envConfig.Profile)
	if err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, "✓ AWS credentials: valid")

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
func validateIncludes(client *aws.SecretsClient, ctx context.Context, includes []config.IncludeEntry, scope string) int {
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
func validateMapping(client *aws.SecretsClient, ctx context.Context, mapping map[string]string, scope string) error {
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

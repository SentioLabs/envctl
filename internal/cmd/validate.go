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

	// Get environment config
	selectedEnv := envName
	if selectedEnv == "" {
		selectedEnv = cfg.DefaultEnvironment
	}
	envConfig, err := cfg.GetEnvironment(selectedEnv)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "✓ Environment: %s\n", selectedEnv)

	// Create AWS client
	client, err := aws.NewSecretsClient(ctx, envConfig.Region)
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

	// Test include secrets
	for _, inc := range cfg.Include {
		if inc.Key != "" {
			// Test specific key access
			_, err := client.GetSecretKey(ctx, inc.Secret, inc.Key)
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stdout, "✓ Include '%s#%s': accessible\n", inc.Secret, inc.Key)
			totalKeys++
		} else {
			// Test full secret access
			incSecrets, err := client.GetSecret(ctx, inc.Secret)
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stdout, "✓ Include '%s': accessible (%d keys)\n", inc.Secret, len(incSecrets))
			totalKeys += len(incSecrets)
		}
	}

	// Test mapping references
	if len(cfg.Mapping) > 0 {
		for envVar, ref := range cfg.Mapping {
			secretRef, err := config.ParseSecretRef(ref)
			if err != nil {
				return err
			}
			_, err = client.GetSecretKey(ctx, secretRef.SecretName, secretRef.KeyName)
			if err != nil {
				return fmt.Errorf("mapping %s -> %s: %w", envVar, ref, err)
			}
		}
		fmt.Fprintf(os.Stdout, "✓ Mapping: %d entries resolved\n", len(cfg.Mapping))
	}

	fmt.Fprintln(os.Stdout)
	fmt.Fprintf(os.Stdout, "Total: %d environment variables will be set\n", totalKeys)

	return nil
}

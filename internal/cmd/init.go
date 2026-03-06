// Package cmd implements the CLI commands for envctl.
// This file contains the init command for creating starter configuration files.
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sentiolabs/envctl/internal/config"
	"github.com/spf13/cobra"
)

const (
	// backendOnePassword is the identifier for the 1Password backend.
	backendOnePassword = "1password"
	// configFilePerm is the file permission for created config files.
	configFilePerm = 0o600
)

// Command flags for init command.
var (
	initSecret  string
	initBackend string

	// initCmd creates a starter .envctl.yaml configuration file.
	initCmd = &cobra.Command{
		Use:   "init",
		Short: "Create a starter configuration file",
		Long: `Create a starter .envctl.yaml configuration file in the current directory.

Example:
  envctl init
  envctl init --secret myapp/dev
  envctl init --backend 1password --secret "My App Secrets"`,
		RunE: runInit,
	}
)

// init registers the init command with the root command.
func init() {
	initCmd.Flags().StringVar(&initSecret, "secret", "", "primary secret/item name for dev environment")
	initCmd.Flags().StringVar(&initBackend, "backend", "aws", "secrets backend: aws or 1password")
	rootCmd.AddCommand(initCmd)
}

// runInit creates a new .envctl.yaml config file with the specified backend.
//
//nolint:revive // CLI output to stdout always succeeds
func runInit(cmd *cobra.Command, args []string) error {
	configPath := filepath.Join(".", config.ConfigFileName)

	// Check if file already exists
	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf("config file already exists: %s", configPath)
	}

	// Validate backend
	if initBackend != "aws" && initBackend != backendOnePassword {
		return fmt.Errorf("invalid backend: %s (must be 'aws' or '1password')", initBackend)
	}

	// Generate content based on backend
	var content string
	if initBackend == backendOnePassword {
		content = generateOnePasswordConfig()
	} else {
		content = generateAWSConfig()
	}

	// Write file with restricted permissions (config may contain sensitive paths)
	if err := os.WriteFile(configPath, []byte(content), configFilePerm); err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "Created %s\n", configPath)
	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, "Next steps:")
	if initBackend == backendOnePassword {
		fmt.Fprintln(os.Stdout, "  1. Edit .envctl.yaml with your 1Password item names")
		fmt.Fprintln(os.Stdout, "  2. Ensure 1Password CLI is installed and configured")
		fmt.Fprintln(os.Stdout, "  3. Run 'envctl validate' to test connectivity")
	} else {
		fmt.Fprintln(os.Stdout, "  1. Edit .envctl.yaml with your AWS secret names")
		fmt.Fprintln(os.Stdout, "  2. Run 'envctl validate' to test connectivity")
	}
	fmt.Fprintln(os.Stdout, "  4. Run 'envctl run -- your-command' to start development")
	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, "Don't forget to add .env to your .gitignore!")

	return nil
}

func generateAWSConfig() string {
	if initSecret != "" {
		return fmt.Sprintf(`# envctl configuration
# See: https://github.com/sentiolabs/envctl

version: 1

# Default environment when -e/--env not specified
default_environment: dev

# Optional: AWS settings (region/profile). Defaults to AWS SDK credential chain.
# aws:
#   region: us-east-1
#   profile: my-profile

# Environment definitions
environments:
  dev:
    secret: %s
  # staging:
  #   secret: %s-staging
  # prod:
  #   secret: %s-prod

# Optional: Additional secrets to include (must be environment-keyed)
# include:
#   dev:
#     - secret: shared/datadog
#     - secret: shared/stripe
#       key: test_key
#       as: STRIPE_SECRET_KEY

# Optional: Explicit env var mappings
# mapping:
#   DATABASE_URL: %s#database_url
`, initSecret, initSecret, initSecret, initSecret)
	}

	return `# envctl configuration
# See: https://github.com/sentiolabs/envctl

version: 1

# Default environment when -e/--env not specified
default_environment: dev

# Optional: AWS settings (region/profile). Defaults to AWS SDK credential chain.
# aws:
#   region: us-east-1
#   profile: my-profile

# Environment definitions
environments:
  dev:
    secret: your-app/dev  # Replace with your AWS secret name
    # Per-environment AWS overrides go under an aws: block:
    # aws:
    #   region: us-west-2
    #   profile: my-profile
  # staging:
  #   secret: your-app/staging
  # prod:
  #   secret: your-app/prod

# Optional: Additional secrets to include (must be environment-keyed)
# include:
#   dev:
#     # Pull all keys from a shared secret
#     - secret: shared/datadog
#
#     # Pull specific key and rename it
#     - secret: shared/stripe
#       key: test_key
#       as: STRIPE_SECRET_KEY

# Optional: Explicit env var mappings using secret#key syntax
# mapping:
#   DATABASE_URL: your-app/dev#database_url
#   REDIS_URL: your-app/dev#redis_url
`
}

func generateOnePasswordConfig() string {
	if initSecret != "" {
		return fmt.Sprintf(`# envctl configuration - 1Password
# See: https://github.com/sentiolabs/envctl

version: 1

# 1Password settings (the '1pass' block selects the backend)
1pass:
  vault: Development  # Default vault (change to your vault name)
  # account: my-team    # Optional: short domain, full URL, or account ID

# Default environment when -e/--env not specified
default_environment: dev

# Environment definitions
# For 1Password, 'secret' is the item name in your vault
environments:
  dev:
    secret: %s
  # staging:
  #   secret: %s Staging
  # prod:
  #   secret: %s Prod

# Optional: Additional 1Password items to include (must be environment-keyed)
# include:
#   dev:
#     - secret: Shared Secrets
#     - secret: API Keys
#       key: stripe_key
#       as: STRIPE_SECRET_KEY

# Optional: Explicit env var mappings
# mapping:
#   DATABASE_URL: Database Credentials#connection_string
`, initSecret, initSecret, initSecret)
	}

	return `# envctl configuration - 1Password
# See: https://github.com/sentiolabs/envctl

version: 1

# 1Password settings (the '1pass' block selects the backend)
1pass:
  vault: Development  # Default vault (change to your vault name)
  # account: my-team    # Optional: short domain, full URL, or account ID

# Default environment when -e/--env not specified
default_environment: dev

# Environment definitions
# For 1Password, 'secret' is the item name in your vault
environments:
  dev:
    secret: My App Dev  # Replace with your 1Password item name
  # staging:
  #   secret: My App Staging
  # prod:
  #   secret: My App Prod

# Optional: Additional 1Password items to include (must be environment-keyed)
# include:
#   dev:
#     # Pull all fields from a shared item
#     - secret: Shared Secrets
#
#     # Pull specific field and rename it
#     - secret: API Keys
#       key: stripe_key
#       as: STRIPE_SECRET_KEY

# Optional: Explicit env var mappings
# mapping:
#   DATABASE_URL: Database Credentials#connection_string
#   REDIS_URL: Redis Config#url
`
}

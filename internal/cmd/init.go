package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sentiolabs/envctl/internal/config"
	"github.com/spf13/cobra"
)

var (
	initSecret string

	initCmd = &cobra.Command{
		Use:   "init",
		Short: "Create a starter configuration file",
		Long: `Create a starter .envctl.yaml configuration file in the current directory.

Example:
  envctl init
  envctl init --secret myapp/dev`,
		RunE: runInit,
	}
)

func init() {
	initCmd.Flags().StringVar(&initSecret, "secret", "", "primary secret name for dev environment")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	configPath := filepath.Join(".", config.ConfigFileName)

	// Check if file already exists
	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf("config file already exists: %s", configPath)
	}

	// Generate content
	var content string
	if initSecret != "" {
		content = fmt.Sprintf(`# envctl configuration
# See: https://github.com/sentiolabs/envctl

version: 1

# Default environment when -e/--env not specified
default_environment: dev

# Environment definitions
environments:
  dev:
    secret: %s
  # staging:
  #   secret: %s-staging
  # prod:
  #   secret: %s-prod

# Optional: Additional secrets to include
# include:
#   - secret: shared/datadog
#   - secret: shared/stripe
#     key: test_key
#     as: STRIPE_SECRET_KEY

# Optional: Explicit env var mappings
# mapping:
#   DATABASE_URL: %s#database_url
`, initSecret, initSecret, initSecret, initSecret)
	} else {
		content = `# envctl configuration
# See: https://github.com/sentiolabs/envctl

version: 1

# Default environment when -e/--env not specified
default_environment: dev

# Environment definitions
environments:
  dev:
    secret: your-app/dev  # Replace with your secret name
    # region: us-west-2   # Optional: override AWS region
  # staging:
  #   secret: your-app/staging
  # prod:
  #   secret: your-app/prod

# Optional: Additional secrets to include
# include:
#   # Pull all keys from a shared secret
#   - secret: shared/datadog
#
#   # Pull specific key and rename it
#   - secret: shared/stripe
#     key: test_key
#     as: STRIPE_SECRET_KEY

# Optional: Explicit env var mappings using secret#key syntax
# mapping:
#   DATABASE_URL: your-app/dev#database_url
#   REDIS_URL: your-app/dev#redis_url
`
	}

	// Write file
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "Created %s\n", configPath)
	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, "Next steps:")
	fmt.Fprintln(os.Stdout, "  1. Edit .envctl.yaml with your secret names")
	fmt.Fprintln(os.Stdout, "  2. Run 'envctl validate' to test connectivity")
	fmt.Fprintln(os.Stdout, "  3. Run 'envctl run -- your-command' to start development")
	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, "Don't forget to add .env to your .gitignore!")

	return nil
}

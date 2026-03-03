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
	outputFile string

	envCmd = &cobra.Command{
		Use:   "env",
		Short: "Output secrets in .env format",
		Long: `Output secrets in .env format for use with Docker Compose or other tools
that read .env files.

Example:
  envctl env > .env
  envctl env -e staging > .env
  envctl env -o .env`,
		RunE: runEnv,
	}
)

func init() {
	envCmd.Flags().StringVarP(&outputFile, "output", "o", "", "output file (default: stdout)")
	rootCmd.AddCommand(envCmd)
}

func runEnv(cmd *cobra.Command, args []string) error {
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

	// Resolve environment config
	envConfig, _, err := resolveEnvironmentConfig(cfg)
	if err != nil {
		return err
	}
	selectedEnv := envName
	if selectedEnv == "" {
		selectedEnv = cfg.DefaultEnvironment
	}
	verboseLog("Using environment: %s (secret: %s)", selectedEnv, envConfig.Secret)

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
	verboseLog("Loaded %d environment variables", len(entries))

	// Determine output destination
	var w *os.File
	if outputFile != "" {
		f, err := os.Create(outputFile)
		if err != nil {
			return err
		}
		defer f.Close()
		w = f
		verboseLog("Writing to: %s", outputFile)

		// Check gitignore
		checkGitignore()
	} else {
		w = os.Stdout
	}

	// Write output
	return output.WriteEnvFile(w, entries, selectedEnv)
}

// checkGitignore warns if .env is not in .gitignore.
func checkGitignore() {
	data, err := os.ReadFile(".gitignore")
	if err != nil {
		verboseLog("Warning: .gitignore not found - ensure .env is not committed")
		return
	}
	if !containsEnvIgnore(string(data)) {
		verboseLog("Warning: .env may not be in .gitignore - secrets could be committed")
	}
}

// containsEnvIgnore checks if .gitignore contains .env pattern.
func containsEnvIgnore(content string) bool {
	lines := []string{".env", "*.env", ".env*"}
	for _, pattern := range lines {
		if containsPattern(content, pattern) {
			return true
		}
	}
	return false
}

// containsPattern checks if content contains a gitignore pattern.
func containsPattern(content, pattern string) bool {
	// Simple check - look for the pattern on its own line
	return len(content) > 0 && (content == pattern ||
		len(content) > len(pattern)+1 && (content[:len(pattern)+1] == pattern+"\n" ||
			content[len(content)-len(pattern)-1:] == "\n"+pattern))
}

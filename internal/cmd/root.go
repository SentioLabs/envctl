// Package cmd implements the CLI commands for envctl.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	// Persistent flags
	configFile string
	envName    string
	verbose    bool

	rootCmd = &cobra.Command{
		Use:   "envctl",
		Short: "AWS Secrets Manager CLI for local development",
		Long: `envctl enables developers to use AWS Secrets Manager as the single source
of truth for application secrets during local development.

It provides a simple way to inject secrets into process environments or
generate .env files for Docker Compose workflows.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
)

func init() {
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "config file path (default: .envctl.yaml)")
	rootCmd.PersistentFlags().StringVarP(&envName, "env", "e", "", "environment name (default: from config)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

// verboseLog prints a message if verbose mode is enabled.
func verboseLog(format string, args ...any) {
	if verbose {
		fmt.Fprintf(os.Stderr, "[envctl] "+format+"\n", args...)
	}
}

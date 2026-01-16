// Package cmd implements the CLI commands for envctl.
// This file contains the docs command for displaying built-in documentation.
package cmd

import (
	"fmt"
	"os"

	"github.com/sentiolabs/envctl/internal/docs"
	"github.com/spf13/cobra"
)

// validTopics lists all available documentation topics for completion.
var (
	validTopics = []string{"config", "examples", "k8s", "patterns", "1password"}

	// docsCmd displays embedded documentation for various topics.
	docsCmd = &cobra.Command{
		Use:   "docs [topic]",
		Short: "Display documentation for envctl",
		Long: `Display documentation about envctl configuration and usage.

Available topics:
  config      Configuration file format (.envctl.yaml)
  examples    Example configurations for common patterns
  k8s         Converting Kubernetes secrets to envctl
  patterns    Common integration patterns (Docker, direnv, etc.)
  1password   Using 1Password as a secrets backend

Run 'envctl docs <topic>' for detailed information on a topic.`,
		Args:              cobra.MaximumNArgs(1),
		RunE:              runDocs,
		ValidArgsFunction: completeDocTopics,
	}
)

// init registers the docs command with the root command.
func init() {
	rootCmd.AddCommand(docsCmd)
}

// runDocs displays documentation for the specified topic.
// If no topic is provided, it displays the overview documentation.
func runDocs(cmd *cobra.Command, args []string) error {
	topic := ""
	if len(args) > 0 {
		topic = args[0]
	}

	var content string
	switch topic {
	case "":
		content = docs.Overview
	case "config":
		content = docs.Config
	case "examples":
		content = docs.Examples
	case "k8s":
		content = docs.K8s
	case "patterns":
		content = docs.Patterns
	case "1password":
		content = docs.OnePassword
	default:
		return fmt.Errorf("unknown topic: %s\nAvailable topics: config, examples, k8s, patterns, 1password", topic)
	}

	fmt.Fprint(os.Stdout, content) //nolint:revive // output to stdout always succeeds
	return nil
}

// completeDocTopics provides shell completion for documentation topics.
func completeDocTopics(
	cmd *cobra.Command,
	args []string,
	toComplete string,
) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return validTopics, cobra.ShellCompDirectiveNoFileComp
}

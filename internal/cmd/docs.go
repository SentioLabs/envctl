package cmd

import (
	"fmt"
	"os"

	"github.com/sentiolabs/envctl/internal/docs"
	"github.com/spf13/cobra"
)

var (
	validTopics = []string{"config", "examples", "k8s", "patterns"}

	docsCmd = &cobra.Command{
		Use:   "docs [topic]",
		Short: "Display documentation for envctl",
		Long: `Display documentation about envctl configuration and usage.

Available topics:
  config    Configuration file format (.envctl.yaml)
  examples  Example configurations for common patterns
  k8s       Converting Kubernetes secrets to envctl
  patterns  Common integration patterns (Docker, direnv, etc.)

Run 'envctl docs <topic>' for detailed information on a topic.`,
		Args:              cobra.MaximumNArgs(1),
		RunE:              runDocs,
		ValidArgsFunction: completeDocTopics,
	}
)

func init() {
	rootCmd.AddCommand(docsCmd)
}

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
	default:
		return fmt.Errorf("unknown topic: %s\nAvailable topics: config, examples, k8s, patterns", topic)
	}

	fmt.Fprint(os.Stdout, content)
	return nil
}

func completeDocTopics(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return validTopics, cobra.ShellCompDirectiveNoFileComp
}

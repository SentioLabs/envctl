// Package cmd implements the CLI commands for envctl.
// This file contains the version command for displaying build information.
package cmd

import (
	"fmt"

	"github.com/sentiolabs/envctl/internal/version"
	"github.com/spf13/cobra"
)

// versionCmd displays the version, git commit, and build date.
//
//nolint:revive,forbidigo // CLI output to stdout always succeeds
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Long:  `Print the version, git commit, and build date of envctl.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("envctl %s\n", version.Version)
		fmt.Printf("  commit: %s\n", version.GitCommit)
		fmt.Printf("  built:  %s\n", version.BuildDate)
	},
}

// init registers the version command with the root command.
func init() {
	rootCmd.AddCommand(versionCmd)
}

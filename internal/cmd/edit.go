// Package cmd implements the CLI commands for envctl.
// This file contains the edit command for launching the interactive secret editor TUI.
package cmd

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sentiolabs/envctl/internal/config"
	"github.com/sentiolabs/envctl/internal/secrets"
	"github.com/sentiolabs/envctl/internal/tui"
	"github.com/spf13/cobra"
)

// Command flags for edit command.
var (
	editVault string
	editItem  string

	editCmd = &cobra.Command{
		Use:   "edit",
		Short: "Interactive secret editor",
		Long: `Launch an interactive TUI for browsing and editing secrets.

Supports editing field values, renaming keys, deleting fields,
creating new items, and toggling field types (1Password).

Example:
  envctl edit
  envctl edit --vault BACstack
  envctl edit --vault BACstack --item "Core API"`,
		RunE: runEdit,
	}
)

// init registers the edit command with the root command.
func init() {
	editCmd.Flags().StringVar(&editVault, "vault", "", "pre-select vault (skip vault picker)")
	editCmd.Flags().StringVar(&editItem, "item", "", "pre-select item (skip to field editor)")
	rootCmd.AddCommand(editCmd)
}

// runEdit launches the interactive TUI for browsing and editing secrets.
func runEdit(cmd *cobra.Command, args []string) error {
	if editItem != "" && editVault == "" {
		return fmt.Errorf("--item requires --vault")
	}

	ctx := context.Background()

	// Load config if available
	var cfg *config.Config
	configPath := configFile
	if configPath == "" {
		configPath, _ = config.FindConfig()
	}
	if configPath != "" {
		cfg, _ = config.Load(configPath)
	}

	// Resolve environment config for backend selection
	var envConfig *config.Environment
	if cfg != nil {
		envConfig, _, _ = resolveEnvironmentConfig(cfg)
	}

	editor, err := secrets.NewEditor(ctx, secrets.EditorOptions{
		Config: cfg,
		Env:    envConfig,
	})
	if err != nil {
		return err
	}

	model := tui.New(tui.Options{
		Editor: editor,
		Vault:  editVault,
		Item:   editItem,
	})

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}

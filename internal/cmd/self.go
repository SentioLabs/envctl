// Package cmd implements the CLI commands for envctl.
// This file contains the self command group and self update subcommand.
package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/sentiolabs/envctl/internal/version"
	"github.com/spf13/cobra"
	"golang.org/x/mod/semver"
)

var (
	selfForce bool
	selfCheck bool
	selfYes   bool
)

// selfCmd is the parent command for self-management subcommands.
var selfCmd = &cobra.Command{
	Use:   "self",
	Short: "Manage the envctl CLI itself",
}

// selfUpdateCmd checks for and installs updates to the envctl binary.
var selfUpdateCmd = &cobra.Command{
	Use:          "update",
	Short:        "Update envctl to the latest version",
	SilenceUsage: true,
	Long: `Update envctl to the latest version from GitHub releases.

Examples:
  envctl self update          Update if a new version is available
  envctl self update --check  Check for updates without installing
  envctl self update --force  Force reinstall even if up-to-date
  envctl self update -y       Update without confirmation prompt`,
	RunE: runSelfUpdate,
}

func init() {
	selfCmd.AddCommand(selfUpdateCmd)
	rootCmd.AddCommand(selfCmd)

	selfUpdateCmd.Flags().BoolVarP(&selfForce, "force", "f", false, "Force update even if already up-to-date")
	selfUpdateCmd.Flags().BoolVar(&selfCheck, "check", false, "Check for updates without installing")
	selfUpdateCmd.Flags().BoolVarP(&selfYes, "yes", "y", false, "Skip confirmation prompt")
}

// githubReleasesURL is the base URL for the GitHub releases API.
// It is a package-level var so tests can override it.
var githubReleasesURL = "https://api.github.com/repos/sentiolabs/envctl/releases"

// githubRelease represents the relevant fields from the GitHub releases API response.
type githubRelease struct {
	TagName string `json:"tag_name"`
}

// getLatestVersion fetches the latest release tag from the GitHub API.
func getLatestVersion() (string, error) {
	resp, err := http.Get(githubReleasesURL + "/latest") //nolint:noctx // Simple one-shot HTTP request for CLI tool
	if err != nil {
		return "", fmt.Errorf("failed to fetch latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %d from GitHub API", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("failed to decode release response: %w", err)
	}

	return release.TagName, nil
}

// ensureVPrefix normalizes a version string to have a "v" prefix.
// Empty strings and "dev" are treated as development versions.
func ensureVPrefix(v string) string {
	if v == "" || v == "dev" {
		return "v0.0.0-dev"
	}
	if !strings.HasPrefix(v, "v") {
		return "v" + v
	}
	return v
}

// runSelfUpdate implements the self update command logic.
//
//nolint:revive // CLI output to stdout
func runSelfUpdate(cmd *cobra.Command, _ []string) error {
	current := ensureVPrefix(version.Version)

	latest, err := getLatestVersion()
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	cmp := semver.Compare(latest, current)

	// Check-only mode: print status and return
	if selfCheck {
		switch {
		case cmp == 0:
			fmt.Fprintf(cmd.OutOrStdout(), "envctl %s is up to date\n", current)
		case cmp > 0:
			fmt.Fprintf(cmd.OutOrStdout(), "Update available: %s -> %s\n", current, latest)
		default:
			fmt.Fprintf(cmd.OutOrStdout(), "Current version %s is newer than latest release %s\n", current, latest)
		}
		return nil
	}

	// Already up to date
	if cmp == 0 && !selfForce {
		fmt.Fprintf(cmd.OutOrStdout(), "envctl %s is already up to date\n", current)
		return nil
	}

	// Current is newer than latest
	if cmp < 0 && !selfForce {
		fmt.Fprintf(cmd.OutOrStdout(), "Current version %s is newer than latest release %s\n", current, latest)
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Updating envctl %s -> %s...\n", current, latest)

	// Confirmation prompt
	if !selfYes {
		fmt.Fprint(cmd.OutOrStdout(), "Continue? [y/N] ")
		reader := bufio.NewReader(os.Stdin)
		answer, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}
		answer = strings.TrimSpace(answer)
		if answer != "y" && answer != "Y" {
			fmt.Fprintln(cmd.OutOrStdout(), "Update cancelled.")
			return nil
		}
	}

	return runInstallScript(latest)
}

// runInstallScript downloads and runs the install script for the given version tag.
func runInstallScript(tag string) error {
	const installURL = "https://raw.githubusercontent.com/sentiolabs/envctl/main/scripts/install.sh"
	script := fmt.Sprintf(
		"curl -fsSL %s | bash -s -- --force --tag=%s", installURL, tag,
	)
	installCmd := exec.Command("bash", "-c", script)
	installCmd.Stdout = os.Stdout
	installCmd.Stderr = os.Stderr
	installCmd.Stdin = os.Stdin

	if err := installCmd.Run(); err != nil {
		return fmt.Errorf("install script failed: %w", err)
	}
	return nil
}

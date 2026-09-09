// Package cmd implements the CLI commands for envctl.
// This file wires the self command group (update, channel) to selfupdate-go.
package cmd

import (
	"github.com/sentiolabs/envctl/internal/userconfig"
	"github.com/sentiolabs/envctl/internal/version"
	"github.com/sentiolabs/selfupdate-go"
	"github.com/sentiolabs/selfupdate-go/cobracmd"
	"github.com/spf13/cobra"
)

// binaryName and repoOwner identify the GitHub releases self update reads.
const (
	binaryName = "envctl"
	repoOwner  = "sentiolabs"
)

// selfUpdater is the shared updater. Tests swap its Source and Store.
var selfUpdater = newSelfUpdater()

// newSelfUpdater builds the updater for the envctl binary: releases come from
// github.com/sentiolabs/envctl, the channel lives in the per-user config file,
// and installs verify and replace the running executable in place.
func newSelfUpdater() *selfupdate.Updater {
	return &selfupdate.Updater{
		Name:      binaryName,
		Version:   version.Version,
		Source:    &selfupdate.GitHubSource{Owner: repoOwner, Repo: binaryName},
		Store:     userconfig.ChannelStore(),
		Installer: &selfupdate.ArchiveInstaller{Name: binaryName},
	}
}

func init() {
	// --check has no shorthand: the root command owns -c for --config.
	rootCmd.AddCommand(newSelfCommand(selfUpdater))
}

// newSelfCommand binds native download progress to the same stream as Cobra.
func newSelfCommand(u *selfupdate.Updater) *cobra.Command {
	command := cobracmd.New(u)
	update, _, _ := command.Find([]string{"update"})
	update.PreRun = func(cmd *cobra.Command, _ []string) {
		if installer, ok := u.Installer.(*selfupdate.ArchiveInstaller); ok {
			installer.Out = cmd.OutOrStdout()
			if u.Out != nil {
				installer.Out = u.Out
			}
		}
	}
	return command
}

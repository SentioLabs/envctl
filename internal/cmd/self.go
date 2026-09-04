// Package cmd implements the CLI commands for envctl.
// This file wires the self command group (update, channel) to go-selfupdate.
package cmd

import (
	"github.com/sentiolabs/envctl/internal/userconfig"
	"github.com/sentiolabs/envctl/internal/version"
	"github.com/sentiolabs/go-selfupdate"
	"github.com/sentiolabs/go-selfupdate/cobracmd"
)

// installScriptURL is the install script that self update pipes into bash
// with --force --tag=<tag>.
const installScriptURL = "https://raw.githubusercontent.com/sentiolabs/envctl/main/scripts/install.sh"

// binaryName and repoOwner identify the GitHub releases self update reads.
const (
	binaryName = "envctl"
	repoOwner  = "sentiolabs"
)

// selfUpdater is the shared updater. Tests swap its Source and Store.
var selfUpdater = newSelfUpdater()

// newSelfUpdater builds the updater for the envctl binary: releases come from
// github.com/sentiolabs/envctl, the channel lives in the per-user config file,
// and installs run the repository's install script.
func newSelfUpdater() *selfupdate.Updater {
	return &selfupdate.Updater{
		Name:      binaryName,
		Version:   version.Version,
		Source:    &selfupdate.GitHubSource{Owner: repoOwner, Repo: binaryName},
		Store:     userconfig.ChannelStore(),
		Installer: &selfupdate.ScriptInstaller{ScriptURL: installScriptURL},
	}
}

func init() {
	// --check has no shorthand: the root command owns -c for --config.
	rootCmd.AddCommand(cobracmd.New(selfUpdater))
}

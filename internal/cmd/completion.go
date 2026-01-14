package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh]",
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for envctl.

To load completions:

Bash:
  # Linux
  $ envctl completion bash > /etc/bash_completion.d/envctl

  # macOS with Homebrew
  $ envctl completion bash > $(brew --prefix)/etc/bash_completion.d/envctl

  # Or load in current session
  $ source <(envctl completion bash)

Zsh:
  # If shell completion is not already enabled, enable it:
  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # Load completions for every session
  $ envctl completion zsh > "${fpath[1]}/_envctl"

  # Or for Oh My Zsh
  $ envctl completion zsh > ~/.oh-my-zsh/completions/_envctl

  # Or load in current session
  $ source <(envctl completion zsh)

After installing, restart your shell or source your profile.
`,
	Example: `  envctl completion bash
  envctl completion zsh
  source <(envctl completion bash)`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return cmd.Root().GenBashCompletion(os.Stdout)
		case "zsh":
			return cmd.Root().GenZshCompletion(os.Stdout)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}

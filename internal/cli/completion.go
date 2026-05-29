package cli

import (
	"github.com/spf13/cobra"
)

func newCompletionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "completion [bash|zsh|fish]",
		Short: "Generate shell completion script",
		Long: `Generate the autocompletion script for the specified shell.

Bash:
  source <(ncgo completion bash)
  # Or to persist: ncgo completion bash > /usr/local/etc/bash_completion.d/ncgo

Zsh:
  source <(ncgo completion zsh)
  # Or: ncgo completion zsh > /usr/local/share/zsh/site-functions/_ncgo

Fish:
  ncgo completion fish > ~/.config/fish/completions/ncgo.fish`,
		Example: `  ncgo completion bash > /usr/local/etc/bash_completion.d/ncgo
  ncgo completion zsh > /usr/local/share/zsh/site-functions/_ncgo
  ncgo completion fish > ~/.config/fish/completions/ncgo.fish`,
		ValidArgs: []string{"bash", "zsh", "fish"},
		Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			switch args[0] {
			case "bash":
				return cmd.Root().GenBashCompletionV2(out, true)
			case "zsh":
				return cmd.Root().GenZshCompletion(out)
			case "fish":
				return cmd.Root().GenFishCompletion(out, true)
			}
			return nil
		},
	}
}

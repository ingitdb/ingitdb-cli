package commands

// specscore: feature/cli/pull

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Pull returns the pull command.
func Pull() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pull",
		Short: "Pull latest changes, resolve conflicts, and rebuild views",
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("not yet implemented")
		},
	}
	addPathFlag(cmd)
	cmd.Flags().String("strategy", "", "git pull strategy: rebase (default) or merge")
	cmd.Flags().String("remote", "", "remote name (default: origin)")
	cmd.Flags().String("branch", "", "branch to pull (default: tracking branch)")
	return cmd
}

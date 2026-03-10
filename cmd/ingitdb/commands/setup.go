package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Setup returns the setup command.
func Setup() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Set up a new inGitDB database",
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("not yet implemented")
		},
	}
	addPathFlag(cmd)
	return cmd
}

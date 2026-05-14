package commands

// specscore: feature/cli/watch

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Watch returns the watch command.
func Watch() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Watch database for changes and log events to stdout",
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("not yet implemented")
		},
	}
	addPathFlag(cmd)
	addFormatFlag(cmd, "text")
	return cmd
}

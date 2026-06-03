package commands

// specscore: feature/cli/version

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version returns the version command.
func Version(ver, commit, date string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print build version, commit hash, and build date",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "ingitdb %s (%s) @ %s\n", ver, commit, date)
			return err
		},
	}
}

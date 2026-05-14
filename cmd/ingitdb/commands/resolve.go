package commands

// specscore: feature/cli/resolve

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Resolve returns the resolve command.
func Resolve() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resolve",
		Short: "Resolve merge conflicts in database files",
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("not yet implemented")
		},
	}
	addPathFlag(cmd)
	cmd.Flags().String("file", "", "specific file to resolve")
	return cmd
}

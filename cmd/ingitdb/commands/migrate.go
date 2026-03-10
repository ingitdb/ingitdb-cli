package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Migrate returns the migrate command.
func Migrate() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Migrate data between schema versions",
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("not yet implemented")
		},
	}
	cmd.Flags().String("from", "", "source schema version")
	_ = cmd.MarkFlagRequired("from")
	cmd.Flags().String("to", "", "target schema version")
	_ = cmd.MarkFlagRequired("to")
	cmd.Flags().String("target", "", "migration target")
	_ = cmd.MarkFlagRequired("target")
	addPathFlag(cmd)
	addFormatFlag(cmd, "")
	cmd.Flags().String("collections", "", "comma-separated list of collections to migrate")
	cmd.Flags().String("output-dir", "", "directory for migration output")
	return cmd
}

package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

func deleteRecords() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "records",
		Short: "Delete individual records from a collection",
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("not yet implemented")
		},
	}
	addPathFlag(cmd)
	addCollectionFlag(cmd, true)
	cmd.Flags().String("in", "", "regular expression scoping deletion to a sub-path")
	cmd.Flags().String("filter-name", "", "pattern to match record names to delete")
	return cmd
}


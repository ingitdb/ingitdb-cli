package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

func deleteCollection() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "collection",
		Short: "Delete a collection and all its records",
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("not yet implemented")
		},
	}
	addPathFlag(cmd)
	addCollectionFlag(cmd, true)
	return cmd
}


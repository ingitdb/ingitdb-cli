package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

func deleteView() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "view",
		Short: "Delete a view definition and its materialised files",
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("not yet implemented")
		},
	}
	addPathFlag(cmd)
	cmd.Flags().String("view", "", "view id to delete")
	_ = cmd.MarkFlagRequired("view")
	return cmd
}

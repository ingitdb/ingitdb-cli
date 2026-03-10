package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Find returns the find command.
func Find() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "find",
		Short: "Search for records matching a pattern",
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("not yet implemented")
		},
	}
	addPathFlag(cmd)
	cmd.Flags().String("substr", "", "match records containing this substring")
	cmd.Flags().String("re", "", "match records where a field value matches this regular expression")
	cmd.Flags().String("exact", "", "match records where a field value matches exactly")
	cmd.Flags().String("in", "", "regular expression scoping the search to a sub-path")
	cmd.Flags().Int("limit", 0, "maximum number of records to return")
	cmd.Flags().String("fields", "", "comma-separated list of fields to search (default: all fields)")
	return cmd
}

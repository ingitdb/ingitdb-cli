package commands

import (
	"github.com/dal-go/dalgo/dal"
	"github.com/spf13/cobra"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// Delete returns the delete command group.
func Delete(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	logf func(...any),
) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"d"},
		Short:   "Delete database objects (collection, view, or records)",
	}
	cmd.AddCommand(
		deleteCollection(),
		deleteView(),
		deleteRecords(),
		deleteRecord(homeDir, getWd, readDefinition, newDB, logf),
	)
	return cmd
}


package commands

import (
	"github.com/dal-go/dalgo/dal"
	"github.com/spf13/cobra"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// Update returns the update command group.
func Update(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	logf func(...any),
) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "update",
		Aliases: []string{"u"},
		Short:   "Update database objects",
	}
	cmd.AddCommand(updateRecord(homeDir, getWd, readDefinition, newDB, logf))
	return cmd
}


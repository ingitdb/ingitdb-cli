package commands

import (
	"github.com/dal-go/dalgo/dal"
	"github.com/spf13/cobra"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// Create returns the create command group.
func Create(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	logf func(...any),
) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "create",
		Aliases: []string{"c"},
		Short:   "Create database objects",
	}
	cmd.AddCommand(createRecord(homeDir, getWd, readDefinition, newDB, logf))
	return cmd
}

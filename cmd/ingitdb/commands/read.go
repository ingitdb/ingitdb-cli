package commands

import (
	"github.com/dal-go/dalgo/dal"
	"github.com/spf13/cobra"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// Read returns the read command group.
func Read(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	logf func(...any),
) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "read",
		Aliases: []string{"r"},
		Short:   "Read database objects",
	}
	cmd.AddCommand(
		readRecord(homeDir, getWd, readDefinition, newDB, logf),
		readCollection(homeDir, getWd, readDefinition, logf),
	)
	return cmd
}


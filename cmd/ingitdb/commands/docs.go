package commands

import (
	"github.com/spf13/cobra"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// Docs returns the docs command.
func Docs(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	logf func(...any),
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "docs",
		Short: "Manage documentation",
	}
	cmd.AddCommand(docsUpdate(homeDir, getWd, readDefinition, logf))
	return cmd
}


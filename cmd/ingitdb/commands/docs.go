package commands

import (
	"github.com/urfave/cli/v3"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// Docs returns the docs command.
func Docs(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	logf func(...any),
) *cli.Command {
	return &cli.Command{
		Name:  "docs",
		Usage: "Manage documentation",
		Commands: []*cli.Command{
			docsUpdate(homeDir, getWd, readDefinition, logf),
		},
	}
}

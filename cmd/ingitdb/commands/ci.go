package commands

import (
	"github.com/urfave/cli/v3"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/materializer"
)

// CI returns the ci command.
// Currently it executes materialize; future versions may add CI-specific
// optimisations such as validating and materializing only the collections
// affected by a pull-request diff.
func CI(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	viewBuilder materializer.ViewBuilder,
	logf func(...any),
) *cli.Command {
	mat := Materialize(homeDir, getWd, readDefinition, viewBuilder, logf)
	return &cli.Command{
		Name:   "ci",
		Usage:  "Run CI checks for the database (currently: materialize views)",
		Flags:  mat.Flags,
		Action: mat.Action,
	}
}

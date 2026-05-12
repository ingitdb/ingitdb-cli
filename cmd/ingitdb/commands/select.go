package commands

import (
	"fmt"

	"github.com/dal-go/dalgo/dal"
	"github.com/spf13/cobra"

	"github.com/ingitdb/ingitdb-cli/cmd/ingitdb/commands/sqlflags"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// Select returns the `ingitdb select` command. It queries records from
// a single collection in either single-record mode (--id) or set mode
// (--from with optional --where/--order-by/--fields/--limit/--min-affected).
// Output format defaults to yaml in single-record mode and csv in set
// mode.
func Select(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	logf func(...any),
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "select",
		Short: "Query records from a collection (SQL SELECT)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_ = logf
			id, _ := cmd.Flags().GetString("id")
			from, _ := cmd.Flags().GetString("from")
			mode, err := sqlflags.ResolveMode(id, from)
			if err != nil {
				return err
			}
			switch mode {
			case sqlflags.ModeID:
				return fmt.Errorf("select --id: not yet implemented")
			case sqlflags.ModeFrom:
				return fmt.Errorf("select --from: not yet implemented")
			default:
				return fmt.Errorf("invalid mode")
			}
		},
	}
	addPathFlag(cmd)
	addGitHubFlags(cmd)
	sqlflags.RegisterIDFlag(cmd)
	sqlflags.RegisterFromFlag(cmd)
	sqlflags.RegisterWhereFlag(cmd)
	sqlflags.RegisterOrderByFlag(cmd)
	sqlflags.RegisterFieldsFlag(cmd)
	sqlflags.RegisterMinAffectedFlag(cmd)
	cmd.Flags().Int("limit", 0, "maximum number of records to return (0 = no limit; set mode only)")
	addFormatFlag(cmd, "")
	// Suppress dependency-injection params; they are used by later tasks.
	_, _, _, _, _ = homeDir, getWd, readDefinition, newDB, logf
	return cmd
}

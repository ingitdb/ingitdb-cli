package commands

import (
	"fmt"

	"github.com/dal-go/dalgo/dal"
	"github.com/spf13/cobra"

	"github.com/ingitdb/ingitdb-cli/cmd/ingitdb/commands/sqlflags"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// Update returns the `ingitdb update` command. Two modes inherited
// from sqlflags: single-record (--id) and set (--from + --where|--all).
// Patch operations: --set (repeatable assignment) and --unset
// (comma-separated field list). Shallow patch at the top level.
// --min-affected guards set-mode invocations with all-or-nothing
// semantics.
func Update(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	logf func(...any),
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update records in a collection (SQL UPDATE)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_ = logf
			// Reject shared flags that don't apply to update.
			for _, flag := range []string{"into", "order-by", "fields"} {
				if cmd.Flags().Changed(flag) {
					return fmt.Errorf("--%s is not valid with update", flag)
				}
			}

			id, _ := cmd.Flags().GetString("id")
			from, _ := cmd.Flags().GetString("from")
			mode, err := sqlflags.ResolveMode(id, from)
			if err != nil {
				return err
			}

			// Require at least one of --set or --unset.
			setExprs, _ := cmd.Flags().GetStringArray("set")
			unsetExprs, _ := cmd.Flags().GetStringArray("unset")
			if len(setExprs) == 0 && len(unsetExprs) == 0 {
				return fmt.Errorf("at least one of --set or --unset is required")
			}

			switch mode {
			case sqlflags.ModeID:
				return fmt.Errorf("update --id: not yet implemented")
			case sqlflags.ModeFrom:
				return fmt.Errorf("update --from: not yet implemented")
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
	sqlflags.RegisterSetFlag(cmd)
	sqlflags.RegisterUnsetFlag(cmd)
	sqlflags.RegisterAllFlag(cmd)
	sqlflags.RegisterMinAffectedFlag(cmd)
	// Register the forbidden shared flags so cobra doesn't error on
	// "unknown flag"; we reject them in RunE with our own message.
	sqlflags.RegisterIntoFlag(cmd)
	sqlflags.RegisterOrderByFlag(cmd)
	sqlflags.RegisterFieldsFlag(cmd)
	// Suppress unused DI params; they're used by later tasks.
	_, _, _, _, _ = homeDir, getWd, readDefinition, newDB, logf
	return cmd
}

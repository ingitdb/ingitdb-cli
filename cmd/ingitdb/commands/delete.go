package commands

import (
	"fmt"

	"github.com/dal-go/dalgo/dal"
	"github.com/spf13/cobra"

	"github.com/ingitdb/ingitdb-cli/cmd/ingitdb/commands/sqlflags"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// Delete returns the `ingitdb delete` command. Two modes inherited
// from sqlflags: single-record (--id) and set (--from + --where|--all).
// --min-affected guards set-mode invocations with all-or-nothing
// destructive atomicity: when the matched count is below the
// threshold, NO record is deleted.
//
// This command replaces the legacy `delete record`, `delete records`,
// `delete collection`, and `delete view` subcommands. Per
// cli-sql-verbs Idea: when a new verb's name collides with an old
// top-level command, the legacy parent is removed in the same release.
func Delete(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	logf func(...any),
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete records from a collection (SQL DELETE)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_ = logf
			// Reject shared flags that don't apply to delete.
			for _, flag := range []string{"into", "set", "unset", "order-by", "fields"} {
				if cmd.Flags().Changed(flag) {
					return fmt.Errorf("--%s is not valid with delete", flag)
				}
			}

			id, _ := cmd.Flags().GetString("id")
			from, _ := cmd.Flags().GetString("from")
			mode, err := sqlflags.ResolveMode(id, from)
			if err != nil {
				return err
			}

			switch mode {
			case sqlflags.ModeID:
				return fmt.Errorf("delete --id: not yet implemented")
			case sqlflags.ModeFrom:
				return fmt.Errorf("delete --from: not yet implemented")
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
	sqlflags.RegisterAllFlag(cmd)
	sqlflags.RegisterMinAffectedFlag(cmd)
	// Register the forbidden shared flags so cobra doesn't error on
	// "unknown flag"; we reject them in RunE with our own message.
	sqlflags.RegisterIntoFlag(cmd)
	sqlflags.RegisterSetFlag(cmd)
	sqlflags.RegisterUnsetFlag(cmd)
	sqlflags.RegisterOrderByFlag(cmd)
	sqlflags.RegisterFieldsFlag(cmd)
	// Suppress unused DI params; they're used by later tasks.
	_, _, _, _, _ = homeDir, getWd, readDefinition, newDB, logf
	return cmd
}

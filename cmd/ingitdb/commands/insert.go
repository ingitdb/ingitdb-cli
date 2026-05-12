package commands

import (
	"fmt"
	"io"
	"os"

	"github.com/dal-go/dalgo/dal"
	"github.com/spf13/cobra"

	"github.com/ingitdb/ingitdb-cli/cmd/ingitdb/commands/sqlflags"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// Insert returns the `ingitdb insert` command.
//
// Required: --into=<collection>. Exactly one data source: --data, stdin
// (when not a TTY), --edit (opens $EDITOR), or --empty (key-only record).
// Record key comes from --key or a top-level $id field in the data;
// supplying both with different values is rejected.
func Insert(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	logf func(...any),
	stdin io.Reader,
	isStdinTTY func() bool,
	openEditor func(string) error,
) *cobra.Command {
	if stdin == nil {
		stdin = os.Stdin
	}
	if isStdinTTY == nil {
		isStdinTTY = func() bool { return isFdTTY(os.Stdin) }
	}
	if openEditor == nil {
		openEditor = defaultOpenEditor
	}

	cmd := &cobra.Command{
		Use:   "insert",
		Short: "Insert a new record into a collection (SQL INSERT)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_ = logf
			// Reject shared flags that don't apply to insert.
			for _, flag := range []string{"from", "id", "where", "set", "unset", "all", "min-affected", "order-by", "fields"} {
				if cmd.Flags().Changed(flag) {
					return fmt.Errorf("--%s is not valid with insert (insert uses --into + --key + data source)", flag)
				}
			}
			into, _ := cmd.Flags().GetString("into")
			if into == "" {
				return fmt.Errorf("--into is required")
			}
			return fmt.Errorf("insert: not yet implemented")
		},
	}
	addPathFlag(cmd)
	addGitHubFlags(cmd)
	sqlflags.RegisterIntoFlag(cmd)
	// Insert-specific flags.
	cmd.Flags().String("key", "", "record key (alternative: $id field in --data)")
	cmd.Flags().String("data", "", "record data as YAML or JSON (e.g. '{title: \"Ireland\"}')")
	cmd.Flags().Bool("edit", false, "open $EDITOR with a schema-derived template")
	cmd.Flags().Bool("empty", false, "create the record with only the key, no fields")
	// Register the forbidden shared flags so cobra doesn't error on
	// "unknown flag"; we reject them at RunE time with our own message.
	sqlflags.RegisterFromFlag(cmd)
	sqlflags.RegisterIDFlag(cmd)
	sqlflags.RegisterWhereFlag(cmd)
	sqlflags.RegisterSetFlag(cmd)
	sqlflags.RegisterUnsetFlag(cmd)
	sqlflags.RegisterAllFlag(cmd)
	sqlflags.RegisterMinAffectedFlag(cmd)
	sqlflags.RegisterOrderByFlag(cmd)
	sqlflags.RegisterFieldsFlag(cmd)
	// Suppress unused DI params; they're used by later tasks.
	_, _, _, _, _, _, _, _ = homeDir, getWd, readDefinition, newDB, stdin, isStdinTTY, openEditor, logf
	return cmd
}

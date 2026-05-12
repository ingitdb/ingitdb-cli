package commands

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/dal-go/dalgo/dal"
	"github.com/spf13/cobra"

	"github.com/ingitdb/ingitdb-cli/cmd/ingitdb/commands/sqlflags"
	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ingitdb"
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
			ctx := cmd.Context()

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

			// Resolve target collection.
			ictx, err := resolveInsertContext(ctx, cmd, into, homeDir, getWd, readDefinition, newDB)
			if err != nil {
				return err
			}

			// Read data from whichever source the user supplied.
			data, err := readInsertData(cmd, stdin, isStdinTTY, openEditor, ictx.colDef)
			if err != nil {
				return err
			}

			// Resolve the record key (added in Task 4); for now use --key only.
			recordKey, _ := cmd.Flags().GetString("key")
			if recordKey == "" {
				return fmt.Errorf("--key is required (Task 4 will add $id-in-data fallback)")
			}

			// Insert the record (collision check added in Task 5).
			key := dal.NewKeyWithID(ictx.colDef.ID, recordKey)
			record := dal.NewRecordWithData(key, data)
			err = ictx.db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
				return tx.Insert(ctx, record)
			})
			if err != nil {
				return err
			}
			// Materialize local views if applicable.
			return buildLocalViews(ctx, recordContext{
				db:      ictx.db,
				colDef:  ictx.colDef,
				dirPath: ictx.dirPath,
				def:     ictx.def,
			})
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
	return cmd
}

// readInsertData reads record content from exactly one data source
// (--data, stdin, --edit, --empty) and returns the parsed map.
// Mutual exclusion is enforced: more than one source supplied = error.
// Zero sources (and TTY stdin) = error.
func readInsertData(
	cmd *cobra.Command,
	stdin io.Reader,
	isStdinTTY func() bool,
	openEditor func(string) error,
	colDef *ingitdb.CollectionDef,
) (map[string]any, error) {
	dataStr, _ := cmd.Flags().GetString("data")
	editFlag, _ := cmd.Flags().GetBool("edit")
	emptyFlag, _ := cmd.Flags().GetBool("empty")
	stdinHasContent := !isStdinTTY()

	// Count active sources to enforce mutual exclusion.
	active := 0
	if dataStr != "" {
		active++
	}
	if editFlag {
		active++
	}
	if emptyFlag {
		active++
	}
	if stdinHasContent {
		active++
	}
	if active > 1 {
		return nil, fmt.Errorf("at most one data source allowed (--data, stdin, --edit, --empty); got %d", active)
	}

	switch {
	case dataStr != "":
		data, err := dalgo2ingitdb.ParseRecordContentForCollection([]byte(dataStr), colDef)
		if err != nil {
			return nil, fmt.Errorf("failed to parse --data: %w", err)
		}
		return data, nil
	case editFlag:
		data, noChanges, err := runWithEditor(colDef, openEditor)
		if err != nil {
			return nil, err
		}
		if noChanges {
			return nil, fmt.Errorf("editor template was not edited; record not created")
		}
		return data, nil
	case emptyFlag:
		return map[string]any{}, nil
	case stdinHasContent:
		content, err := io.ReadAll(stdin)
		if err != nil {
			return nil, fmt.Errorf("failed to read stdin: %w", err)
		}
		return dalgo2ingitdb.ParseRecordContentForCollection(content, colDef)
	default:
		return nil, fmt.Errorf("no data source — use --data, pipe stdin, --edit, or --empty")
	}
}

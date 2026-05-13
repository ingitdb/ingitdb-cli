package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

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

			// Batch mode validation: --format must be one of the four supported
			// stream formats. Empty means single-record mode.
			format, _ := cmd.Flags().GetString("format")
			if cmd.Flags().Changed("format") {
				switch format {
				case "jsonl", "yaml", "ingr", "csv":
					// valid; batch mode active
				default:
					return fmt.Errorf("invalid --format=%q; supported batch formats are: jsonl, yaml, ingr, csv (markdown is supported as a storage format only, not as a stream format)", format)
				}
			}

			batchMode := cmd.Flags().Changed("format")

			// Reject shared flags that don't apply to insert.
			for _, flag := range []string{"from", "id", "where", "set", "unset", "all", "min-affected", "order-by", "fields"} {
				if !cmd.Flags().Changed(flag) {
					continue
				}
				// Carve-out: --fields is permitted in batch-CSV mode (see req:batch-csv-fields-flag).
				if flag == "fields" && batchMode && format == "csv" {
					continue
				}
				return fmt.Errorf("--%s is not valid with insert (insert uses --into + --key + data source)", flag)
			}

			// In batch mode, reject single-record flags.
			if batchMode {
				for _, f := range []string{"data", "edit", "empty", "key"} {
					if cmd.Flags().Changed(f) {
						return fmt.Errorf("--%s is not valid in batch mode (--format=%s); batch mode reads multi-record stream from stdin and resolves keys from each record's $id", f, format)
					}
				}
			}

			// --key-column is only valid in batch-CSV mode.
			if cmd.Flags().Changed("key-column") {
				if !batchMode || format != "csv" {
					return fmt.Errorf("--key-column is valid only with --format=csv")
				}
			}

			// --fields is rejected outside single-record (existing logic above)
			// AND outside batch-CSV. The shared-flag loop rejects it in
			// single-record mode; here we add the batch-non-csv guard.
			if batchMode && format != "csv" && cmd.Flags().Changed("fields") {
				return fmt.Errorf("--fields is valid only with --format=csv (used to override the CSV header row or drive parsing when no header is present)")
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

			// Batch mode validation: reject TTY stdin before attempting to read data.
			if batchMode {
				if isStdinTTY() {
					return fmt.Errorf("batch mode (--format=%s) requires piped stdin; refusing to read from a TTY", format)
				}
				keyColumn, _ := cmd.Flags().GetString("key-column")
				var fields []string
				// Only honor --fields when explicitly set; the shared
				// sqlflags.RegisterFieldsFlag default is "*", which would
				// otherwise be misread as a one-column header override.
				if cmd.Flags().Changed("fields") {
					fieldsCSV, _ := cmd.Flags().GetString("fields")
					fields = strings.Split(fieldsCSV, ",")
					for i := range fields {
						fields[i] = strings.TrimSpace(fields[i])
					}
				}
				return runBatchInsert(ctx, format, keyColumn, fields, stdin, ictx, cmd.ErrOrStderr())
			}

			// Read data from whichever source the user supplied.
			data, err := readInsertData(cmd, stdin, isStdinTTY, openEditor, ictx.colDef)
			if err != nil {
				return err
			}

			// Resolve the record key. Either --key, a top-level $id in
			// the data, or both consistently supplied.
			recordKey, data, err := resolveInsertKey(cmd, data)
			if err != nil {
				return err
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
	addRemoteFlags(cmd)
	sqlflags.RegisterIntoFlag(cmd)
	// Insert-specific flags.
	cmd.Flags().String("key", "", "record key (alternative: $id field in --data)")
	cmd.Flags().String("data", "", "record data as YAML or JSON (e.g. '{title: \"Ireland\"}')")
	cmd.Flags().Bool("edit", false, "open $EDITOR with a schema-derived template")
	cmd.Flags().Bool("empty", false, "create the record with only the key, no fields")
	cmd.Flags().String("format", "", "batch mode: stream format (jsonl, yaml, ingr, csv); when set, reads multi-record stream from stdin")
	cmd.Flags().String("key-column", "", "batch-csv mode only: column name to use as the record key (overrides $id/id auto-resolution)")
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

// resolveInsertKey returns the record key derived from --key and/or a
// top-level $id field in data, plus the data map with $id stripped.
// Rules:
//   - --key only: use --key.
//   - $id only: use $id; remove $id from data.
//   - both, equal: use the value; remove $id from data.
//   - both, different: reject naming both values.
//   - neither: reject.
//
// The returned data map is always the cleaned form (no $id key) even
// when --key alone is supplied, so the downstream Insert always sees
// the same shape.
func resolveInsertKey(cmd *cobra.Command, data map[string]any) (string, map[string]any, error) {
	flagKey, _ := cmd.Flags().GetString("key")

	var dataKey string
	dataHasID := false
	if v, ok := data["$id"]; ok {
		dataHasID = true
		dataKey = fmt.Sprintf("%v", v)
	}

	// Always strip $id from data — it is metadata, not a stored field.
	if dataHasID {
		delete(data, "$id")
	}

	switch {
	case flagKey != "" && dataHasID:
		if flagKey != dataKey {
			return "", nil, fmt.Errorf("--key=%q conflicts with $id=%q in data; supply one or make them match", flagKey, dataKey)
		}
		return flagKey, data, nil
	case flagKey != "":
		return flagKey, data, nil
	case dataHasID:
		if dataKey == "" {
			return "", nil, fmt.Errorf("$id in data is empty")
		}
		return dataKey, data, nil
	default:
		return "", nil, fmt.Errorf("record key required: supply --key or include a $id field in the data")
	}
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

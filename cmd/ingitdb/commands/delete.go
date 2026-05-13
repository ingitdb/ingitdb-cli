package commands

import (
	"context"
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
				return runDeleteByID(cmd.Context(), cmd, id, homeDir, getWd, readDefinition, newDB)
			case sqlflags.ModeFrom:
				return runDeleteFromSet(cmd.Context(), cmd, from, homeDir, getWd, readDefinition, newDB)
			default:
				return fmt.Errorf("invalid mode")
			}
		},
	}
	addPathFlag(cmd)
	addRemoteFlags(cmd)
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
	return cmd
}

// runDeleteFromSet handles --from set mode: fetch all records, apply
// WHERE filter (or --all), then delete each matching record in a
// single transaction.
func runDeleteFromSet(
	ctx context.Context,
	cmd *cobra.Command,
	from string,
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
) error {
	// Mutual exclusion: --where XOR --all.
	whereExprs, _ := cmd.Flags().GetStringArray("where")
	allFlag, _ := cmd.Flags().GetBool("all")
	if len(whereExprs) > 0 && allFlag {
		return fmt.Errorf("--where and --all are mutually exclusive")
	}
	if len(whereExprs) == 0 && !allFlag {
		return fmt.Errorf("set mode requires one of --where or --all")
	}

	// Parse --where conditions.
	conds := make([]sqlflags.Condition, 0, len(whereExprs))
	for _, e := range whereExprs {
		c, parseErr := sqlflags.ParseWhere(e)
		if parseErr != nil {
			return fmt.Errorf("invalid --where %q: %w", e, parseErr)
		}
		conds = append(conds, c)
	}

	// Resolve collection (local or GitHub).
	ictx, err := resolveInsertContext(ctx, cmd, from, homeDir, getWd, readDefinition, newDB)
	if err != nil {
		return err
	}

	// Read-only pass: collect matching keys.
	qb := dal.NewQueryBuilder(dal.From(dal.NewRootCollectionRef(from, "")))
	q := qb.SelectIntoRecord(func() dal.Record {
		k := dal.NewKeyWithID(from, "")
		return dal.NewRecordWithData(k, map[string]any{})
	})
	var matchedKeys []string
	err = ictx.db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		reader, qerr := tx.ExecuteQueryToRecordsReader(ctx, q)
		if qerr != nil {
			return qerr
		}
		defer func() { _ = reader.Close() }()
		for {
			rec, nextErr := reader.Next()
			if nextErr != nil {
				break
			}
			recKey := fmt.Sprintf("%v", rec.Key().ID)
			if !allFlag {
				data, ok := rec.Data().(map[string]any)
				if !ok {
					continue
				}
				match, evalErr := evalAllWhere(data, recKey, conds)
				if evalErr != nil {
					return evalErr
				}
				if !match {
					continue
				}
			}
			matchedKeys = append(matchedKeys, recKey)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	// --min-affected pre-flight check. If the matched count is below
	// the threshold, fail BEFORE opening the write transaction.
	// Destructive atomicity: no record is deleted when below
	// threshold.
	if n, supplied, mErr := sqlflags.MinAffectedFromCmd(cmd); mErr != nil {
		return mErr
	} else if supplied && len(matchedKeys) < n {
		return fmt.Errorf("matched %d records, required at least %d", len(matchedKeys), n)
	}

	// Read-write pass: delete each matching key.
	err = ictx.db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		for _, k := range matchedKeys {
			key := dal.NewKeyWithID(from, k)
			if delErr := tx.Delete(ctx, key); delErr != nil {
				return delErr
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Materialize local views (no-op when source is GitHub).
	if ictx.dirPath == "" {
		return nil
	}
	return buildLocalViews(ctx, recordContext{
		db:      ictx.db,
		colDef:  ictx.colDef,
		dirPath: ictx.dirPath,
		def:     ictx.def,
	})
}

// runDeleteByID handles --id mode: fetch one record to confirm it
// exists, then delete it inside RunReadwriteTransaction. Returns
// non-zero if the record doesn't exist.
func runDeleteByID(
	ctx context.Context,
	cmd *cobra.Command,
	id string,
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
) error {
	// Reject set-mode-only flags in single-record mode.
	if cmd.Flags().Changed("where") {
		return fmt.Errorf("--where is invalid with --id (single-record mode)")
	}
	if cmd.Flags().Changed("all") {
		return fmt.Errorf("--all is invalid with --id (single-record mode)")
	}
	if cmd.Flags().Changed("min-affected") {
		return fmt.Errorf("--min-affected is invalid with --id (single-record mode)")
	}

	rctx, err := resolveRecordContext(ctx, cmd, id, homeDir, getWd, readDefinition, newDB)
	if err != nil {
		return err
	}

	key := dal.NewKeyWithID(rctx.colDef.ID, rctx.recordKey)
	err = rctx.db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		// Pre-flight existence check. tx.Delete may or may not error
		// on missing keys depending on the backend; we want an
		// explicit user-facing diagnostic.
		probe := dal.NewRecordWithData(key, map[string]any{})
		if getErr := tx.Get(ctx, probe); getErr != nil {
			return getErr
		}
		if !probe.Exists() {
			return fmt.Errorf("record not found: %s", id)
		}
		return tx.Delete(ctx, key)
	})
	if err != nil {
		return err
	}
	return buildLocalViews(ctx, rctx)
}

package commands

import (
	"context"
	"fmt"

	"github.com/dal-go/dalgo/dal"
	"github.com/spf13/cobra"

	"github.com/ingitdb/ingitdb-cli/cmd/ingitdb/commands/sqlflags"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// patchTarget pairs a record key with its data map, used as the unit
// of work for set-mode patching.
type patchTarget struct {
	key  string
	data map[string]any
}

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
				return runUpdateByID(cmd.Context(), cmd, id, setExprs, unsetExprs, homeDir, getWd, readDefinition, newDB)
			case sqlflags.ModeFrom:
				return runUpdateFromSet(cmd.Context(), cmd, from, setExprs, unsetExprs, homeDir, getWd, readDefinition, newDB)
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
	sqlflags.RegisterSetFlag(cmd)
	sqlflags.RegisterUnsetFlag(cmd)
	sqlflags.RegisterAllFlag(cmd)
	sqlflags.RegisterMinAffectedFlag(cmd)
	// Register the forbidden shared flags so cobra doesn't error on
	// "unknown flag"; we reject them in RunE with our own message.
	sqlflags.RegisterIntoFlag(cmd)
	sqlflags.RegisterOrderByFlag(cmd)
	sqlflags.RegisterFieldsFlag(cmd)
	return cmd
}

// runUpdateByID handles --id mode: fetch one record, apply the patch
// (set + unset), write back. Returns non-zero if the record doesn't
// exist. Shallow patch semantics: fields not named in --set/--unset
// are preserved unchanged.
func runUpdateByID(
	ctx context.Context,
	cmd *cobra.Command,
	id string,
	setExprs []string,
	unsetExprs []string,
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

	// Parse --set assignments and --unset field lists.
	sets, err := parseSetExprs(setExprs)
	if err != nil {
		return err
	}
	unsets, err := parseUnsetExprs(unsetExprs)
	if err != nil {
		return err
	}
	if err := sqlflags.RejectSetUnsetSameField(sets, unsets); err != nil {
		return err
	}

	rctx, err := resolveRecordContext(ctx, cmd, id, homeDir, getWd, readDefinition, newDB)
	if err != nil {
		return err
	}

	key := dal.NewKeyWithID(rctx.colDef.ID, rctx.recordKey)
	err = rctx.db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		data := map[string]any{}
		record := dal.NewRecordWithData(key, data)
		if getErr := tx.Get(ctx, record); getErr != nil {
			return getErr
		}
		if !record.Exists() {
			return fmt.Errorf("record not found: %s", id)
		}
		applyPatch(data, sets, unsets)
		return tx.Set(ctx, record)
	})
	if err != nil {
		return err
	}
	return buildLocalViews(ctx, rctx)
}

// parseSetExprs converts the raw --set strings into Assignment values
// via sqlflags.ParseSet. Returns the first parse error.
func parseSetExprs(exprs []string) ([]sqlflags.Assignment, error) {
	out := make([]sqlflags.Assignment, 0, len(exprs))
	for _, e := range exprs {
		a, err := sqlflags.ParseSet(e)
		if err != nil {
			return nil, fmt.Errorf("invalid --set %q: %w", e, err)
		}
		out = append(out, a)
	}
	return out, nil
}

// parseUnsetExprs accumulates all --unset entries into a flat field
// list. Each --unset value may be comma-separated; the flag itself is
// repeatable.
func parseUnsetExprs(exprs []string) ([]string, error) {
	var out []string
	for _, e := range exprs {
		fields, err := sqlflags.ParseUnset(e)
		if err != nil {
			return nil, fmt.Errorf("invalid --unset %q: %w", e, err)
		}
		out = append(out, fields...)
	}
	return out, nil
}

// applyPatch applies the shallow patch (set + unset) to a record's
// data map in place. Fields not named in either list are preserved.
func applyPatch(data map[string]any, sets []sqlflags.Assignment, unsets []string) {
	for _, a := range sets {
		data[a.Field] = a.Value
	}
	for _, f := range unsets {
		delete(data, f)
	}
}

// runUpdateFromSet handles --from set mode: fetch all records, apply
// WHERE filter (or --all), apply patch to each matching record in a
// single transaction.
func runUpdateFromSet(
	ctx context.Context,
	cmd *cobra.Command,
	from string,
	setExprs []string,
	unsetExprs []string,
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

	// Parse patches.
	sets, err := parseSetExprs(setExprs)
	if err != nil {
		return err
	}
	unsets, err := parseUnsetExprs(unsetExprs)
	if err != nil {
		return err
	}
	if err := sqlflags.RejectSetUnsetSameField(sets, unsets); err != nil {
		return err
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

	// Fetch matching records via a read-only pass.
	qb := dal.NewQueryBuilder(dal.From(dal.NewRootCollectionRef(from, "")))
	q := qb.SelectIntoRecord(func() dal.Record {
		k := dal.NewKeyWithID(from, "")
		return dal.NewRecordWithData(k, map[string]any{})
	})
	var matches []patchTarget
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
			data, ok := rec.Data().(map[string]any)
			if !ok {
				continue
			}
			recKey := fmt.Sprintf("%v", rec.Key().ID)
			if !allFlag {
				matched, evalErr := evalAllWhere(data, recKey, conds)
				if evalErr != nil {
					return evalErr
				}
				if !matched {
					continue
				}
			}
			matches = append(matches, patchTarget{key: recKey, data: data})
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	// --min-affected pre-flight check. If the matched count is below
	// the threshold, fail BEFORE opening the write transaction.
	if n, supplied, mErr := sqlflags.MinAffectedFromCmd(cmd); mErr != nil {
		return mErr
	} else if supplied && len(matches) < n {
		return fmt.Errorf("matched %d records, required at least %d", len(matches), n)
	}

	// Apply patches in a single read-write transaction.
	err = ictx.db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		for _, m := range matches {
			applyPatch(m.data, sets, unsets)
			key := dal.NewKeyWithID(from, m.key)
			record := dal.NewRecordWithData(key, m.data)
			if setErr := tx.Set(ctx, record); setErr != nil {
				return setErr
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Materialize local views.
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

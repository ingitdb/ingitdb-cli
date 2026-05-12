package commands

import (
	"context"
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
				return runUpdateByID(cmd.Context(), cmd, id, setExprs, unsetExprs, homeDir, getWd, readDefinition, newDB)
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

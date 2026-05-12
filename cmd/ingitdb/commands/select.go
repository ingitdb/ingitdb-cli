package commands

import (
	"context"
	"fmt"
	"strings"

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
			ctx := cmd.Context()

			id, _ := cmd.Flags().GetString("id")
			from, _ := cmd.Flags().GetString("from")
			mode, err := sqlflags.ResolveMode(id, from)
			if err != nil {
				return err
			}

			fieldsRaw, _ := cmd.Flags().GetString("fields")
			fields, parseErr := sqlflags.ParseFields(fieldsRaw)
			if parseErr != nil {
				return parseErr
			}
			format, _ := cmd.Flags().GetString("format")
			format = strings.ToLower(format)

			switch mode {
			case sqlflags.ModeID:
				return runSelectByID(ctx, cmd, id, fields, format, homeDir, getWd, readDefinition, newDB)
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
	return cmd
}

// runSelectByID handles --id mode: fetch one record, project fields,
// emit a bare mapping / object.
func runSelectByID(
	ctx context.Context,
	cmd *cobra.Command,
	id string,
	fields []string,
	format string,
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
) error {
	// Reject set-mode flags per shared-cli-flags applicability rules.
	whereExprs, _ := cmd.Flags().GetStringArray("where")
	orderByVal, _ := cmd.Flags().GetString("order-by")
	limitVal, _ := cmd.Flags().GetInt("limit")
	if n, supplied, mErr := sqlflags.MinAffectedFromCmd(cmd); mErr != nil {
		return mErr
	} else if supplied {
		_ = n
		return fmt.Errorf("--min-affected is invalid with --id (single-record mode)")
	}
	if len(whereExprs) > 0 {
		return fmt.Errorf("--where is invalid with --id (single-record mode); use --from for set queries")
	}
	if orderByVal != "" {
		return fmt.Errorf("--order-by is invalid with --id (single-record mode)")
	}
	if limitVal != 0 {
		return fmt.Errorf("--limit is invalid with --id (single-record mode)")
	}

	rctx, err := resolveRecordContext(ctx, cmd, id, homeDir, getWd, readDefinition, newDB)
	if err != nil {
		return err
	}

	data := map[string]any{}
	key := dal.NewKeyWithID(rctx.colDef.ID, rctx.recordKey)
	record := dal.NewRecordWithData(key, data)
	err = rctx.db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		return tx.Get(ctx, record)
	})
	if err != nil {
		return err
	}
	if !record.Exists() {
		return fmt.Errorf("record not found: %s", id)
	}
	projected := projectRecord(data, rctx.recordKey, fields)
	if format == "" {
		format = "yaml"
	}
	return writeSingleRecord(cmd.OutOrStdout(), projected, format, fields)
}

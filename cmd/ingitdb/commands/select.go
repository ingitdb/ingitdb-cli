package commands

import (
	"context"
	"fmt"
	"io"
	"sort"
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
				return runSelectFromSet(ctx, cmd, from, fields, format, homeDir, getWd, readDefinition, newDB)
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

// runSelectFromSet handles --from set mode: fetch every record from
// the collection, apply WHERE conditions, project fields, and emit
// the result. Supports both local (--path) and remote (--remote) sources.
func runSelectFromSet(
	ctx context.Context,
	cmd *cobra.Command,
	from string,
	fields []string,
	format string,
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
) error {
	remoteValue, _ := cmd.Flags().GetString("remote")
	pathValue, _ := cmd.Flags().GetString("path")

	if remoteValue != "" && pathValue != "" {
		return fmt.Errorf("--path and --remote are mutually exclusive")
	}

	if remoteValue != "" {
		spec, err := resolveRemoteFromFlags(cmd, remoteValue)
		if err != nil {
			return err
		}
		def, readErr := readRemoteDefinitionForCollection(ctx, spec, from)
		if readErr != nil {
			return fmt.Errorf("failed to read remote definition: %w", readErr)
		}
		cfg := newGitHubConfig(spec, remoteToken(cmd, spec.Host))
		db, dbErr := gitHubDBFactory.NewGitHubDBWithDef(cfg, def)
		if dbErr != nil {
			return fmt.Errorf("failed to open remote database: %w", dbErr)
		}
		return runSelectFromSetWithDB(ctx, cmd, from, fields, format, db)
	}

	dirPath, err := resolveDBPath(cmd, homeDir, getWd)
	if err != nil {
		return err
	}
	def, err := readDefinition(dirPath)
	if err != nil {
		return fmt.Errorf("failed to read database definition: %w", err)
	}
	if _, ok := def.Collections[from]; !ok {
		return fmt.Errorf("collection %q not found in definition", from)
	}
	db, err := newDB(dirPath, def)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	return runSelectFromSetWithDB(ctx, cmd, from, fields, format, db)
}

// runSelectFromSetWithDB executes the set-mode query against a pre-opened DB.
// Both local and GitHub paths call this after resolving their respective DBs.
func runSelectFromSetWithDB(
	ctx context.Context,
	cmd *cobra.Command,
	from string,
	fields []string,
	format string,
	db dal.DB,
) error {
	whereExprs, _ := cmd.Flags().GetStringArray("where")
	conds := make([]sqlflags.Condition, 0, len(whereExprs))
	for _, expr := range whereExprs {
		c, err := sqlflags.ParseWhere(expr)
		if err != nil {
			return fmt.Errorf("invalid --where %q: %w", expr, err)
		}
		conds = append(conds, c)
	}

	qb := dal.NewQueryBuilder(dal.From(dal.NewRootCollectionRef(from, "")))
	q := qb.SelectIntoRecord(func() dal.Record {
		key := dal.NewKeyWithID(from, "")
		return dal.NewRecordWithData(key, map[string]any{})
	})

	var rows []map[string]any
	err := db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
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
			match, evalErr := evalAllWhere(data, recKey, conds)
			if evalErr != nil {
				return evalErr
			}
			if !match {
				continue
			}
			rows = append(rows, projectRecord(data, recKey, fields))
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	// --order-by: sort the result slice after filtering.
	orderRaw, _ := cmd.Flags().GetString("order-by")
	orders, orderErr := sqlflags.ParseOrderBy(orderRaw)
	if orderErr != nil {
		return orderErr
	}
	if len(orders) > 0 {
		sort.SliceStable(rows, func(i, j int) bool {
			for _, o := range orders {
				cmp := compareValues(rows[i][o.Field], rows[j][o.Field])
				if cmp == 0 {
					continue
				}
				if o.Descending {
					return cmp > 0
				}
				return cmp < 0
			}
			return false
		})
	}

	// --min-affected pre-flight check (after WHERE, before --limit).
	if n, supplied, mErr := sqlflags.MinAffectedFromCmd(cmd); mErr != nil {
		return mErr
	} else if supplied && len(rows) < n {
		return fmt.Errorf("matched %d records, required at least %d", len(rows), n)
	}

	// --limit: cap the result count after ordering.
	limitVal, _ := cmd.Flags().GetInt("limit")
	if limitVal < 0 {
		return fmt.Errorf("--limit must be >= 0, got %d", limitVal)
	}
	if limitVal > 0 && len(rows) > limitVal {
		rows = rows[:limitVal]
	}

	if format == "" {
		format = "csv"
	}
	return writeSetMode(cmd.OutOrStdout(), rows, format, fields)
}

// writeSetMode is the set-mode output dispatcher. Empty rows still
// produce format-appropriate output (csv header / [] for json/yaml /
// md header / INGR header + "# 0 records" footer).
func writeSetMode(w io.Writer, rows []map[string]any, format string, columns []string) error {
	if rows == nil {
		rows = []map[string]any{}
	}
	switch format {
	case "csv":
		return writeCSV(w, rows, columns)
	case "json":
		if len(rows) == 0 {
			_, err := fmt.Fprintln(w, "[]")
			return err
		}
		return writeJSON(w, rows)
	case "yaml", "yml":
		if len(rows) == 0 {
			_, err := fmt.Fprintln(w, "[]")
			return err
		}
		return writeYAML(w, rows)
	case "md", "markdown":
		return writeMarkdown(w, rows, columns)
	case "ingr":
		return writeINGR(w, rows, columns)
	default:
		return fmt.Errorf("unknown format %q, use csv, json, yaml, md, or ingr", format)
	}
}

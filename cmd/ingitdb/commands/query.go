package commands

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/dal-go/dalgo/dal"
	"github.com/spf13/cobra"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// Query returns the query command.
func Query(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	logf func(...any),
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "query",
		Short: "Query records from a collection",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_ = logf

			ctx := cmd.Context()

			// 1. Resolve DB path.
			dirPath, err := resolveDBPath(cmd, homeDir, getWd)
			if err != nil {
				return err
			}

			// 2. Read definition.
			def, err := readDefinition(dirPath)
			if err != nil {
				return fmt.Errorf("failed to read database definition: %w", err)
			}

			// 3. Validate collection exists.
			colID, _ := cmd.Flags().GetString("collection")
			colDef, ok := def.Collections[colID]
			if !ok {
				return fmt.Errorf("collection %q not found in definition", colID)
			}
			_ = colDef

			// 4. Parse --fields.
			fieldsVal, _ := cmd.Flags().GetString("fields")
			fields := parseFields(fieldsVal)

			// 5. Parse --where conditions.
			whereExprs, _ := cmd.Flags().GetStringArray("where")
			conditions := make([]dal.Condition, 0, len(whereExprs))
			for _, expr := range whereExprs {
				cond, parseErr := parseWhereExpr(expr)
				if parseErr != nil {
					return fmt.Errorf("invalid --where %q: %w", expr, parseErr)
				}
				conditions = append(conditions, cond)
			}

			// 6. Parse --order-by.
			orderByVal, _ := cmd.Flags().GetString("order-by")
			orderExprs, err := parseOrderBy(orderByVal)
			if err != nil {
				return fmt.Errorf("invalid --order-by: %w", err)
			}

			// 7. Build query.
			qb := dal.NewQueryBuilder(dal.From(dal.NewRootCollectionRef(colID, "")))
			qb.Where(conditions...).OrderBy(orderExprs...)
			q := qb.SelectIntoRecord(func() dal.Record {
				key := dal.NewKeyWithID(colID, "")
				return dal.NewRecordWithData(key, map[string]any{})
			})

			// 8. Open DB.
			db, err := newDB(dirPath, def)
			if err != nil {
				return fmt.Errorf("failed to open database: %w", err)
			}

			// 9. Execute query.
			var reader dal.RecordsReader
			err = db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
				reader, err = tx.ExecuteQueryToRecordsReader(ctx, q)
				return err
			})
			if err != nil {
				return fmt.Errorf("query failed: %w", err)
			}
			defer func() { _ = reader.Close() }()

			// 10. Collect results.
			var rows []map[string]any
			for {
				rec, nextErr := reader.Next()
				if nextErr != nil {
					break
				}
				data := rec.Data().(map[string]any)
				recKey := fmt.Sprintf("%v", rec.Key().ID)
				rows = append(rows, projectRecord(data, recKey, fields))
			}

			// Determine column order for tabular formats.
			var columns []string
			if len(fields) > 0 {
				columns = fields
			}

			// 11. Write output.
			format, _ := cmd.Flags().GetString("format")
			format = strings.ToLower(format)
			switch format {
			case "csv", "":
				return writeCSV(os.Stdout, rows, columns)
			case "json":
				return writeJSON(os.Stdout, rows)
			case "yaml", "yml":
				return writeYAML(os.Stdout, rows)
			case "md", "markdown":
				return writeMarkdown(os.Stdout, rows, columns)
			default:
				return fmt.Errorf("unknown format %q, use csv, json, yaml, or md", format)
			}
		},
	}
	addPathFlag(cmd)
	addCollectionFlag(cmd, true)
	cmd.Flags().StringP("fields", "f", "*", `fields to select: * = all, $id = record key, field1,field2 = specific fields`)
	cmd.Flags().StringArrayP("where", "w", nil, `filter expression (repeatable): field>value, field==value, etc.`)
	cmd.Flags().String("order-by", "", `comma-separated fields; prefix '-' = descending`)
	addFormatFlag(cmd, "csv")
	return cmd
}


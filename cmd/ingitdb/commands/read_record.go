package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/dal-go/dalgo/dal"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func readRecord(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	logf func(...any),
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "record",
		Short: "Read a single record from a collection",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_ = logf
			ctx := cmd.Context()
			id, _ := cmd.Flags().GetString("id")
			githubVal, _ := cmd.Flags().GetString("github")
			pathVal, _ := cmd.Flags().GetString("path")
			if githubVal != "" && pathVal != "" {
				return fmt.Errorf("--path with --github is not supported yet")
			}
			rctx, err := resolveRecordContext(ctx, cmd, id, homeDir, getWd, readDefinition, newDB)
			if err != nil {
				return err
			}
			key := dal.NewKeyWithID(rctx.colDef.ID, rctx.recordKey)
			data := map[string]any{}
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
			format, _ := cmd.Flags().GetString("format")
			switch format {
			case "yaml", "yml":
				out, marshalErr := yaml.Marshal(data)
				if marshalErr != nil {
					return fmt.Errorf("failed to marshal output as YAML: %w", marshalErr)
				}
				_, _ = os.Stdout.Write(out)
			case "json":
				out, marshalErr := json.MarshalIndent(data, "", "  ")
				if marshalErr != nil {
					return fmt.Errorf("failed to marshal output as JSON: %w", marshalErr)
				}
				_, _ = fmt.Fprintf(os.Stdout, "%s\n", out)
			default:
				return fmt.Errorf("unknown format %q, use yaml or json", format)
			}
			return nil
		},
	}
	addPathFlag(cmd)
	addGitHubFlags(cmd)
	cmd.Flags().String("id", "", "record ID in the format collection/path/key (e.g. countries/ie)")
	_ = cmd.MarkFlagRequired("id")
	addFormatFlag(cmd, "yaml")
	return cmd
}


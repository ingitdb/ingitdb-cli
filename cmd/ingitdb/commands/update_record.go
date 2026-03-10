package commands

import (
	"context"
	"fmt"
	"maps"

	"github.com/dal-go/dalgo/dal"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func updateRecord(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	logf func(...any),
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "record",
		Short: "Update fields of an existing record",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_ = logf
			ctx := cmd.Context()
			id, _ := cmd.Flags().GetString("id")
			setStr, _ := cmd.Flags().GetString("set")
			rctx, err := resolveRecordContext(ctx, cmd, id, homeDir, getWd, readDefinition, newDB)
			if err != nil {
				return err
			}
			var patch map[string]any
			if unmarshalErr := yaml.Unmarshal([]byte(setStr), &patch); unmarshalErr != nil {
				return fmt.Errorf("failed to parse --set: %w", unmarshalErr)
			}
			key := dal.NewKeyWithID(rctx.colDef.ID, rctx.recordKey)
			err = rctx.db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
				data := map[string]any{}
				record := dal.NewRecordWithData(key, data)
				getErr := tx.Get(ctx, record)
				if getErr != nil {
					return getErr
				}
				if !record.Exists() {
					return fmt.Errorf("record not found: %s", id)
				}
				maps.Copy(data, patch)
				return tx.Set(ctx, record)
			})
			if err != nil {
				return err
			}
			return buildLocalViews(ctx, rctx)
		},
	}
	addPathFlag(cmd)
	addGitHubFlags(cmd)
	cmd.Flags().String("id", "", "record ID in the format collection/path/key (e.g. todo.countries/ie)")
	_ = cmd.MarkFlagRequired("id")
	cmd.Flags().String("set", "", "fields to update as YAML or JSON (e.g. '{title: \"Ireland, Republic of\"}')")
	_ = cmd.MarkFlagRequired("set")
	return cmd
}


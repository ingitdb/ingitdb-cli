package commands

import (
	"context"
	"fmt"

	"github.com/dal-go/dalgo/dal"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func createRecord(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	logf func(...any),
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "record",
		Short: "Create a new record in a collection",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			id, _ := cmd.Flags().GetString("id")
			dataStr, _ := cmd.Flags().GetString("data")
			rctx, err := resolveRecordContext(ctx, cmd, id, homeDir, getWd, readDefinition, newDB)
			if err != nil {
				return err
			}
			if rctx.dirPath != "" {
				logf("inGitDB db path: ", rctx.dirPath)
			}
			var data map[string]any
			if unmarshalErr := yaml.Unmarshal([]byte(dataStr), &data); unmarshalErr != nil {
				return fmt.Errorf("failed to parse --data: %w", unmarshalErr)
			}
			key := dal.NewKeyWithID(rctx.colDef.ID, rctx.recordKey)
			record := dal.NewRecordWithData(key, data)
			err = rctx.db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
				return tx.Insert(ctx, record)
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
	cmd.Flags().String("data", "", "record data as YAML or JSON (e.g. '{title: \"Ireland\"}')")
	_ = cmd.MarkFlagRequired("data")
	return cmd
}

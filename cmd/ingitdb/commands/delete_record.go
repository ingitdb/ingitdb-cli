package commands

import (
	"context"

	"github.com/dal-go/dalgo/dal"
	"github.com/spf13/cobra"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func deleteRecord(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	logf func(...any),
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "record",
		Short: "Delete a single record by its ID",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_ = logf
			ctx := cmd.Context()
			id, _ := cmd.Flags().GetString("id")
			rctx, err := resolveRecordContext(ctx, cmd, id, homeDir, getWd, readDefinition, newDB)
			if err != nil {
				return err
			}
			key := dal.NewKeyWithID(rctx.colDef.ID, rctx.recordKey)
			err = rctx.db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
				return tx.Delete(ctx, key)
			})
			if err != nil {
				return err
			}
			return buildLocalViews(ctx, rctx)
		},
	}
	addPathFlag(cmd)
	addGitHubFlags(cmd)
	cmd.Flags().String("id", "", "record ID in the format collection/path/key (e.g. todo.tags/ie)")
	_ = cmd.MarkFlagRequired("id")
	return cmd
}

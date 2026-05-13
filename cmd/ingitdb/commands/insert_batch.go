package commands

import (
	"context"
	"fmt"
	"io"

	"github.com/dal-go/dalgo/dal"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ingitdb"
)

// runBatchInsert implements --format batch mode. It reads stdin per the
// selected stream format, inserts all records inside a single
// transaction, and materializes local views once after commit.
//
// On any pre-commit failure (parse, missing key, schema violation,
// collision, write error, commit error), the batch is rolled back and
// the error is returned. On post-commit view-materialization failure,
// the inserted records remain on disk and a distinct error is returned.
func runBatchInsert(
	ctx context.Context,
	format string,
	keyColumn string,
	fields []string,
	stdin io.Reader,
	ictx insertContext,
	stderr io.Writer,
) error {
	records, err := parseBatchStream(format, keyColumn, fields, stdin)
	if err != nil {
		return err
	}
	if len(records) == 0 {
		_, _ = fmt.Fprintln(stderr, "0 records inserted")
		return nil
	}
	// Pre-commit intra-batch duplicate check.
	err = rejectIntraBatchDuplicates(records)
	if err != nil {
		return err
	}
	// Atomic insert. Any individual failure aborts the whole batch.
	commitErr := ictx.db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		for _, rec := range records {
			key := dal.NewKeyWithID(ictx.colDef.ID, rec.Key)
			r := dal.NewRecordWithData(key, rec.Data)
			insertErr := tx.Insert(ctx, r)
			if insertErr != nil {
				return fmt.Errorf("record at position %d (key=%q): %w", rec.Position, rec.Key, insertErr)
			}
		}
		return nil
	})
	if commitErr != nil {
		return commitErr
	}
	// Post-commit: materialize local views once. Failures here cannot
	// be rolled back — records are on disk.
	rctx := recordContext{
		db:      ictx.db,
		colDef:  ictx.colDef,
		dirPath: ictx.dirPath,
		def:     ictx.def,
	}
	viewErr := buildLocalViews(ctx, rctx)
	if viewErr != nil {
		return fmt.Errorf("records inserted but view materialization failed: %w", viewErr)
	}
	_, _ = fmt.Fprintf(stderr, "%d records inserted\n", len(records))
	return nil
}

// parseBatchStream routes to the format-specific parser.
func parseBatchStream(format, keyColumn string, fields []string, r io.Reader) ([]dalgo2ingitdb.ParsedRecord, error) {
	_, _ = keyColumn, fields
	switch format {
	case "jsonl":
		return dalgo2ingitdb.ParseBatchJSONL(r)
	case "yaml", "ingr", "csv":
		return nil, fmt.Errorf("batch format %q not yet implemented", format)
	default:
		return nil, fmt.Errorf("unsupported batch format %q", format)
	}
}

// rejectIntraBatchDuplicates returns an error if two records in the
// batch share a resolved key.
func rejectIntraBatchDuplicates(records []dalgo2ingitdb.ParsedRecord) error {
	seen := make(map[string]int, len(records))
	for _, rec := range records {
		if prev, dup := seen[rec.Key]; dup {
			return fmt.Errorf("duplicate key %q in batch: positions %d and %d", rec.Key, prev, rec.Position)
		}
		seen[rec.Key] = rec.Position
	}
	return nil
}

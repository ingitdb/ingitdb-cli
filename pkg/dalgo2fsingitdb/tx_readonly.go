package dalgo2fsingitdb

import (
	"context"
	"fmt"
	"maps"

	"github.com/dal-go/dalgo/dal"
	"github.com/dal-go/dalgo/recordset"
	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

var _ dal.ReadTransaction = (*readonlyTx)(nil)

type readonlyTx struct {
	db localDB
}

func (r readonlyTx) Options() dal.TransactionOptions {
	//TODO implement me
	panic("implement me")
}

func (r readonlyTx) Get(ctx context.Context, record dal.Record) error {
	_ = ctx
	if r.db.def == nil {
		return fmt.Errorf("definition is required: use NewLocalDBWithDef")
	}
	key := record.Key()
	collectionID := key.Collection()
	colDef, ok := r.db.def.Collections[collectionID]
	if !ok {
		return fmt.Errorf("collection %q not found in definition", collectionID)
	}
	recordKey := fmt.Sprintf("%v", key.ID)
	path := resolveRecordPath(colDef, recordKey)
	switch colDef.RecordFile.RecordType {
	case ingitdb.SingleRecord:
		var (
			data  map[string]any
			found bool
			err   error
		)
		if colDef.RecordFile.Format == ingitdb.RecordFormatMarkdown {
			data, found, err = readMarkdownRecord(path, colDef)
		} else {
			data, found, err = readRecordFromFile(path, colDef.RecordFile.Format)
		}
		if err != nil {
			return err
		}
		if !found {
			record.SetError(dal.ErrRecordNotFound)
			return nil
		}
		record.SetError(nil)
		target := record.Data().(map[string]any)
		maps.Copy(target, data)
	case ingitdb.MapOfRecords:
		allRecords, found, err := readMapOfRecordsFile(path, colDef.RecordFile.Format)
		if err != nil {
			return err
		}
		if !found {
			record.SetError(dal.ErrRecordNotFound)
			return nil
		}
		recordData, exists := allRecords[recordKey]
		if !exists {
			record.SetError(dal.ErrRecordNotFound)
			return nil
		}
		record.SetError(nil)
		target := record.Data().(map[string]any)
		maps.Copy(target, dalgo2ingitdb.ApplyLocaleToRead(recordData, colDef.Columns))
	default:
		return fmt.Errorf("not yet implemented for record type %q", colDef.RecordFile.RecordType)
	}
	return nil
}

func (r readonlyTx) Exists(ctx context.Context, key *dal.Key) (bool, error) {
	//TODO implement me
	panic("implement me")
}

func (r readonlyTx) GetMulti(ctx context.Context, records []dal.Record) error {
	//TODO implement me
	panic("implement me")
}

func (r readonlyTx) ExecuteQueryToRecordsReader(ctx context.Context, query dal.Query) (dal.RecordsReader, error) {
	return executeQueryToRecordsReader(ctx, r, query)
}

// ExecuteQueryToRecordsetReader executes a structured query against a single
// collection and returns a recordset-shaped reader. Stored columns carry each
// record's value; computed (formula) columns are registered via
// recordset.NewComputedColumn bound to a Starlark-backed evaluator, so computed
// values resolve lazily when a consumer reads them rather than being baked in.
func (r readonlyTx) ExecuteQueryToRecordsetReader(_ context.Context, query dal.Query, _ ...recordset.Option) (dal.RecordsetReader, error) {
	colDef, err := collectionFromQuery(r.db.def, query)
	if err != nil {
		return nil, err
	}
	stored, err := readAllStoredRecords(colDef)
	if err != nil {
		return nil, err
	}
	return dalgo2ingitdb.NewRecordsetReader(dalgo2ingitdb.BuildRecordset(colDef, stored)), nil
}

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
		data, found, err := readRecordFromFile(path, colDef.RecordFile.Format)
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
	//TODO implement me
	panic("implement me")
}

func (r readonlyTx) ExecuteQueryToRecordsetReader(ctx context.Context, query dal.Query, options ...recordset.Option) (dal.RecordsetReader, error) {
	//TODO implement me
	panic("implement me")
}

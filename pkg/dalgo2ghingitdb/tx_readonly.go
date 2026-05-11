package dalgo2ghingitdb

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
	db *githubDB
}

func (r readonlyTx) Options() dal.TransactionOptions {
	return nil
}

func (r readonlyTx) Get(ctx context.Context, record dal.Record) error {
	if r.db.def == nil {
		return fmt.Errorf("definition is required")
	}
	key := record.Key()
	collectionID := key.Collection()
	colDef, ok := r.db.def.Collections[collectionID]
	if !ok {
		return fmt.Errorf("collection %q not found in definition", collectionID)
	}
	recordKey := fmt.Sprintf("%v", key.ID)
	recordPath := resolveRecordPath(colDef, recordKey)
	switch colDef.RecordFile.RecordType {
	case ingitdb.SingleRecord:
		data, found, err := r.readSingleRecord(ctx, recordPath, colDef)
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
		data, found, err := r.readRecordFromMap(ctx, recordPath, recordKey, colDef)
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
	default:
		return fmt.Errorf("record type %q is not supported", colDef.RecordFile.RecordType)
	}
	return nil
}

func (r readonlyTx) Exists(ctx context.Context, key *dal.Key) (bool, error) {
	_, _ = ctx, key
	return false, fmt.Errorf("exists is not implemented by %s", DatabaseID)
}

func (r readonlyTx) GetMulti(ctx context.Context, records []dal.Record) error {
	_, _ = ctx, records
	return fmt.Errorf("getmulti is not implemented by %s", DatabaseID)
}

func (r readonlyTx) ExecuteQueryToRecordsReader(ctx context.Context, query dal.Query) (dal.RecordsReader, error) {
	_, _ = ctx, query
	return nil, fmt.Errorf("query is not implemented by %s", DatabaseID)
}

func (r readonlyTx) ExecuteQueryToRecordsetReader(ctx context.Context, query dal.Query, options ...recordset.Option) (dal.RecordsetReader, error) {
	_, _, _ = ctx, query, options
	return nil, fmt.Errorf("query is not implemented by %s", DatabaseID)
}

func (r readonlyTx) readSingleRecord(ctx context.Context, recordPath string, colDef *ingitdb.CollectionDef) (map[string]any, bool, error) {
	content, found, err := r.db.fileReader.ReadFile(ctx, recordPath)
	if err != nil {
		return nil, false, err
	}
	if !found {
		return nil, false, nil
	}
	data, parseErr := dalgo2ingitdb.ParseRecordContentForCollection(content, colDef)
	if parseErr != nil {
		return nil, false, parseErr
	}
	return data, true, nil
}

func (r readonlyTx) readRecordFromMap(ctx context.Context, recordPath, recordKey string, colDef *ingitdb.CollectionDef) (map[string]any, bool, error) {
	content, found, err := r.db.fileReader.ReadFile(ctx, recordPath)
	if err != nil {
		return nil, false, err
	}
	if !found {
		return nil, false, nil
	}
	allRecords, parseErr := dalgo2ingitdb.ParseMapOfRecordsContent(content, colDef.RecordFile.Format)
	if parseErr != nil {
		return nil, false, parseErr
	}
	recordData, exists := allRecords[recordKey]
	if !exists {
		return nil, false, nil
	}
	localizedData := dalgo2ingitdb.ApplyLocaleToRead(recordData, colDef.Columns)
	return localizedData, true, nil
}

func (r readonlyTx) resolveCollection(key *dal.Key) (*ingitdb.CollectionDef, string, error) {
	if r.db.def == nil {
		return nil, "", fmt.Errorf("definition is required")
	}
	collectionID := key.Collection()
	colDef, ok := r.db.def.Collections[collectionID]
	if !ok {
		return nil, "", fmt.Errorf("collection %q not found in definition", collectionID)
	}
	if colDef.RecordFile == nil {
		return nil, "", fmt.Errorf("collection %q has no record file", collectionID)
	}
	recordKey := fmt.Sprintf("%v", key.ID)
	return colDef, recordKey, nil
}

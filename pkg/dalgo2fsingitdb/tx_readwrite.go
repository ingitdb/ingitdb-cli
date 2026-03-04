package dalgo2fsingitdb

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/dal-go/dalgo/dal"
	"github.com/dal-go/dalgo/update"
	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

var _ dal.ReadwriteTransaction = (*readwriteTx)(nil)

type readwriteTx struct {
	readonlyTx
}

func (r readwriteTx) ID() string {
	//TODO implement me
	panic("implement me")
}

func (r readwriteTx) Set(ctx context.Context, record dal.Record) error {
	_ = ctx
	colDef, recordKey, err := r.resolveCollection(record.Key())
	if err != nil {
		return err
	}
	record.SetError(nil)
	path := resolveRecordPath(colDef, recordKey)
	data := record.Data().(map[string]any)
	switch colDef.RecordFile.RecordType {
	case ingitdb.MapOfRecords:
		allRecords, _, err := readMapOfRecordsFile(path, colDef.RecordFile.Format)
		if err != nil {
			return err
		}
		if allRecords == nil {
			allRecords = make(map[string]map[string]any)
		}
		allRecords[recordKey] = dalgo2ingitdb.ApplyLocaleToWrite(data, colDef.Columns)
		return writeMapOfRecordsFile(path, colDef.RecordFile.Format, allRecords)
	default:
		return writeRecordToFile(path, colDef.RecordFile.Format, data)
	}
}

func (r readwriteTx) SetMulti(ctx context.Context, records []dal.Record) error {
	//TODO implement me
	panic("implement me")
}

func (r readwriteTx) Delete(ctx context.Context, key *dal.Key) error {
	_ = ctx
	colDef, recordKey, err := r.resolveCollection(key)
	if err != nil {
		return err
	}
	path := resolveRecordPath(colDef, recordKey)
	switch colDef.RecordFile.RecordType {
	case ingitdb.MapOfRecords:
		allRecords, found, err := readMapOfRecordsFile(path, colDef.RecordFile.Format)
		if err != nil {
			return err
		}
		if !found {
			return dal.ErrRecordNotFound
		}
		if _, exists := allRecords[recordKey]; !exists {
			return dal.ErrRecordNotFound
		}
		delete(allRecords, recordKey)
		return writeMapOfRecordsFile(path, colDef.RecordFile.Format, allRecords)
	default:
		return deleteRecordFile(path)
	}
}

func (r readwriteTx) DeleteMulti(ctx context.Context, keys []*dal.Key) error {
	//TODO implement me
	panic("implement me")
}

func (r readwriteTx) Update(ctx context.Context, key *dal.Key, updates []update.Update, preconditions ...dal.Precondition) error {
	//TODO implement me
	panic("implement me")
}

func (r readwriteTx) UpdateRecord(ctx context.Context, record dal.Record, updates []update.Update, preconditions ...dal.Precondition) error {
	//TODO implement me
	panic("implement me")
}

func (r readwriteTx) UpdateMulti(ctx context.Context, keys []*dal.Key, updates []update.Update, preconditions ...dal.Precondition) error {
	//TODO implement me
	panic("implement me")
}

func (r readwriteTx) Insert(ctx context.Context, record dal.Record, opts ...dal.InsertOption) error {
	_, _ = ctx, opts
	colDef, recordKey, err := r.resolveCollection(record.Key())
	if err != nil {
		return err
	}
	path := resolveRecordPath(colDef, recordKey)
	switch colDef.RecordFile.RecordType {
	case ingitdb.MapOfRecords:
		allRecords, _, err := readMapOfRecordsFile(path, colDef.RecordFile.Format)
		if err != nil {
			return err
		}
		if allRecords == nil {
			allRecords = make(map[string]map[string]any)
		}
		if _, exists := allRecords[recordKey]; exists {
			return fmt.Errorf("record already exists: %s in %s", recordKey, path)
		}
		record.SetError(nil)
		data := record.Data().(map[string]any)
		allRecords[recordKey] = dalgo2ingitdb.ApplyLocaleToWrite(data, colDef.Columns)
		return writeMapOfRecordsFile(path, colDef.RecordFile.Format, allRecords)
	default:
		_, statErr := os.Stat(path)
		if statErr == nil {
			return fmt.Errorf("record already exists: %s", path)
		}
		if !errors.Is(statErr, os.ErrNotExist) {
			return fmt.Errorf("failed to check file %s: %w", path, statErr)
		}
		record.SetError(nil)
		data := record.Data().(map[string]any)
		return writeRecordToFile(path, colDef.RecordFile.Format, data)
	}
}

func (r readwriteTx) resolveCollection(key *dal.Key) (*ingitdb.CollectionDef, string, error) {
	if r.db.def == nil {
		return nil, "", fmt.Errorf("definition is required: use NewLocalDBWithDef")
	}
	collectionID := key.Collection()
	colDef, ok := r.db.def.Collections[collectionID]
	if !ok {
		return nil, "", fmt.Errorf("collection %q not found in definition", collectionID)
	}
	if colDef.RecordFile == nil {
		return nil, "", fmt.Errorf("collection %q has no record_file definition", collectionID)
	}
	recordKey := fmt.Sprintf("%v", key.ID)
	return colDef, recordKey, nil
}

func (r readwriteTx) InsertMulti(ctx context.Context, records []dal.Record, opts ...dal.InsertOption) error {
	//TODO implement me
	panic("implement me")
}

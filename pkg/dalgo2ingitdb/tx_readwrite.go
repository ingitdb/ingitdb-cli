package dalgo2ingitdb

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/dal-go/dalgo/dal"
	"github.com/dal-go/dalgo/update"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// readwriteTx is the read-write transaction handle. It embeds readonlyTx
// to inherit Get / Exists / GetMulti / query support; write methods are
// implemented directly using exclusive locks on the affected record file.
type readwriteTx struct {
	readonlyTx
}

// ID returns the transaction identifier. inGitDB does not use a per-tx
// identifier, so this returns an empty string.
func (r readwriteTx) ID() string { return "" }

// Set stores a record, overwriting any existing value. For MapOfRecords
// the surrounding file is read, the record is inserted/updated in place,
// and the whole file is re-written.
func (r readwriteTx) Set(_ context.Context, record dal.Record) error {
	colDef, recordKey, err := r.resolveCollection(record.Key())
	if err != nil {
		return err
	}
	record.SetError(nil)
	data := record.Data().(map[string]any)
	path := resolveRecordPath(colDef, recordKey)
	switch colDef.RecordFile.RecordType {
	case ingitdb.SingleRecord:
		if err := writeSingleRecordFile(path, colDef, data); err != nil {
			return err
		}
	case ingitdb.MapOfRecords:
		allRecords, _, readErr := readMapOfRecordsFile(path, colDef.RecordFile.Format)
		if readErr != nil {
			return readErr
		}
		if allRecords == nil {
			allRecords = make(map[string]map[string]any)
		}
		allRecords[recordKey] = ApplyLocaleToWrite(data, colDef.Columns)
		if err := writeMapOfRecordsFile(path, colDef, allRecords); err != nil {
			return err
		}
	default:
		return fmt.Errorf("dalgo2ingitdb: Set not implemented for record type %q", colDef.RecordFile.RecordType)
	}
	record.SetError(nil)
	return nil
}

// SetMulti applies Set to each record sequentially.
func (r readwriteTx) SetMulti(ctx context.Context, records []dal.Record) error {
	for _, rec := range records {
		if err := r.Set(ctx, rec); err != nil {
			return err
		}
	}
	return nil
}

// Insert stores a record if it does not already exist. Returns an error
// when the key is already taken.
func (r readwriteTx) Insert(_ context.Context, record dal.Record, _ ...dal.InsertOption) error {
	colDef, recordKey, err := r.resolveCollection(record.Key())
	if err != nil {
		return err
	}
	record.SetError(nil)
	data := record.Data().(map[string]any)
	path := resolveRecordPath(colDef, recordKey)
	switch colDef.RecordFile.RecordType {
	case ingitdb.SingleRecord:
		if _, statErr := os.Stat(path); statErr == nil {
			return fmt.Errorf("dalgo2ingitdb: record already exists: %s", path)
		} else if !errors.Is(statErr, fs.ErrNotExist) {
			return fmt.Errorf("dalgo2ingitdb: stat %s: %w", path, statErr)
		}
		if err := writeSingleRecordFile(path, colDef, data); err != nil {
			return err
		}
	case ingitdb.MapOfRecords:
		allRecords, _, readErr := readMapOfRecordsFile(path, colDef.RecordFile.Format)
		if readErr != nil {
			return readErr
		}
		if allRecords == nil {
			allRecords = make(map[string]map[string]any)
		}
		if _, exists := allRecords[recordKey]; exists {
			return fmt.Errorf("dalgo2ingitdb: record already exists: %s in %s", recordKey, path)
		}
		allRecords[recordKey] = ApplyLocaleToWrite(data, colDef.Columns)
		if err := writeMapOfRecordsFile(path, colDef, allRecords); err != nil {
			return err
		}
	default:
		return fmt.Errorf("dalgo2ingitdb: Insert not implemented for record type %q", colDef.RecordFile.RecordType)
	}
	record.SetError(nil)
	return nil
}

// InsertMulti applies Insert to each record sequentially.
func (r readwriteTx) InsertMulti(ctx context.Context, records []dal.Record, opts ...dal.InsertOption) error {
	for _, rec := range records {
		if err := r.Insert(ctx, rec, opts...); err != nil {
			return err
		}
	}
	return nil
}

// Delete removes a record. Returns dal.ErrRecordNotFound when the record
// does not exist.
func (r readwriteTx) Delete(_ context.Context, key *dal.Key) error {
	colDef, recordKey, err := r.resolveCollection(key)
	if err != nil {
		return err
	}
	path := resolveRecordPath(colDef, recordKey)
	switch colDef.RecordFile.RecordType {
	case ingitdb.SingleRecord:
		return deleteSingleRecordFile(path)
	case ingitdb.MapOfRecords:
		allRecords, found, readErr := readMapOfRecordsFile(path, colDef.RecordFile.Format)
		if readErr != nil {
			return readErr
		}
		if !found {
			return dal.ErrRecordNotFound
		}
		if _, exists := allRecords[recordKey]; !exists {
			return dal.ErrRecordNotFound
		}
		delete(allRecords, recordKey)
		return writeMapOfRecordsFile(path, colDef, allRecords)
	default:
		return fmt.Errorf("dalgo2ingitdb: Delete not implemented for record type %q", colDef.RecordFile.RecordType)
	}
}

// DeleteMulti applies Delete to each key sequentially.
func (r readwriteTx) DeleteMulti(ctx context.Context, keys []*dal.Key) error {
	for _, k := range keys {
		if err := r.Delete(ctx, k); err != nil {
			return err
		}
	}
	return nil
}

// Update applies field-level updates by reading the record, mutating it
// in memory, then writing it back. Preconditions and nested field paths
// are not supported in this driver yet.
func (r readwriteTx) Update(ctx context.Context, key *dal.Key, updates []update.Update, preconditions ...dal.Precondition) error {
	if len(preconditions) > 0 {
		return fmt.Errorf("dalgo2ingitdb: Update preconditions are not supported")
	}
	rec := dal.NewRecordWithData(key, map[string]any{})
	if err := r.Get(ctx, rec); err != nil {
		return err
	}
	if err := rec.Error(); err != nil {
		return err
	}
	data := rec.Data().(map[string]any)
	if err := applyUpdates(data, updates); err != nil {
		return err
	}
	return r.Set(ctx, dal.NewRecordWithData(key, data))
}

// UpdateRecord applies updates to the given record's in-memory data
// then persists it. The record's Data is mutated in place.
func (r readwriteTx) UpdateRecord(ctx context.Context, record dal.Record, updates []update.Update, preconditions ...dal.Precondition) error {
	if len(preconditions) > 0 {
		return fmt.Errorf("dalgo2ingitdb: UpdateRecord preconditions are not supported")
	}
	data := record.Data().(map[string]any)
	if err := applyUpdates(data, updates); err != nil {
		return err
	}
	return r.Set(ctx, record)
}

// UpdateMulti applies Update to each key sequentially.
func (r readwriteTx) UpdateMulti(ctx context.Context, keys []*dal.Key, updates []update.Update, preconditions ...dal.Precondition) error {
	for _, k := range keys {
		if err := r.Update(ctx, k, updates, preconditions...); err != nil {
			return err
		}
	}
	return nil
}

// applyUpdates mutates data according to updates. Only single-segment
// FieldName paths are honoured; nested FieldPath updates return an error.
func applyUpdates(data map[string]any, updates []update.Update) error {
	for _, u := range updates {
		name := u.FieldName()
		if name == "" {
			path := u.FieldPath()
			if len(path) != 1 {
				return fmt.Errorf("dalgo2ingitdb: nested field paths are not supported (%v)", path)
			}
			name = path[0]
		}
		data[name] = u.Value()
	}
	return nil
}

var _ dal.ReadwriteTransaction = (*readwriteTx)(nil)

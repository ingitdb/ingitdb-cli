package dalgo2ingitdb

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/dal-go/dalgo/dal"
	dalrecord "github.com/dal-go/dalgo/record"
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
	key := record.Key()
	colDef, recordKey, err := r.resolveCollection(key)
	if err != nil {
		return err
	}
	record.SetError(nil)
	data, err := dalrecord.DataToMap(record.Data())
	if err != nil {
		return err
	}
	collectionID := key.Collection()
	if err := r.validateNoStoredComputedValues(collectionID, colDef, recordKey, data); err != nil {
		return err
	}
	if err := r.validateWriteForeignKeys("Set", collectionID, colDef, data); err != nil {
		return err
	}
	path := resolveRecordPath(colDef, recordKey)
	switch colDef.RecordFile.RecordType {
	case ingitdb.SingleRecord:
		if err := writeSingleRecordFile(path, colDef, data); err != nil {
			return err
		}
	case ingitdb.MapOfRecords:
		allRecords, readErr := readMapOfRecordsFile(path, colDef.RecordFile.Format)
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
	key := record.Key()
	colDef, recordKey, err := r.resolveCollection(key)
	if err != nil {
		return err
	}
	record.SetError(nil)
	data, err := dalrecord.DataToMap(record.Data())
	if err != nil {
		return err
	}
	collectionID := key.Collection()
	if err := r.validateNoStoredComputedValues(collectionID, colDef, recordKey, data); err != nil {
		return err
	}
	if err := r.validateWriteForeignKeys("Insert", collectionID, colDef, data); err != nil {
		return err
	}
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
		allRecords, readErr := readMapOfRecordsFile(path, colDef.RecordFile.Format)
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

// Delete removes a record. Deleting a record that does not exist is a no-op
// (returns nil), per the idempotent Delete contract shared by dalgo adapters.
func (r readwriteTx) Delete(_ context.Context, key *dal.Key) error {
	colDef, recordKey, err := r.resolveCollection(key)
	if err != nil {
		return err
	}
	parentExists, err := foreignKeyTargetExists(colDef, recordKey)
	if err != nil {
		return fmt.Errorf("dalgo2ingitdb: Delete foreign key lookup failed for parent collection %q key %q: %w", colDef.ID, recordKey, err)
	}
	if parentExists {
		collectionID := key.Collection()
		if err := r.validateDeleteForeignKeys(collectionID, recordKey); err != nil {
			return err
		}
	}
	path := resolveRecordPath(colDef, recordKey)
	switch colDef.RecordFile.RecordType {
	case ingitdb.SingleRecord:
		return deleteSingleRecordFile(path)
	case ingitdb.MapOfRecords:
		allRecords, readErr := readMapOfRecordsFile(path, colDef.RecordFile.Format)
		if readErr != nil {
			return readErr
		}
		if _, exists := allRecords[recordKey]; !exists {
			return nil // idempotent: deleting a missing record is a no-op
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
	// Get returns dal.ErrRecordNotFound for a missing record, so updating a
	// non-existent record fails here rather than silently creating it.
	if err := r.Get(ctx, rec); err != nil {
		return err
	}
	data := rec.Data().(map[string]any)
	if err := applyUpdates(data, updates); err != nil {
		return err
	}
	colDef, _, err := r.resolveCollection(key)
	if err != nil {
		return err
	}
	collectionID := key.Collection()
	if err := r.validateWriteForeignKeys("Update", collectionID, colDef, data); err != nil {
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
	data, err := dalrecord.DataToMap(record.Data())
	if err != nil {
		return err
	}
	if err := applyUpdates(data, updates); err != nil {
		return err
	}
	key := record.Key()
	colDef, _, err := r.resolveCollection(key)
	if err != nil {
		return err
	}
	collectionID := key.Collection()
	if err := r.validateWriteForeignKeys("UpdateRecord", collectionID, colDef, data); err != nil {
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

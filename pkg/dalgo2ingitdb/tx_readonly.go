package dalgo2ingitdb

import (
	"context"
	"fmt"
	"maps"

	"github.com/dal-go/dalgo/dal"
	"github.com/dal-go/dalgo/recordset"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// readonlyTx is a snapshot-based read transaction. The Definition is
// loaded once when the transaction starts; collection lookups within the
// transaction use that snapshot. Individual file reads still take a shared
// lock so concurrent writers are blocked while the file is being parsed.
type readonlyTx struct {
	db   *Database
	def  *ingitdb.Definition
	opts dal.TransactionOptions
}

// Options returns the options the transaction was created with. inGitDB
// ignores isolation level today.
func (r readonlyTx) Options() dal.TransactionOptions { return r.opts }

// Get loads a single record. SingleRecord and MapOfRecords layouts are
// supported. A missing record sets dal.ErrRecordNotFound on the record
// and returns nil.
func (r readonlyTx) Get(_ context.Context, record dal.Record) error {
	colDef, recordKey, err := r.resolveCollection(record.Key())
	if err != nil {
		return err
	}
	path := resolveRecordPath(colDef, recordKey)
	switch colDef.RecordFile.RecordType {
	case ingitdb.SingleRecord:
		data, found, readErr := readSingleRecordFile(path, colDef)
		if readErr != nil {
			return readErr
		}
		if !found {
			record.SetError(dal.ErrRecordNotFound)
			return nil
		}
		record.SetError(nil)
		target := record.Data().(map[string]any)
		maps.Copy(target, ApplyLocaleToRead(data, colDef.Columns))
		return nil
	case ingitdb.MapOfRecords:
		allRecords, found, readErr := readMapOfRecordsFile(path, colDef.RecordFile.Format)
		if readErr != nil {
			return readErr
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
		maps.Copy(target, ApplyLocaleToRead(recordData, colDef.Columns))
		return nil
	default:
		return fmt.Errorf("dalgo2ingitdb: Get not implemented for record type %q", colDef.RecordFile.RecordType)
	}
}

// Exists reports whether the record identified by key is present on disk.
func (r readonlyTx) Exists(_ context.Context, key *dal.Key) (bool, error) {
	colDef, recordKey, err := r.resolveCollection(key)
	if err != nil {
		return false, err
	}
	path := resolveRecordPath(colDef, recordKey)
	switch colDef.RecordFile.RecordType {
	case ingitdb.SingleRecord:
		data, found, readErr := readSingleRecordFile(path, colDef)
		if readErr != nil {
			return false, readErr
		}
		_ = data
		return found, nil
	case ingitdb.MapOfRecords:
		allRecords, found, readErr := readMapOfRecordsFile(path, colDef.RecordFile.Format)
		if readErr != nil {
			return false, readErr
		}
		if !found {
			return false, nil
		}
		_, exists := allRecords[recordKey]
		return exists, nil
	default:
		return false, fmt.Errorf("dalgo2ingitdb: Exists not implemented for record type %q", colDef.RecordFile.RecordType)
	}
}

// GetMulti loads each record by calling Get; a single failure aborts the
// batch and is returned to the caller. Per-record ErrRecordNotFound is
// reported via record.SetError, not as a method-level error — matching
// the convention used by dal's reference drivers.
func (r readonlyTx) GetMulti(ctx context.Context, records []dal.Record) error {
	for _, rec := range records {
		if err := r.Get(ctx, rec); err != nil {
			return err
		}
	}
	return nil
}

// ExecuteQueryToRecordsReader executes a structured query against a
// single collection in memory. WHERE / ORDER BY / LIMIT are applied after
// loading every record in the collection.
func (r readonlyTx) ExecuteQueryToRecordsReader(ctx context.Context, query dal.Query) (dal.RecordsReader, error) {
	return executeQueryToRecordsReader(ctx, r, query)
}

// ExecuteQueryToRecordsetReader is not implemented yet. Callers that need
// recordset-shaped output should fall back to ExecuteQueryToRecordsReader.
func (r readonlyTx) ExecuteQueryToRecordsetReader(_ context.Context, _ dal.Query, _ ...recordset.Option) (dal.RecordsetReader, error) {
	return nil, dal.ErrNotSupported
}

// resolveCollection looks up the collection definition for a key and
// renders the record key as a string.
func (r readonlyTx) resolveCollection(key *dal.Key) (*ingitdb.CollectionDef, string, error) {
	if r.def == nil {
		return nil, "", fmt.Errorf("dalgo2ingitdb: transaction has no loaded definition")
	}
	if key == nil {
		return nil, "", fmt.Errorf("dalgo2ingitdb: key is nil")
	}
	collectionID := key.Collection()
	colDef, ok := r.def.Collections[collectionID]
	if !ok {
		return nil, "", fmt.Errorf("dalgo2ingitdb: collection %q not found in definition", collectionID)
	}
	if colDef.RecordFile == nil {
		return nil, "", fmt.Errorf("dalgo2ingitdb: collection %q has no record_file definition", collectionID)
	}
	recordKey := fmt.Sprintf("%v", key.ID)
	return colDef, recordKey, nil
}

var _ dal.ReadTransaction = (*readonlyTx)(nil)

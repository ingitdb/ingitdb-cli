package dalgo2ghingitdb

import (
	"context"
	"fmt"

	"github.com/dal-go/dalgo/dal"
	"github.com/dal-go/dalgo/update"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// BatchingGitHubDB wraps a githubDB so that RunReadwriteTransaction buffers
// every tx.Set / tx.Insert / tx.Delete call inside the worker, then emits
// exactly one commit via the Git Data API when the worker returns nil.
//
// Reads (Get, RunReadonlyTransaction, ExecuteQuery*) delegate to the
// underlying githubDB unchanged — they read the pre-tx state from remote and
// do not observe buffered writes (set-mode callers fetch matches in a
// separate read-only pass before opening the write tx, so this limitation
// does not affect them).
//
// Use this in place of the per-file github DB when an operation may touch
// multiple records — `update --from --where`, `delete --from --where`,
// `update --from --all`, `delete --from --all`. Single-record operations
// already produce one commit through the Contents API and should keep
// using the plain githubDB.
type BatchingGitHubDB struct {
	*githubDB
	commitMessage string
	writer        *TreeWriter
}

// NewBatchingGitHubDB builds a BatchingGitHubDB for the given Config + def.
// commitMessage is the message used when flushing buffered changes; callers
// supply something human-readable like "ingitdb: update countries (batch)".
func NewBatchingGitHubDB(cfg Config, def *ingitdb.Definition, commitMessage string) (*BatchingGitHubDB, error) {
	if def == nil {
		return nil, fmt.Errorf("definition is required")
	}
	if commitMessage == "" {
		return nil, fmt.Errorf("commit message is required")
	}
	inner, err := NewGitHubDBWithDef(cfg, def)
	if err != nil {
		return nil, err
	}
	concrete, ok := inner.(*githubDB)
	if !ok { // untestable: NewGitHubDBWithDef always returns *githubDB
		return nil, fmt.Errorf("internal error: expected *githubDB")
	}
	writer, err := NewTreeWriter(cfg)
	if err != nil {
		return nil, err
	}
	return &BatchingGitHubDB{
		githubDB:      concrete,
		commitMessage: commitMessage,
		writer:        writer,
	}, nil
}

// RunReadwriteTransaction overrides githubDB.RunReadwriteTransaction with a
// batching variant. Every Set / Insert / Delete inside f is buffered; when f
// returns nil, all buffered changes are flushed to GitHub as one commit. If
// f returns an error, no changes are committed (the remote is untouched).
func (db *BatchingGitHubDB) RunReadwriteTransaction(ctx context.Context, f dal.RWTxWorker, options ...dal.TransactionOption) error {
	_ = options
	tx := &batchingTx{
		readonlyTx:    readonlyTx{db: db.githubDB},
		bufferedFiles: make(map[string]TreeChange),
		workingMaps:   make(map[string]map[string]map[string]any),
		mapColDefs:    make(map[string]*ingitdb.CollectionDef),
		mapLoaded:     make(map[string]bool),
	}
	if err := f(ctx, tx); err != nil {
		return err
	}
	changes, err := tx.flushChanges()
	if err != nil {
		return err
	}
	if len(changes) == 0 {
		return nil // nothing was buffered; no commit needed
	}
	_, err = db.writer.CommitChanges(ctx, db.commitMessage, changes)
	return err
}

// Compile-time check: BatchingGitHubDB satisfies dal.DB. The embedded
// *githubDB supplies every method except the overridden
// RunReadwriteTransaction.
var _ dal.DB = (*BatchingGitHubDB)(nil)

// batchingTx implements dal.ReadwriteTransaction by buffering all writes.
//
// SingleRecord collections: each Set / Insert encodes the new record
// content and stores a TreeChange in bufferedFiles; Delete stores a
// TreeChange with nil Content (deletion).
//
// MapOfRecords collections: the in-flight state of each map file lives in
// workingMaps. On first touch we read the current remote state via
// ensureMapLoaded; subsequent Set / Insert / Delete modify the in-memory
// map. At flush time, every modified working map is re-encoded into one
// TreeChange.
type batchingTx struct {
	readonlyTx
	bufferedFiles map[string]TreeChange
	workingMaps   map[string]map[string]map[string]any
	mapColDefs    map[string]*ingitdb.CollectionDef
	mapLoaded     map[string]bool
}

var _ dal.ReadwriteTransaction = (*batchingTx)(nil)

func (t *batchingTx) Set(ctx context.Context, record dal.Record) error {
	colDef, recordKey, err := t.resolveCollection(record.Key())
	if err != nil {
		return err
	}
	recordPath := resolveRecordPath(colDef, recordKey)
	record.SetError(nil)
	data, ok := record.Data().(map[string]any)
	if !ok {
		return fmt.Errorf("record data is not map[string]any")
	}
	switch colDef.RecordFile.RecordType {
	case ingitdb.MapOfRecords:
		if loadErr := t.ensureMapLoaded(ctx, recordPath, colDef); loadErr != nil {
			return loadErr
		}
		t.workingMaps[recordPath][recordKey] = dalgo2ingitdb.ApplyLocaleToWrite(data, colDef.Columns)
		return nil
	default:
		encoded, encodeErr := encodeRecordContent(data, colDef.RecordFile.Format)
		if encodeErr != nil {
			return encodeErr
		}
		t.bufferedFiles[recordPath] = TreeChange{Path: recordPath, Content: encoded}
		return nil
	}
}

func (t *batchingTx) Insert(ctx context.Context, record dal.Record, opts ...dal.InsertOption) error {
	_ = opts
	colDef, recordKey, err := t.resolveCollection(record.Key())
	if err != nil {
		return err
	}
	recordPath := resolveRecordPath(colDef, recordKey)
	data, ok := record.Data().(map[string]any)
	if !ok {
		return fmt.Errorf("record data is not map[string]any")
	}
	switch colDef.RecordFile.RecordType {
	case ingitdb.MapOfRecords:
		if loadErr := t.ensureMapLoaded(ctx, recordPath, colDef); loadErr != nil {
			return loadErr
		}
		if _, exists := t.workingMaps[recordPath][recordKey]; exists {
			return fmt.Errorf("record already exists: %s/%s", colDef.ID, recordKey)
		}
		record.SetError(nil)
		t.workingMaps[recordPath][recordKey] = dalgo2ingitdb.ApplyLocaleToWrite(data, colDef.Columns)
		return nil
	default:
		// Check buffered state first: a buffered non-nil Content means the
		// record was Set / Inserted earlier in this tx → collision.
		if existing, has := t.bufferedFiles[recordPath]; has && existing.Content != nil {
			return fmt.Errorf("record already exists: %s/%s", colDef.ID, recordKey)
		}
		// If buffered as deletion, the remote-side file is logically gone
		// for the rest of this tx; allow re-insert.
		bufferedAsDelete := false
		if existing, has := t.bufferedFiles[recordPath]; has && existing.Content == nil {
			bufferedAsDelete = true
		}
		if !bufferedAsDelete {
			_, _, found, readErr := t.db.fileReader.readFileWithSHA(ctx, recordPath)
			if readErr != nil {
				return readErr
			}
			if found {
				return fmt.Errorf("record already exists: %s/%s", colDef.ID, recordKey)
			}
		}
		record.SetError(nil)
		encoded, encodeErr := encodeRecordContent(data, colDef.RecordFile.Format)
		if encodeErr != nil {
			return encodeErr
		}
		t.bufferedFiles[recordPath] = TreeChange{Path: recordPath, Content: encoded}
		return nil
	}
}

func (t *batchingTx) Delete(ctx context.Context, key *dal.Key) error {
	colDef, recordKey, err := t.resolveCollection(key)
	if err != nil {
		return err
	}
	recordPath := resolveRecordPath(colDef, recordKey)
	switch colDef.RecordFile.RecordType {
	case ingitdb.MapOfRecords:
		if loadErr := t.ensureMapLoaded(ctx, recordPath, colDef); loadErr != nil {
			return loadErr
		}
		if _, exists := t.workingMaps[recordPath][recordKey]; !exists {
			return dal.ErrRecordNotFound
		}
		delete(t.workingMaps[recordPath], recordKey)
		return nil
	default:
		if existing, has := t.bufferedFiles[recordPath]; has {
			if existing.Content == nil {
				return dal.ErrRecordNotFound // already buffered as deleted
			}
			// Was buffered as a write earlier; convert to delete.
			t.bufferedFiles[recordPath] = TreeChange{Path: recordPath, Content: nil}
			return nil
		}
		_, _, found, readErr := t.db.fileReader.readFileWithSHA(ctx, recordPath)
		if readErr != nil {
			return readErr
		}
		if !found {
			return dal.ErrRecordNotFound
		}
		t.bufferedFiles[recordPath] = TreeChange{Path: recordPath, Content: nil}
		return nil
	}
}

// ensureMapLoaded reads the current remote state of a MapOfRecords file into
// workingMaps[recordPath] if not already loaded. Missing files load as an
// empty map so first-touch Insert / Set creates the file.
func (t *batchingTx) ensureMapLoaded(ctx context.Context, recordPath string, colDef *ingitdb.CollectionDef) error {
	if t.mapLoaded[recordPath] {
		return nil
	}
	content, _, found, readErr := t.db.fileReader.readFileWithSHA(ctx, recordPath)
	if readErr != nil {
		return readErr
	}
	var loaded map[string]map[string]any
	if !found {
		loaded = make(map[string]map[string]any)
	} else {
		parsed, parseErr := dalgo2ingitdb.ParseMapOfRecordsContent(content, colDef.RecordFile.Format)
		if parseErr != nil {
			return parseErr
		}
		loaded = parsed
	}
	t.workingMaps[recordPath] = loaded
	t.mapColDefs[recordPath] = colDef
	t.mapLoaded[recordPath] = true
	return nil
}

// flushChanges computes the final []TreeChange list. SingleRecord entries
// are already in bufferedFiles and pass through unchanged. MapOfRecords
// working maps are encoded into one TreeChange per modified file. A map
// that has been emptied is written as a deletion of the underlying file
// (consistent with the spec's "leave no trace" semantics for drop and
// truncate-style behaviors).
func (t *batchingTx) flushChanges() ([]TreeChange, error) {
	changes := make([]TreeChange, 0, len(t.bufferedFiles)+len(t.workingMaps))
	for _, ch := range t.bufferedFiles {
		changes = append(changes, ch)
	}
	for recordPath, working := range t.workingMaps {
		colDef := t.mapColDefs[recordPath]
		if len(working) == 0 {
			changes = append(changes, TreeChange{Path: recordPath, Content: nil})
			continue
		}
		encoded, encodeErr := dalgo2ingitdb.EncodeMapOfRecordsContent(
			working, colDef.RecordFile.Format, colDef.ID, colDef.ColumnsOrder)
		if encodeErr != nil {
			return nil, fmt.Errorf("encode map for %s: %w", recordPath, encodeErr)
		}
		changes = append(changes, TreeChange{Path: recordPath, Content: encoded})
	}
	return changes, nil
}

// SetMulti / DeleteMulti / Update / UpdateRecord / UpdateMulti / InsertMulti
// follow the upstream readwriteTx behavior: they are not implemented and
// return an error. Set-mode callers loop Set / Delete instead.

func (t *batchingTx) SetMulti(ctx context.Context, records []dal.Record) error {
	_, _ = ctx, records
	return fmt.Errorf("not implemented by %s (batching)", DatabaseID)
}

func (t *batchingTx) DeleteMulti(ctx context.Context, keys []*dal.Key) error {
	_, _ = ctx, keys
	return fmt.Errorf("not implemented by %s (batching)", DatabaseID)
}

func (t *batchingTx) Update(ctx context.Context, key *dal.Key, updates []update.Update, preconditions ...dal.Precondition) error {
	_, _, _, _ = ctx, key, updates, preconditions
	return fmt.Errorf("not implemented by %s (batching)", DatabaseID)
}

func (t *batchingTx) UpdateRecord(ctx context.Context, record dal.Record, updates []update.Update, preconditions ...dal.Precondition) error {
	_, _, _, _ = ctx, record, updates, preconditions
	return fmt.Errorf("not implemented by %s (batching)", DatabaseID)
}

func (t *batchingTx) UpdateMulti(ctx context.Context, keys []*dal.Key, updates []update.Update, preconditions ...dal.Precondition) error {
	_, _, _, _ = ctx, keys, updates, preconditions
	return fmt.Errorf("not implemented by %s (batching)", DatabaseID)
}

func (t *batchingTx) InsertMulti(ctx context.Context, records []dal.Record, opts ...dal.InsertOption) error {
	_, _, _ = ctx, records, opts
	return fmt.Errorf("not implemented by %s (batching)", DatabaseID)
}

func (t *batchingTx) ID() string {
	return ""
}

package dalgo2ingitdb

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/dal-go/dalgo/dal"
	"github.com/dal-go/dalgo/ddl"
	"github.com/dal-go/dalgo/recordset"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// Database is the dal.DB implementation for inGitDB projects on the local
// filesystem. It implements the schema-management capability interfaces
// (dbschema.SchemaReader, ddl.SchemaModifier, ddl.TransactionalDDL), the
// dal.DB record-access methods, and reports dal.ConcurrencyAvailable via
// the embedded helper struct — concurrent connections are safe because
// every read and write path goes through gofrs/flock file locking.
//
// Record access loads the project Definition once per transaction via the
// injected CollectionsReader; individual file operations take a shared
// (read) or exclusive (write) advisory lock on the affected file.
// ExecuteQueryToRecordsetReader is not yet implemented and returns
// dal.ErrNotSupported.
type Database struct {
	// dal.ConcurrencyAvailable: gofrs/flock provides cross-platform file
	// locking (syscall.Flock on Unix, LockFileEx on Windows), so two DB
	// handles against the same project directory can operate concurrently
	// without data races.
	dal.ConcurrencyAvailable

	projectPath string
	reader      ingitdb.CollectionsReader
}

// NewDatabase constructs a Database rooted at projectPath. The reader is
// used to load the project Definition at the start of each transaction
// and inside DB-level record-access methods. Returns an error if
// projectPath is empty or does not exist; the constructor does NOT load
// any collection definitions.
func NewDatabase(projectPath string, reader ingitdb.CollectionsReader) (dal.DB, error) {
	if projectPath == "" {
		return nil, errors.New("dalgo2ingitdb: projectPath is required")
	}
	info, err := os.Stat(projectPath)
	if err != nil {
		return nil, fmt.Errorf("dalgo2ingitdb: stat %s: %w", projectPath, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("dalgo2ingitdb: %s is not a directory", projectPath)
	}
	return &Database{
		projectPath: projectPath,
		reader:      reader,
	}, nil
}

// DatabaseID is the name reported by Database.ID() and used as the
// Adapter name.
const DatabaseID = "dalgo2ingitdb"

// ID returns the driver identifier.
func (db *Database) ID() string { return DatabaseID }

// Adapter returns the dalgo adapter descriptor.
func (db *Database) Adapter() dal.Adapter {
	return dal.NewAdapter(DatabaseID, "v0.0.1")
}

// Schema returns nil — inGitDB does not yet expose a dal.Schema view of
// its collection definitions. Callers needing schema introspection should
// use dbschema.SchemaReader instead.
func (db *Database) Schema() dal.Schema { return nil }

// SupportsTransactionalDDL satisfies ddl.TransactionalDDL by reporting
// that this driver does NOT guarantee all-or-nothing for multi-op
// AlterCollection calls. A failure mid-sequence leaves earlier ops
// applied; the caller receives a *ddl.PartialSuccessError.
func (db *Database) SupportsTransactionalDDL() bool { return false }

// loadDefinition reads the project's Definition via the injected reader.
// Returns an error when no reader has been wired up.
func (db *Database) loadDefinition() (*ingitdb.Definition, error) {
	if db.reader == nil {
		return nil, errors.New("dalgo2ingitdb: no CollectionsReader configured")
	}
	def, err := db.reader.ReadDefinition(db.projectPath)
	if err != nil {
		return nil, fmt.Errorf("dalgo2ingitdb: read definition: %w", err)
	}
	return def, nil
}

// RunReadonlyTransaction loads the project Definition and invokes the
// worker with a readonly transaction. The Definition is captured at the
// start of the transaction; subsequent on-disk schema changes are not
// observed within the transaction.
func (db *Database) RunReadonlyTransaction(ctx context.Context, f dal.ROTxWorker, _ ...dal.TransactionOption) error {
	def, err := db.loadDefinition()
	if err != nil {
		return err
	}
	return f(ctx, readonlyTx{db: db, def: def})
}

// RunReadwriteTransaction loads the project Definition and invokes the
// worker with a read-write transaction. inGitDB does not guarantee
// atomicity across multiple file writes within a transaction; each
// individual file write is locked exclusively, but a worker that fails
// after writing some files leaves those writes in place.
func (db *Database) RunReadwriteTransaction(ctx context.Context, f dal.RWTxWorker, _ ...dal.TransactionOption) error {
	def, err := db.loadDefinition()
	if err != nil {
		return err
	}
	return f(ctx, readwriteTx{readonlyTx: readonlyTx{db: db, def: def}})
}

// Get loads a single record. See readonlyTx.Get for semantics.
func (db *Database) Get(ctx context.Context, record dal.Record) error {
	def, err := db.loadDefinition()
	if err != nil {
		return err
	}
	return readonlyTx{db: db, def: def}.Get(ctx, record)
}

// Exists reports whether the record identified by key exists on disk.
func (db *Database) Exists(ctx context.Context, key *dal.Key) (bool, error) {
	def, err := db.loadDefinition()
	if err != nil {
		return false, err
	}
	return readonlyTx{db: db, def: def}.Exists(ctx, key)
}

// GetMulti loads multiple records.
func (db *Database) GetMulti(ctx context.Context, records []dal.Record) error {
	def, err := db.loadDefinition()
	if err != nil {
		return err
	}
	return readonlyTx{db: db, def: def}.GetMulti(ctx, records)
}

// ExecuteQueryToRecordsReader runs a structured query against a single
// collection. See readonlyTx.ExecuteQueryToRecordsReader for supported
// query features.
func (db *Database) ExecuteQueryToRecordsReader(ctx context.Context, query dal.Query) (dal.RecordsReader, error) {
	def, err := db.loadDefinition()
	if err != nil {
		return nil, err
	}
	return readonlyTx{db: db, def: def}.ExecuteQueryToRecordsReader(ctx, query)
}

// ExecuteQueryToRecordsetReader is not implemented yet; callers should
// use ExecuteQueryToRecordsReader instead.
func (db *Database) ExecuteQueryToRecordsetReader(_ context.Context, _ dal.Query, _ ...recordset.Option) (dal.RecordsetReader, error) {
	return nil, dal.ErrNotSupported
}

// Compile-time interface checks. SchemaReader / SchemaModifier assertions
// live in schema_reader.go / schema_modifier.go.
var (
	_ dal.DB               = (*Database)(nil)
	_ ddl.TransactionalDDL = (*Database)(nil)
)

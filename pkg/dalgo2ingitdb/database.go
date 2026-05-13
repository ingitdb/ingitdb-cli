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
// (dbschema.SchemaReader, ddl.SchemaModifier, ddl.TransactionalDDL) and
// reports dal.ConcurrencyAvailable via the embedded helper struct —
// concurrent connections are safe because every read and write path
// goes through gofrs/flock file locking.
//
// Record-access methods (Get / GetMulti / Exists / RunReadonlyTransaction
// / RunReadwriteTransaction / ExecuteQuery*) are stubbed with
// dal.ErrNotSupported. The MVP scope is schema management; record CRUD is
// covered by the sibling dalgo2fsingitdb driver and is a follow-up here.
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
// stored for future use (record-access methods will need it when
// implemented). Returns an error if projectPath is empty or does not
// exist; the constructor does NOT load any collection definitions.
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

// RunReadonlyTransaction is a stub returning dal.ErrNotSupported. The
// schema-management surface does not need transactions; record access
// will be added in a follow-up.
func (db *Database) RunReadonlyTransaction(_ context.Context, _ dal.ROTxWorker, _ ...dal.TransactionOption) error {
	return dal.ErrNotSupported
}

// RunReadwriteTransaction is a stub returning dal.ErrNotSupported. See
// RunReadonlyTransaction.
func (db *Database) RunReadwriteTransaction(_ context.Context, _ dal.RWTxWorker, _ ...dal.TransactionOption) error {
	return dal.ErrNotSupported
}

// Get is a stub returning dal.ErrNotSupported.
func (db *Database) Get(_ context.Context, _ dal.Record) error {
	return dal.ErrNotSupported
}

// Exists is a stub returning dal.ErrNotSupported.
func (db *Database) Exists(_ context.Context, _ *dal.Key) (bool, error) {
	return false, dal.ErrNotSupported
}

// GetMulti is a stub returning dal.ErrNotSupported.
func (db *Database) GetMulti(_ context.Context, _ []dal.Record) error {
	return dal.ErrNotSupported
}

// ExecuteQueryToRecordsReader is a stub returning dal.ErrNotSupported.
func (db *Database) ExecuteQueryToRecordsReader(_ context.Context, _ dal.Query) (dal.RecordsReader, error) {
	return nil, dal.ErrNotSupported
}

// ExecuteQueryToRecordsetReader is a stub returning dal.ErrNotSupported.
func (db *Database) ExecuteQueryToRecordsetReader(_ context.Context, _ dal.Query, _ ...recordset.Option) (dal.RecordsetReader, error) {
	return nil, dal.ErrNotSupported
}

// Compile-time interface checks. SchemaReader / SchemaModifier assertions
// live in schema_reader.go / schema_modifier.go once those methods land.
var (
	_ dal.DB               = (*Database)(nil)
	_ ddl.TransactionalDDL = (*Database)(nil)
)

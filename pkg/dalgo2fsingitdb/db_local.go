package dalgo2fsingitdb

import (
	"context"

	"github.com/dal-go/dalgo/dal"
	"github.com/dal-go/dalgo/recordset"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

var _ dal.DB = (*localDB)(nil)

func NewLocalDB(rootDirPath string) (dal.DB, error) {
	// TODO: rootDirPath should point to existing directory
	return &localDB{
		rootDirPath: rootDirPath,
	}, nil
}

// NewLocalDBWithDef creates a localDB with a preloaded schema definition.
// The definition enables CRUD operations that require collection metadata.
func NewLocalDBWithDef(rootDirPath string, def *ingitdb.Definition) (dal.DB, error) {
	return &localDB{
		rootDirPath: rootDirPath,
		def:         def,
	}, nil
}

type localDB struct {
	// dal.NoConcurrency: this driver does not yet implement file locking,
	// so concurrent connections are not safe. dalgo2ingitdb is the
	// concurrent-safe sibling driver.
	dal.NoConcurrency
	rootDirPath string
	def         *ingitdb.Definition
}

func (db localDB) ID() string {
	return DatabaseID
}

func (db localDB) Adapter() dal.Adapter {
	return dal.NewAdapter(DatabaseID, "v0.0.1")
}

// Schema maps ingitdb.Definition to dal.Schema
func (db localDB) Schema() dal.Schema {
	//TODO implement me
	panic("implement me")
}

// RunReadonlyTransaction pull recent changes from origin (if URL to remote repo is specified)
func (db localDB) RunReadonlyTransaction(ctx context.Context, f dal.ROTxWorker, options ...dal.TransactionOption) error {
	tx := readonlyTx{db: db}
	return f(ctx, tx)
}

// RunReadwriteTransaction pull recent changes from origin and creates a new local branch
func (db localDB) RunReadwriteTransaction(ctx context.Context, f dal.RWTxWorker, options ...dal.TransactionOption) error {
	tx := readwriteTx{readonlyTx: readonlyTx{db: db}}
	return f(ctx, tx)
}

func (db localDB) Get(ctx context.Context, record dal.Record) error {
	//TODO implement me
	panic("implement me")
}

func (db localDB) Exists(ctx context.Context, key *dal.Key) (bool, error) {
	//TODO implement me
	panic("implement me")
}

func (db localDB) GetMulti(ctx context.Context, records []dal.Record) error {
	//TODO implement me
	panic("implement me")
}

func (db localDB) ExecuteQueryToRecordsReader(ctx context.Context, query dal.Query) (dal.RecordsReader, error) {
	//TODO implement me
	panic("implement me")
}

func (db localDB) ExecuteQueryToRecordsetReader(ctx context.Context, query dal.Query, options ...recordset.Option) (dal.RecordsetReader, error) {
	//TODO implement me
	panic("implement me")
}

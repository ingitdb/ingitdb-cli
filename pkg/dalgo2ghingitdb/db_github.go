// Package dalgo2ghingitdb provides a DALgo database adapter for reading inGitDB repositories from GitHub using the GitHub API.
// It supports read-only access to public repositories with no authentication required.
// Future versions will support authentication and write operations for private repositories.
package dalgo2ghingitdb

import (
	"context"
	"fmt"

	"github.com/dal-go/dalgo/dal"
	"github.com/dal-go/dalgo/recordset"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

var _ dal.DB = (*githubDB)(nil)

// NewGitHubDB creates a GitHub repository adapter.
// Note: Definition is required for most operations, so prefer NewGitHubDBWithDef.
func NewGitHubDB(cfg Config) (dal.DB, error) {
	reader, err := NewGitHubFileReader(cfg)
	if err != nil {
		return nil, err
	}
	concrete, ok := reader.(*githubFileReader)
	if !ok { // untestable: NewGitHubFileReader always returns *githubFileReader
		return nil, fmt.Errorf("internal error: expected *githubFileReader")
	}
	db := &githubDB{
		cfg:        cfg,
		fileReader: concrete,
	}
	return db, nil
}

func NewGitHubDBWithDef(cfg Config, def *ingitdb.Definition) (dal.DB, error) {
	if def == nil {
		return nil, fmt.Errorf("definition is required")
	}
	reader, err := NewGitHubFileReader(cfg)
	if err != nil {
		return nil, err
	}
	concrete, ok := reader.(*githubFileReader)
	if !ok { // untestable: NewGitHubFileReader always returns *githubFileReader
		return nil, fmt.Errorf("internal error: expected *githubFileReader")
	}
	db := &githubDB{
		cfg:        cfg,
		def:        def,
		fileReader: concrete,
	}
	return db, nil
}

type githubDB struct {
	// dal.NoConcurrency: GitHub API rate limits make concurrent connections
	// from a single client process counterproductive. The conservative
	// default surfaces the right answer to callers sizing worker pools.
	dal.NoConcurrency
	cfg        Config
	def        *ingitdb.Definition
	fileReader *githubFileReader
}

func (db *githubDB) ID() string {
	return DatabaseID
}

func (db *githubDB) Adapter() dal.Adapter {
	return dal.NewAdapter(DatabaseID, "v0.0.1")
}

func (db *githubDB) Schema() dal.Schema {
	return nil
}

func (db *githubDB) RunReadonlyTransaction(ctx context.Context, f dal.ROTxWorker, options ...dal.TransactionOption) error {
	_ = options
	tx := readonlyTx{db: db}
	return f(ctx, tx)
}

func (db *githubDB) RunReadwriteTransaction(ctx context.Context, f dal.RWTxWorker, options ...dal.TransactionOption) error {
	_ = options
	tx := readwriteTx{readonlyTx: readonlyTx{db: db}}
	return f(ctx, tx)
}

func (db *githubDB) Get(ctx context.Context, record dal.Record) error {
	tx := readonlyTx{db: db}
	return tx.Get(ctx, record)
}

func (db *githubDB) Exists(ctx context.Context, key *dal.Key) (bool, error) {
	_, _ = ctx, key
	return false, fmt.Errorf("exists is not implemented by %s", DatabaseID)
}

func (db *githubDB) GetMulti(ctx context.Context, records []dal.Record) error {
	_, _ = ctx, records
	return fmt.Errorf("getmulti is not implemented by %s", DatabaseID)
}

func (db *githubDB) ExecuteQueryToRecordsReader(ctx context.Context, query dal.Query) (dal.RecordsReader, error) {
	_, _ = ctx, query
	return nil, fmt.Errorf("query is not implemented by %s", DatabaseID)
}

func (db *githubDB) ExecuteQueryToRecordsetReader(ctx context.Context, query dal.Query, options ...recordset.Option) (dal.RecordsetReader, error) {
	_, _, _ = ctx, query, options
	return nil, fmt.Errorf("query is not implemented by %s", DatabaseID)
}

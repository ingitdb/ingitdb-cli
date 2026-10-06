package commands

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/dal-go/dalgo/dal"
	"github.com/dal-go/dalgo/recordset"
	"github.com/ingitdb/ingitdb-go/ingitdb"
)

type failingSelectDB struct{ dal.DB }

func (failingSelectDB) RunReadonlyTransaction(ctx context.Context, work dal.ROTxWorker, _ ...dal.TransactionOption) error {
	return work(ctx, failingSelectTx{})
}

type failingSelectTx struct{ dal.ReadTransaction }

func (failingSelectTx) ExecuteQueryToRecordsetReader(context.Context, dal.Query, ...recordset.Option) (dal.RecordsetReader, error) {
	return failingSelectReader{}, nil
}

type failingSelectReader struct{}

func (failingSelectReader) Next() (recordset.Row, recordset.Recordset, error) {
	return nil, nil, errors.New("recordset stream failed")
}
func (failingSelectReader) Recordset() recordset.Recordset { return nil }
func (failingSelectReader) Cursor() (string, error)        { return "", nil }
func (failingSelectReader) Close() error                   { return nil }

func TestSelectSetPropagatesReaderError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, _, logf := selectTestDeps(t, dir)
	newDB := func(string, *ingitdb.Definition) (dal.DB, error) { return failingSelectDB{}, nil }
	_, err := runSelectCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--from=test.items", "--format=json")
	if err == nil || !strings.Contains(err.Error(), "recordset stream failed") {
		t.Fatalf("select reader error = %v", err)
	}
}

package dalgo2fsingitdb

import (
	"context"
	"strings"
	"testing"

	"github.com/dal-go/dalgo/dal"
	"github.com/dal-go/dalgo/recordset"
)

func expectPanic(t *testing.T, fn func()) {
	t.Helper()

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic, got nil")
		}
		msg, ok := r.(string)
		if !ok {
			err, okErr := r.(error)
			if okErr {
				msg = err.Error()
			}
		}
		if !strings.Contains(msg, "implement me") {
			t.Fatalf("expected panic to contain %q, got %q", "implement me", msg)
		}
	}()

	fn()
}

func TestNewLocalDB(t *testing.T) {
	t.Parallel()

	db, err := NewLocalDB("/tmp/root")
	if err != nil {
		t.Fatalf("expected no error, got %s", err)
	}
	local, ok := db.(*localDB)
	if !ok {
		t.Fatalf("expected *localDB, got %T", db)
	}
	if local.rootDirPath != "/tmp/root" {
		t.Fatalf("expected rootDirPath to be /tmp/root, got %s", local.rootDirPath)
	}
}

func TestLocalDB_ID_Adapter(t *testing.T) {
	t.Parallel()

	db := localDB{rootDirPath: "/tmp/root"}
	if db.ID() != DatabaseID {
		t.Fatalf("expected ID %s, got %s", DatabaseID, db.ID())
	}
	adapter := db.Adapter()
	if adapter.Name() != DatabaseID {
		t.Fatalf("expected adapter name %s, got %s", DatabaseID, adapter.Name())
	}
	if adapter.Version() != "v0.0.1" {
		t.Fatalf("expected adapter version v0.0.1, got %s", adapter.Version())
	}
}

func TestLocalDB_RunTransactions(t *testing.T) {
	t.Parallel()

	db := localDB{rootDirPath: "/tmp/root"}
	ctx := context.Background()

	err := db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		ro, ok := tx.(readonlyTx)
		if !ok {
			t.Fatalf("expected readonlyTx, got %T", tx)
		}
		if ro.db.rootDirPath != "/tmp/root" {
			t.Fatalf("expected rootDirPath /tmp/root, got %s", ro.db.rootDirPath)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected no error, got %s", err)
	}

	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		rw, ok := tx.(readwriteTx)
		if !ok {
			t.Fatalf("expected readwriteTx, got %T", tx)
		}
		if rw.db.rootDirPath != "/tmp/root" {
			t.Fatalf("expected rootDirPath /tmp/root, got %s", rw.db.rootDirPath)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected no error, got %s", err)
	}
}

func TestLocalDB_Panics(t *testing.T) {
	t.Parallel()

	db := localDB{rootDirPath: "/tmp/root"}
	ctx := context.Background()
	var record dal.Record
	var key *dal.Key
	var query dal.Query
	var records []dal.Record
	var options []recordset.Option

	tests := []struct {
		name string
		fn   func()
	}{
		{
			name: "schema",
			fn: func() {
				db.Schema()
			},
		},
		{
			name: "get",
			fn: func() {
				err := db.Get(ctx, record)
				_ = err
			},
		},
		{
			name: "exists",
			fn: func() {
				_, err := db.Exists(ctx, key)
				_ = err
			},
		},
		{
			name: "get_multi",
			fn: func() {
				err := db.GetMulti(ctx, records)
				_ = err
			},
		},
		{
			name: "execute_query_records_reader",
			fn: func() {
				_, err := db.ExecuteQueryToRecordsReader(ctx, query)
				_ = err
			},
		},
		{
			name: "execute_query_recordset_reader",
			fn: func() {
				_, err := db.ExecuteQueryToRecordsetReader(ctx, query, options...)
				_ = err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			expectPanic(t, tt.fn)
		})
	}
}

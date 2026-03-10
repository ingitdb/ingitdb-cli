package dalgo2fsingitdb

import (
	"context"
	"testing"

	"github.com/dal-go/dalgo/dal"
	"github.com/dal-go/dalgo/recordset"
)

func TestReadonlyTx_Panics(t *testing.T) {
	t.Parallel()

	tx := readonlyTx{db: localDB{rootDirPath: "/tmp/root"}}
	ctx := context.Background()
	var key *dal.Key
	var query dal.Query
	var records []dal.Record
	var options []recordset.Option

	tests := []struct {
		name string
		fn   func()
	}{
		{
			name: "options",
			fn: func() {
				tx.Options()
			},
		},
		{
			name: "exists",
			fn: func() {
				_, err := tx.Exists(ctx, key)
				_ = err
			},
		},
		{
			name: "get_multi",
			fn: func() {
				err := tx.GetMulti(ctx, records)
				_ = err
			},
		},
		{
			name: "execute_query_recordset_reader",
			fn: func() {
				_, err := tx.ExecuteQueryToRecordsetReader(ctx, query, options...)
				_ = err
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			expectPanic(t, tt.fn)
		})
	}
}

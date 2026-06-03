package dalgo2fsingitdb

import (
	"context"
	"testing"

	"github.com/dal-go/dalgo/dal"
)

func TestReadonlyTx_Panics(t *testing.T) {
	t.Parallel()

	tx := readonlyTx{db: localDB{rootDirPath: "/tmp/root"}}
	ctx := context.Background()
	var key *dal.Key
	var records []dal.Record

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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			expectPanic(t, tt.fn)
		})
	}
}

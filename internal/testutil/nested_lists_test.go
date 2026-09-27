package testutil

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dal-go/dalgo/dal"
	"github.com/dal-go/record"
)

func TestWriteNestedListsDB_UsesDriverLayout(t *testing.T) {
	t.Parallel()
	dir := WriteNestedListsDB(t)
	for _, rel := range []string{
		"lists/$records/to-buy.yaml",
		"lists/$records/to-watch.yaml",
		"lists/to-buy/items/$records/milk.yaml",
		"lists/to-watch/items/$records/interstellar.yaml",
	} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
			t.Errorf("expected record file %s: %v", rel, err)
		}
	}
}

func TestWriteDeepNestedListsDB_UsesDriverLayout(t *testing.T) {
	t.Parallel()
	dir := WriteDeepNestedListsDB(t)
	for _, rel := range []string{
		"lists/$records/items.yaml",
		"lists/items/items/$records/self.yaml",
		"lists/to-buy/items/milk/tags/$records/dairy.yaml",
	} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
			t.Errorf("expected record file %s: %v", rel, err)
		}
	}
}

type fakeTBForMust struct {
	*testing.T
	failed bool
}

func (f *fakeTBForMust) Helper() {}
func (f *fakeTBForMust) Fatalf(_ string, _ ...any) {
	f.failed = true
	panic("fatalf")
}

func TestMust_Helper(t *testing.T) {
	t.Parallel()
	must(t, nil, "no error")

	fake := &fakeTBForMust{T: &testing.T{}}
	func() {
		defer func() { _ = recover() }()
		must(fake, os.ErrNotExist, "expected error")
	}()
	if !fake.failed {
		t.Fatal("expected must to fail on non-nil error")
	}
}

type mockTxForSet struct {
	dal.ReadwriteTransaction
	err error
}

func (m *mockTxForSet) Set(_ context.Context, _ record.Record) error {
	return m.err
}

func TestSetRecords(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	rec := record.NewRecordWithData(record.NewKeyWithID("lists", "item1"), map[string]any{"title": "Test"})

	if err := setRecords(ctx, &mockTxForSet{err: nil}, rec); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	expectedErr := errors.New("set failed")
	if err := setRecords(ctx, &mockTxForSet{err: expectedErr}, rec); !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
}

package testutil

import (
	"os"
	"path/filepath"
	"testing"
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

package testutil

// specscore: feature/subcollection-addressing

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dal-go/dalgo/dal"
	"github.com/dal-go/record"
	"github.com/ingitdb/dalgo2ingitdb4local"
	"github.com/ingitdb/ingitdb-go/ingitdb/validator"
)

// NestedListItem is one item record of the nested lists fixture.
type NestedListItem struct {
	List    string
	ID      string
	Title   string
	AddedAt string
}

// NestedListItems are the item records WriteNestedListsDB writes, in the
// order they were added (ascending added_at).
var NestedListItems = []NestedListItem{
	{List: "to-buy", ID: "milk", Title: "Milk", AddedAt: "2026-09-17T10:00:00Z"},
	{List: "to-buy", ID: "bananas", Title: "Bananas", AddedAt: "2026-09-17T10:00:01Z"},
	{List: "to-buy", ID: "coffee", Title: "Coffee", AddedAt: "2026-09-17T10:00:02Z"},
	{List: "to-watch", ID: "the-matrix", Title: "The Matrix", AddedAt: "2026-09-17T10:00:03Z"},
	{List: "to-watch", ID: "interstellar", Title: "Interstellar", AddedAt: "2026-09-17T10:00:04Z"},
}

const nestedListsDefinition = `record_file:
  name: "{key}.yaml"
  format: yaml
  type: "map[string]any"
columns:
  title:
    type: string
    required: true
columns_order: [title]
`

const nestedItemsDefinition = `record_file:
  name: "{key}.yaml"
  format: yaml
  type: "map[string]any"
columns:
  title:
    type: string
    required: true
  done:
    type: bool
  added_at:
    type: datetime
columns_order: [title, done, added_at]
`

// WriteNestedListsDB creates, in a temporary directory, an inGitDB database
// shaped like the TODO demo: a root collection `lists` (records `to-buy` and
// `to-watch`) declaring a subcollection `items` (NestedListItems). Definitions
// are written as files; records are written through the local driver with
// parent keys, so they sit at the driver's own on-disk paths. It returns the
// database directory.
func WriteNestedListsDB(t testing.TB) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		".ingitdb/root-collections.yaml":                         "lists: lists\n",
		"lists/.collection/definition.yaml":                      nestedListsDefinition,
		"lists/.collection/subcollections/items/definition.yaml": nestedItemsDefinition,
	}
	for name, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", name, err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	def, err := validator.ReadDefinition(dir)
	if err != nil {
		t.Fatalf("read fixture definition: %v", err)
	}
	db, err := dalgo2fsingitdb.NewLocalDBWithDef(dir, def)
	if err != nil {
		t.Fatalf("open fixture db: %v", err)
	}
	lists := []struct{ id, title string }{{"to-buy", "To buy"}, {"to-watch", "To watch"}}
	err = db.RunReadwriteTransaction(context.Background(), func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		for _, l := range lists {
			key := record.NewKeyWithID("lists", l.id)
			if setErr := tx.Set(ctx, record.NewRecordWithData(key, map[string]any{"title": l.title})); setErr != nil {
				return setErr
			}
		}
		for _, it := range NestedListItems {
			parent := record.NewKeyWithID("lists", it.List)
			key := record.NewKeyWithParentAndID(parent, "items", it.ID)
			data := map[string]any{"title": it.Title, "done": false, "added_at": it.AddedAt}
			if setErr := tx.Set(ctx, record.NewRecordWithData(key, data)); setErr != nil {
				return setErr
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("write fixture records: %v", err)
	}
	return dir
}

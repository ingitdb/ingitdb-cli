package tui

// specscore: feature/cli/demo

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dal-go/dalgo/dal"
	"github.com/ingitdb/dalgo2ingitdb4local"
	"github.com/ingitdb/ingitdb-go/ingitdb"
	"github.com/ingitdb/ingitdb-go/ingitdb/validator"

	"github.com/ingitdb/ingitdb-cli/cmd/ingitdb/commands"
)

// TestModel_Demo_ShowsListsAndItems opens the terminal UI model on a demo
// installed by `ingitdb demo install` and walks lists, to-buy, items.
func TestModel_Demo_ShowsListsAndItems(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "todo-demo")
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
	}
	install := commands.Demo(os.UserHomeDir, os.Getwd, validator.ReadDefinition, newDB)
	install.SetArgs([]string{"install", "--path=" + dir})
	install.SetOut(io.Discard)
	if err := install.Execute(); err != nil {
		t.Fatalf("demo install: %v", err)
	}
	def, err := validator.ReadDefinition(dir)
	if err != nil {
		t.Fatalf("read definition: %v", err)
	}
	m := New(dir, def, newDB, 120, 40)

	m = press(t, m, keyEnter) // open lists
	if got := strings.Join(m.collection.recordKeys, ","); got != "to-buy,to-watch" {
		t.Fatalf("lists = %s, want to-buy,to-watch", got)
	}
	view := m.View().Content
	for _, want := range []string{"To buy", "To watch"} {
		if !strings.Contains(view, want) {
			t.Errorf("lists screen lacks %q:\n%s", want, view)
		}
	}

	m = press(t, m, keyEnter) // open to-buy's items
	if m.collection.colDef.ID != "items" {
		t.Fatalf("want items screen, got %q", m.collection.colDef.ID)
	}
	if got := strings.Join(recordTitles(m.collection), ","); got != "Bananas,Coffee,Milk" {
		t.Errorf("items = %s, want Bananas,Coffee,Milk", got)
	}
	for i, r := range m.collection.records {
		if done, ok := r["done"].(bool); !ok || done {
			t.Errorf("item %s: done = %v, want false", m.collection.recordKeys[i], r["done"])
		}
	}
	view = m.View().Content
	for _, want := range []string{"Milk", "Bananas", "Coffee", "done", "added_at"} {
		if !strings.Contains(view, want) {
			t.Errorf("items screen lacks %q:\n%s", want, view)
		}
	}
}

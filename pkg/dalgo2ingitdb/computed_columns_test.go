package dalgo2ingitdb

import (
	"sort"
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func computedColumnsCollectionDef() *ingitdb.CollectionDef {
	return &ingitdb.CollectionDef{
		ID: "people",
		Columns: map[string]*ingitdb.ColumnDef{
			"first_name": {Type: ingitdb.ColumnTypeString},
			"last_name":  {Type: ingitdb.ColumnTypeString},
			"full_name":  {Type: ingitdb.ColumnTypeString, Formula: `first_name + " " + last_name`},
		},
		ColumnsOrder: []string{"first_name", "last_name", "full_name"},
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		found := false
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func TestValidateNoStoredComputedValues_RejectsSuppliedComputedValue(t *testing.T) {
	t.Parallel()
	colDef := computedColumnsCollectionDef()
	r := readwriteTx{}
	data := map[string]any{"first_name": "Ada", "full_name": "Ada Lovelace"}
	err := r.validateNoStoredComputedValues("people", colDef, "ada", data)
	if err == nil {
		t.Fatal("expected error for stored computed value, got nil")
	}
	if !containsAll(err.Error(), "people", "ada", "full_name") {
		t.Fatalf("error %q must name collection, record key, and column", err.Error())
	}
}

func TestValidateNoStoredComputedValues_AllowsAbsentComputedColumn(t *testing.T) {
	t.Parallel()
	colDef := computedColumnsCollectionDef()
	r := readwriteTx{}
	data := map[string]any{"first_name": "Ada", "last_name": "Lovelace"}
	if err := r.validateNoStoredComputedValues("people", colDef, "ada", data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateNoStoredComputedValues_RejectsEvenWhenValueEmpty(t *testing.T) {
	t.Parallel()
	colDef := computedColumnsCollectionDef()
	r := readwriteTx{}
	data := map[string]any{"first_name": "Ada", "full_name": ""}
	err := r.validateNoStoredComputedValues("people", colDef, "ada", data)
	if err == nil {
		t.Fatal("expected error when computed column key present with empty value, got nil")
	}
	if !containsAll(err.Error(), "people", "ada", "full_name") {
		t.Fatalf("error %q must name collection, record key, and column", err.Error())
	}
}

func TestValidateNoStoredComputedValues_NilColumns(t *testing.T) {
	t.Parallel()
	colDef := &ingitdb.CollectionDef{ID: "empty"}
	r := readwriteTx{}
	data := map[string]any{"x": 1}
	if err := r.validateNoStoredComputedValues("empty", colDef, "k", data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ensure deterministic column iteration does not panic without ColumnsOrder
func TestValidateNoStoredComputedValues_NoColumnsOrder(t *testing.T) {
	t.Parallel()
	colDef := &ingitdb.CollectionDef{
		ID: "people",
		Columns: map[string]*ingitdb.ColumnDef{
			"a":         {Type: ingitdb.ColumnTypeInt},
			"computed":  {Type: ingitdb.ColumnTypeInt, Formula: "a * 2"},
			"computed2": {Type: ingitdb.ColumnTypeInt, Formula: "a * 3"},
		},
	}
	r := readwriteTx{}
	data := map[string]any{"a": 1, "computed": 2, "computed2": 3}
	err := r.validateNoStoredComputedValues("people", colDef, "k", data)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// the first computed column reported should be the lexicographically first
	names := []string{"computed", "computed2"}
	sort.Strings(names)
	if !containsAll(err.Error(), names[0]) {
		t.Fatalf("error %q should name a computed column", err.Error())
	}
}

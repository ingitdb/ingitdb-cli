package dalgo2ingitdb

import (
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func newCSVCollectionDef(columns []string) *ingitdb.CollectionDef {
	cols := make(map[string]*ingitdb.ColumnDef, len(columns))
	for _, c := range columns {
		cols[c] = &ingitdb.ColumnDef{}
	}
	return &ingitdb.CollectionDef{
		ID:           "items",
		Columns:      cols,
		ColumnsOrder: columns,
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "items.csv",
			Format:     ingitdb.RecordFormatCSV,
			RecordType: ingitdb.ListOfRecords,
		},
	}
}

func TestParseRecordContentForCollection_CSV_Roundtrip(t *testing.T) {
	t.Parallel()
	col := newCSVCollectionDef([]string{"id", "email", "age"})
	content := []byte("id,email,age\n1,alice@example.com,30\n2,bob@example.com,25\n")

	data, err := ParseRecordContentForCollection(content, col)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rows, ok := data["$records"].([]map[string]any)
	if !ok {
		t.Fatalf("expected []map[string]any under $records, got %T", data["$records"])
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0]["id"] != "1" || rows[0]["email"] != "alice@example.com" || rows[0]["age"] != "30" {
		t.Errorf("row 0 mismatch: %+v", rows[0])
	}
	if rows[1]["id"] != "2" || rows[1]["email"] != "bob@example.com" || rows[1]["age"] != "25" {
		t.Errorf("row 1 mismatch: %+v", rows[1])
	}
}

func TestParseRecordContentForCollection_CSV_RejectsMissingColumn(t *testing.T) {
	t.Parallel()
	col := newCSVCollectionDef([]string{"id", "email", "age"})
	content := []byte("id,email\n1,alice@example.com\n")

	_, err := ParseRecordContentForCollection(content, col)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "age") {
		t.Errorf("expected error to mention missing column 'age'; got: %v", err)
	}
}

func TestParseRecordContentForCollection_CSV_RejectsReorderedHeader(t *testing.T) {
	t.Parallel()
	col := newCSVCollectionDef([]string{"id", "email", "age"})
	content := []byte("email,id,age\nalice@example.com,1,30\n")

	_, err := ParseRecordContentForCollection(content, col)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "order") && !strings.Contains(err.Error(), "position") {
		t.Errorf("expected error to mention order/position mismatch; got: %v", err)
	}
}

func TestEncodeRecordContentForCollection_CSV_HeaderAndRowOrder(t *testing.T) {
	t.Parallel()
	col := newCSVCollectionDef([]string{"id", "name", "email"})
	rows := []map[string]any{
		{"id": "1", "name": "Alice", "email": "alice@example.com"},
		{"id": "2", "name": "Bob", "email": "bob@example.com"},
	}

	out, err := EncodeRecordContentForCollection(rows, col)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "id,name,email\n1,Alice,alice@example.com\n2,Bob,bob@example.com\n"
	if string(out) != want {
		t.Errorf("output mismatch.\n  got:  %q\n  want: %q", string(out), want)
	}
}

func TestEncodeRecordContentForCollection_CSV_Deterministic(t *testing.T) {
	t.Parallel()
	col := newCSVCollectionDef([]string{"id", "name", "email"})
	rows := []map[string]any{
		{"email": "alice@example.com", "name": "Alice", "id": "1"},
		{"name": "Bob", "id": "2", "email": "bob@example.com"},
	}

	first, err := EncodeRecordContentForCollection(rows, col)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i := 0; i < 5; i++ {
		out, err := EncodeRecordContentForCollection(rows, col)
		if err != nil {
			t.Fatalf("unexpected error on iteration %d: %v", i, err)
		}
		if string(out) != string(first) {
			t.Errorf("non-deterministic output on iteration %d.\n  first:  %q\n  this:   %q",
				i, string(first), string(out))
		}
	}
}

func TestEncodeRecordContentForCollection_CSV_RejectsKeyedMap(t *testing.T) {
	t.Parallel()
	col := newCSVCollectionDef([]string{"id", "name"})
	keyed := map[string]map[string]any{
		"1": {"name": "Alice"},
		"2": {"name": "Bob"},
	}

	_, err := EncodeRecordContentForCollection(keyed, col)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "csv") || !strings.Contains(err.Error(), "list") {
		t.Errorf("expected error to mention csv + list; got: %v", err)
	}
}

func TestEncodeRecordContentForCollection_CSV_RejectsNestedObject(t *testing.T) {
	t.Parallel()
	col := newCSVCollectionDef([]string{"id", "address"})
	rows := []map[string]any{
		{"id": "1", "address": map[string]any{"city": "Berlin"}},
	}

	_, err := EncodeRecordContentForCollection(rows, col)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "address") {
		t.Errorf("expected error to mention field name 'address'; got: %v", err)
	}
	if !strings.Contains(err.Error(), "nested") && !strings.Contains(err.Error(), "array") {
		t.Errorf("expected error to mention nested/array constraint; got: %v", err)
	}
}

func TestEncodeRecordContentForCollection_CSV_RejectsArrayField(t *testing.T) {
	t.Parallel()
	col := newCSVCollectionDef([]string{"id", "tags"})
	rows := []map[string]any{
		{"id": "1", "tags": []any{"a", "b", "c"}},
	}

	_, err := EncodeRecordContentForCollection(rows, col)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "tags") {
		t.Errorf("expected error to mention field name 'tags'; got: %v", err)
	}
}

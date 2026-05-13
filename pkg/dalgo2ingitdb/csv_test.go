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

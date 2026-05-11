package dalgo2fsingitdb

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dal-go/dalgo/dal"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// makeINGRDef builds a Definition with one MapOfRecords INGR collection
// rooted at dirPath. INGR is inherently multi-record and uses $ID as the
// reserved first column, so it pairs naturally with MapOfRecords (keyed by
// $ID).
func makeINGRDef(t *testing.T, dirPath string) *ingitdb.Definition {
	t.Helper()
	colDef := &ingitdb.CollectionDef{
		ID:      "test.entries",
		DirPath: dirPath,
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "entries.ingr",
			Format:     ingitdb.RecordFormatINGR,
			RecordType: ingitdb.MapOfRecords,
		},
		Columns: map[string]*ingitdb.ColumnDef{
			"sku":   {Type: ingitdb.ColumnTypeString},
			"name":  {Type: ingitdb.ColumnTypeString},
			"price": {Type: ingitdb.ColumnTypeFloat},
		},
		ColumnsOrder: []string{"sku", "name", "price"},
	}
	return &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"test.entries": colDef,
		},
	}
}

func ingrFilePath(dirPath string) string {
	// MapOfRecords stores all records in a single file at collection root
	// (no {key} placeholder in the name template, so no $records/ subdir).
	return filepath.Join(dirPath, "entries.ingr")
}

func TestINGR_InsertGet(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	def := makeINGRDef(t, dir)
	db := openTestDB(t, dir, def)

	ctx := context.Background()
	key := dal.NewKeyWithID("test.entries", "e001")
	data := map[string]any{
		"sku":   "WIDGET-001",
		"name":  "Widget",
		"price": 9.99,
	}
	rec := dal.NewRecordWithData(key, data)
	err := db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Insert(ctx, rec)
	})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	// On-disk: a real INGR file with the project's header line.
	got, err := os.ReadFile(ingrFilePath(dir))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	gotStr := string(got)
	if !strings.HasPrefix(gotStr, "# INGR.io | test.entries: ") {
		t.Errorf("INGR file should start with the standard header line, got prefix: %q",
			gotStr[:min(len(gotStr), 50)])
	}
	if !strings.Contains(gotStr, `"e001"`) {
		t.Errorf("INGR file should contain $ID value \"e001\", got:\n%s", gotStr)
	}
	if !strings.Contains(gotStr, `"WIDGET-001"`) {
		t.Errorf("INGR file should contain sku value, got:\n%s", gotStr)
	}

	// Read back via DALgo.
	readData := map[string]any{}
	readRec := dal.NewRecordWithData(dal.NewKeyWithID("test.entries", "e001"), readData)
	err = db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		return tx.Get(ctx, readRec)
	})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !readRec.Exists() {
		t.Fatal("record should exist after Insert")
	}
	if readData["sku"] != "WIDGET-001" {
		t.Errorf("sku: got %v, want %q", readData["sku"], "WIDGET-001")
	}
	if readData["name"] != "Widget" {
		t.Errorf("name: got %v, want %q", readData["name"], "Widget")
	}
	if readData["price"] != 9.99 {
		t.Errorf("price: got %v (%T), want 9.99", readData["price"], readData["price"])
	}
	// $ID is stripped from the per-record field map (it's the key, not a field).
	if _, present := readData["$ID"]; present {
		t.Errorf("$ID should not appear in per-record field map, got: %v", readData["$ID"])
	}
}

func TestINGR_MultipleRecords_RoundTrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	def := makeINGRDef(t, dir)
	db := openTestDB(t, dir, def)
	ctx := context.Background()

	// Insert three records in mixed order.
	inserts := []struct {
		id   string
		data map[string]any
	}{
		{"e003", map[string]any{"sku": "C", "name": "Charlie", "price": 3.30}},
		{"e001", map[string]any{"sku": "A", "name": "Alpha", "price": 1.10}},
		{"e002", map[string]any{"sku": "B", "name": "Bravo", "price": 2.20}},
	}
	for _, ins := range inserts {
		err := db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
			return tx.Insert(ctx, dal.NewRecordWithData(
				dal.NewKeyWithID("test.entries", ins.id), ins.data))
		})
		if err != nil {
			t.Fatalf("Insert %s: %v", ins.id, err)
		}
	}

	// Read each back; verify field values survive the multi-record round-trip.
	for _, ins := range inserts {
		readData := map[string]any{}
		readRec := dal.NewRecordWithData(
			dal.NewKeyWithID("test.entries", ins.id), readData)
		err := db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
			return tx.Get(ctx, readRec)
		})
		if err != nil {
			t.Fatalf("Get %s: %v", ins.id, err)
		}
		if readData["sku"] != ins.data["sku"] {
			t.Errorf("%s.sku: got %v, want %v", ins.id, readData["sku"], ins.data["sku"])
		}
		if readData["name"] != ins.data["name"] {
			t.Errorf("%s.name: got %v, want %v", ins.id, readData["name"], ins.data["name"])
		}
	}

	// On-disk: all three records should appear, sorted by $ID for
	// deterministic output (e001, e002, e003).
	got, err := os.ReadFile(ingrFilePath(dir))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	gotStr := string(got)
	idxA := strings.Index(gotStr, `"e001"`)
	idxB := strings.Index(gotStr, `"e002"`)
	idxC := strings.Index(gotStr, `"e003"`)
	if idxA < 0 || idxB < 0 || idxC < 0 {
		t.Fatalf("INGR file missing one or more $ID values: a=%d b=%d c=%d\n%s",
			idxA, idxB, idxC, gotStr)
	}
	if idxA >= idxB || idxB >= idxC {
		t.Errorf("INGR file should list records sorted by $ID; got positions a=%d b=%d c=%d",
			idxA, idxB, idxC)
	}
}

func TestINGR_Update(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	def := makeINGRDef(t, dir)
	db := openTestDB(t, dir, def)
	ctx := context.Background()
	key := dal.NewKeyWithID("test.entries", "e001")

	if err := db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Insert(ctx, dal.NewRecordWithData(key, map[string]any{
			"sku": "A", "name": "Alpha", "price": 1.10,
		}))
	}); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	if err := db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		cur := map[string]any{}
		rec := dal.NewRecordWithData(key, cur)
		if getErr := tx.Get(ctx, rec); getErr != nil {
			return getErr
		}
		cur["name"] = "Alpha (Updated)"
		cur["price"] = 1.50
		return tx.Set(ctx, rec)
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	readData := map[string]any{}
	readRec := dal.NewRecordWithData(dal.NewKeyWithID("test.entries", "e001"), readData)
	if err := db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		return tx.Get(ctx, readRec)
	}); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if readData["name"] != "Alpha (Updated)" {
		t.Errorf("name: got %v, want %q", readData["name"], "Alpha (Updated)")
	}
	if readData["price"] != 1.50 {
		t.Errorf("price: got %v, want 1.50", readData["price"])
	}
	if readData["sku"] != "A" {
		t.Errorf("sku should be preserved across update: got %v, want %q",
			readData["sku"], "A")
	}
}

func TestINGR_Delete(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	def := makeINGRDef(t, dir)
	db := openTestDB(t, dir, def)
	ctx := context.Background()
	a := dal.NewKeyWithID("test.entries", "a")
	b := dal.NewKeyWithID("test.entries", "b")

	for _, k := range []*dal.Key{a, b} {
		err := db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
			return tx.Insert(ctx, dal.NewRecordWithData(k, map[string]any{
				"sku": k.ID.(string), "name": "x", "price": 1.0,
			}))
		})
		if err != nil {
			t.Fatalf("Insert %v: %v", k.ID, err)
		}
	}

	// Delete one.
	if err := db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Delete(ctx, a)
	}); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// a is gone, b survives.
	recA := dal.NewRecordWithData(dal.NewKeyWithID("test.entries", "a"), map[string]any{})
	recB := dal.NewRecordWithData(dal.NewKeyWithID("test.entries", "b"), map[string]any{})
	if err := db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		if getErr := tx.Get(ctx, recA); getErr != nil {
			return getErr
		}
		return tx.Get(ctx, recB)
	}); err != nil {
		t.Fatalf("Get after Delete: %v", err)
	}
	if recA.Exists() {
		t.Error("record a should not exist after Delete")
	}
	if !recB.Exists() {
		t.Error("record b should still exist after deleting a")
	}
}

func TestINGR_RejectsSingleRecordType(t *testing.T) {
	t.Parallel()
	rfd := ingitdb.RecordFileDef{
		Name:       "x.ingr",
		Format:     ingitdb.RecordFormatINGR,
		RecordType: ingitdb.SingleRecord,
	}
	err := rfd.Validate()
	if err == nil {
		t.Fatal("expected validation error for ingr + SingleRecord, got nil")
	}
	if !strings.Contains(err.Error(), "ingr") {
		t.Errorf("error should mention ingr format, got: %v", err)
	}
}

func TestINGR_RejectsMissingID(t *testing.T) {
	t.Parallel()
	// An INGR file with a record that has no $ID column should fail parse
	// when reshaped into MapOfRecords.
	dir := t.TempDir()
	def := makeINGRDef(t, dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	// Hand-author a malformed INGR (no $ID column declared).
	malformed := `# INGR.io | test.entries: sku, name, price
"X"
"X-name"
1.0
# 1 record
`
	if err := os.WriteFile(ingrFilePath(dir), []byte(malformed), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	db := openTestDB(t, dir, def)
	ctx := context.Background()
	rec := dal.NewRecordWithData(dal.NewKeyWithID("test.entries", "X"), map[string]any{})
	err := db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		return tx.Get(ctx, rec)
	})
	if err == nil {
		t.Fatal("expected error reading INGR without $ID column, got nil")
	}
	if !strings.Contains(err.Error(), "$ID") {
		t.Errorf("error should mention missing $ID, got: %v", err)
	}
	_ = errors.Is // keep the import meaningful even if no Is check below
}

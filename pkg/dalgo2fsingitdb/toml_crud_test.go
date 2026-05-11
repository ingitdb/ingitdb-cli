package dalgo2fsingitdb

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dal-go/dalgo/dal"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// makeTOMLDef builds a Definition with one TOML SingleRecord collection
// rooted at dirPath. The collection mirrors the YAML test fixture
// (`makeTestDef`) but with TOML format and a `.toml` filename template,
// so the test exercises the new format-dispatch paths end-to-end.
func makeTOMLDef(t *testing.T, dirPath string) *ingitdb.Definition {
	t.Helper()
	colDef := &ingitdb.CollectionDef{
		ID:      "test.items",
		DirPath: dirPath,
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "{key}.toml",
			Format:     ingitdb.RecordFormatTOML,
			RecordType: ingitdb.SingleRecord,
		},
		Columns: map[string]*ingitdb.ColumnDef{
			"name":   {Type: ingitdb.ColumnTypeString},
			"score":  {Type: ingitdb.ColumnTypeInt},
			"active": {Type: ingitdb.ColumnTypeBool},
			"date":   {Type: ingitdb.ColumnTypeString},
		},
		ColumnsOrder: []string{"name", "score", "active", "date"},
	}
	return &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"test.items": colDef,
		},
	}
}

func tomlRecordPath(dirPath, key string) string {
	return filepath.Join(dirPath, "$records", key+".toml")
}

func TestTOML_InsertGet(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	def := makeTOMLDef(t, dir)
	db := openTestDB(t, dir, def)

	data := map[string]any{
		"name":   "Acme",
		"score":  int64(42),
		"active": true,
		"date":   "2024-01-01",
	}
	key := dal.NewKeyWithID("test.items", "acme")
	record := dal.NewRecordWithData(key, data)

	ctx := context.Background()
	err := db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Insert(ctx, record)
	})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	// On-disk: a real TOML file with key = value lines.
	filePath := tomlRecordPath(dir, "acme")
	got, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	gotStr := string(got)
	for _, want := range []string{`name = 'Acme'`, "score = 42", "active = true"} {
		if !strings.Contains(gotStr, want) {
			t.Errorf("TOML output missing %q. Got:\n%s", want, gotStr)
		}
	}

	// Read back via DALgo.
	readData := map[string]any{}
	readRecord := dal.NewRecordWithData(dal.NewKeyWithID("test.items", "acme"), readData)
	err = db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		return tx.Get(ctx, readRecord)
	})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !readRecord.Exists() {
		t.Fatal("record should exist after Insert")
	}
	if readData["name"] != "Acme" {
		t.Errorf("name: got %v, want %q", readData["name"], "Acme")
	}
	if readData["active"] != true {
		t.Errorf("active: got %v, want true", readData["active"])
	}
	// TOML integers come back as int64 from pelletier/go-toml/v2.
	switch v := readData["score"].(type) {
	case int64:
		if v != 42 {
			t.Errorf("score: got %d, want 42", v)
		}
	default:
		t.Errorf("score: got type %T (%v), want int64", v, v)
	}
}

func TestTOML_Update(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	def := makeTOMLDef(t, dir)
	db := openTestDB(t, dir, def)

	ctx := context.Background()
	key := dal.NewKeyWithID("test.items", "x")
	initial := map[string]any{
		"name":   "Original",
		"score":  int64(1),
		"active": false,
		"date":   "2024-01-01",
	}
	err := db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Insert(ctx, dal.NewRecordWithData(key, initial))
	})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		cur := map[string]any{}
		rec := dal.NewRecordWithData(key, cur)
		if getErr := tx.Get(ctx, rec); getErr != nil {
			return getErr
		}
		cur["name"] = "Updated"
		cur["score"] = int64(99)
		return tx.Set(ctx, rec)
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	readData := map[string]any{}
	readRec := dal.NewRecordWithData(dal.NewKeyWithID("test.items", "x"), readData)
	err = db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		return tx.Get(ctx, readRec)
	})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if readData["name"] != "Updated" {
		t.Errorf("name: got %v, want %q", readData["name"], "Updated")
	}
	if readData["score"].(int64) != 99 {
		t.Errorf("score: got %v, want 99", readData["score"])
	}
}

func TestTOML_Delete(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	def := makeTOMLDef(t, dir)
	db := openTestDB(t, dir, def)

	ctx := context.Background()
	key := dal.NewKeyWithID("test.items", "doomed")
	data := map[string]any{"name": "Doomed", "score": int64(0), "active": false, "date": "2024-01-01"}
	err := db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Insert(ctx, dal.NewRecordWithData(key, data))
	})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	filePath := tomlRecordPath(dir, "doomed")
	if _, statErr := os.Stat(filePath); statErr != nil {
		t.Fatalf("expected file to exist after Insert: %v", statErr)
	}

	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Delete(ctx, key)
	})
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, statErr := os.Stat(filePath); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("expected file gone after Delete, stat err: %v", statErr)
	}

	readRec := dal.NewRecordWithData(dal.NewKeyWithID("test.items", "doomed"), map[string]any{})
	err = db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		return tx.Get(ctx, readRec)
	})
	if err != nil {
		t.Fatalf("Get after Delete: %v", err)
	}
	if readRec.Exists() {
		t.Error("record should not exist after Delete")
	}
}

func TestTOML_TimeField_RoundTrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	def := makeTOMLDef(t, dir)
	db := openTestDB(t, dir, def)

	when := time.Date(2024, time.January, 15, 9, 30, 0, 0, time.UTC)
	data := map[string]any{
		"name":   "Stamped",
		"score":  int64(7),
		"active": true,
		"date":   when,
	}
	key := dal.NewKeyWithID("test.items", "timed")
	ctx := context.Background()
	if err := db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Insert(ctx, dal.NewRecordWithData(key, data))
	}); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	got, err := os.ReadFile(tomlRecordPath(dir, "timed"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	// TOML serializes RFC 3339 datetimes natively (unquoted).
	if !strings.Contains(string(got), "2024-01-15") {
		t.Errorf("expected ISO-8601 date in TOML output, got:\n%s", got)
	}

	readData := map[string]any{}
	readRec := dal.NewRecordWithData(dal.NewKeyWithID("test.items", "timed"), readData)
	err = db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		return tx.Get(ctx, readRec)
	})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	gotTime, ok := readData["date"].(time.Time)
	if !ok {
		t.Fatalf("date: got %T (%v), want time.Time", readData["date"], readData["date"])
	}
	if !gotTime.Equal(when) {
		t.Errorf("date value: got %v, want %v", gotTime, when)
	}
}

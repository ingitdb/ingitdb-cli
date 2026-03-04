package datavalidator

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func TestNewValidator_ReturnsNonNil(t *testing.T) {
	t.Parallel()

	v := NewValidator()
	if v == nil {
		t.Fatal("NewValidator() returned nil")
	}
}

func TestNewValidator_ImplementsInterface(t *testing.T) {
	t.Parallel()

	// Compile-time check: *simpleValidator must satisfy DataValidator.
	// The returned value is non-nil and usable as DataValidator.
	v := NewValidator()
	if v == nil {
		t.Fatal("NewValidator() returned nil DataValidator")
	}
}

func TestValidate_EmptyDefinition(t *testing.T) {
	t.Parallel()

	v := NewValidator()
	def := &ingitdb.Definition{}
	result, err := v.Validate(context.Background(), "/some/db", def)
	if err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("Validate() returned nil result")
	}
}

func TestValidate_CollectionWithRecords(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	colDir := filepath.Join(dir, "mycollection")
	if err := os.MkdirAll(colDir, 0o755); err != nil {
		t.Fatalf("setup: mkdir collection dir: %v", err)
	}

	// Create 3 record subdirectories
	for _, name := range []string{"rec1", "rec2", "rec3"} {
		recDir := filepath.Join(colDir, name)
		if err := os.Mkdir(recDir, 0o755); err != nil {
			t.Fatalf("setup: mkdir record dir %s: %v", name, err)
		}
	}

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"mycollection": {DirPath: colDir},
		},
	}

	v := NewValidator()
	result, err := v.Validate(context.Background(), dir, def)
	if err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("Validate() returned nil result")
	}

	count := result.GetRecordCount("mycollection")
	if count != 3 {
		t.Errorf("GetRecordCount(mycollection) = %d, want 3", count)
	}

	passed, total := result.GetRecordCounts("mycollection")
	if passed != 3 {
		t.Errorf("GetRecordCounts passed = %d, want 3", passed)
	}
	if total != 3 {
		t.Errorf("GetRecordCounts total = %d, want 3", total)
	}
}

func TestValidate_CollectionWithFlatYAMLRecords(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	recordsDir := filepath.Join(dir, "currencies", "$records")
	if err := os.MkdirAll(recordsDir, 0o755); err != nil {
		t.Fatalf("setup: mkdir: %v", err)
	}

	// Flat YAML record files (no subdirectories)
	for _, name := range []string{"USD.yaml", "EUR.yaml", "GBP.yaml"} {
		if err := os.WriteFile(filepath.Join(recordsDir, name), []byte("id: "+name[:3]), 0o644); err != nil {
			t.Fatalf("setup: write file: %v", err)
		}
	}
	// Non-record file should NOT be counted
	if err := os.WriteFile(filepath.Join(recordsDir, "README.md"), []byte("docs"), 0o644); err != nil {
		t.Fatalf("setup: write README: %v", err)
	}

	colDir := filepath.Join(dir, "currencies")
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"currencies": {DirPath: colDir},
		},
	}

	v := NewValidator()
	result, err := v.Validate(context.Background(), dir, def)
	if err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}

	count := result.GetRecordCount("currencies")
	if count != 3 {
		t.Errorf("GetRecordCount(currencies) = %d, want 3", count)
	}
}

func TestValidate_CollectionDeduplicatesDirAndFile(t *testing.T) {
	t.Parallel()

	// When a record has both a flat file (ord001.yaml) and a subdirectory (ord001/)
	// for subcollection data, it should be counted only once.
	dir := t.TempDir()
	recordsDir := filepath.Join(dir, "orders", "$records")
	if err := os.MkdirAll(recordsDir, 0o755); err != nil {
		t.Fatalf("setup: mkdir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(recordsDir, "ord001.yaml"), []byte("id: ord001"), 0o644); err != nil {
		t.Fatalf("setup: write file: %v", err)
	}
	if err := os.Mkdir(filepath.Join(recordsDir, "ord001"), 0o755); err != nil {
		t.Fatalf("setup: mkdir subdir: %v", err)
	}
	// Second record with only a file
	if err := os.WriteFile(filepath.Join(recordsDir, "ord002.yaml"), []byte("id: ord002"), 0o644); err != nil {
		t.Fatalf("setup: write file: %v", err)
	}

	colDir := filepath.Join(dir, "orders")
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"orders": {DirPath: colDir},
		},
	}

	v := NewValidator()
	result, err := v.Validate(context.Background(), dir, def)
	if err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}

	count := result.GetRecordCount("orders")
	if count != 2 {
		t.Errorf("GetRecordCount(orders) = %d, want 2 (ord001 counted once despite file+dir)", count)
	}
}

func TestValidate_CollectionDirWithDotCollectionExcluded(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	colDir := filepath.Join(dir, "col")
	if err := os.MkdirAll(colDir, 0o755); err != nil {
		t.Fatalf("setup: mkdir: %v", err)
	}

	// .collection dir should be excluded from count
	dotCollectionDir := filepath.Join(colDir, ".collection")
	if err := os.Mkdir(dotCollectionDir, 0o755); err != nil {
		t.Fatalf("setup: mkdir .collection: %v", err)
	}
	// Regular record dir should be included
	if err := os.Mkdir(filepath.Join(colDir, "record1"), 0o755); err != nil {
		t.Fatalf("setup: mkdir record1: %v", err)
	}
	// Non-record file (not yaml/json) should NOT be counted
	if err := os.WriteFile(filepath.Join(colDir, "readme.md"), []byte("hi"), 0o644); err != nil {
		t.Fatalf("setup: write file: %v", err)
	}

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"col": {DirPath: colDir},
		},
	}

	v := NewValidator()
	result, err := v.Validate(context.Background(), dir, def)
	if err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}

	count := result.GetRecordCount("col")
	if count != 1 {
		t.Errorf("GetRecordCount(col) = %d, want 1 (only record1; .collection and files excluded)", count)
	}
}

func TestValidate_NonExistentDirCounts0NoError(t *testing.T) {
	t.Parallel()

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"missing": {DirPath: "/this/path/does/not/exist"},
		},
	}

	v := NewValidator()
	result, err := v.Validate(context.Background(), "/some/db", def)
	if err != nil {
		t.Fatalf("Validate() should not return error for missing dir, got: %v", err)
	}
	if result == nil {
		t.Fatal("Validate() returned nil result")
	}

	count := result.GetRecordCount("missing")
	if count != 0 {
		t.Errorf("GetRecordCount(missing) = %d, want 0", count)
	}
}

func TestValidate_MultipleCollections(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	colA := filepath.Join(dir, "colA")
	colB := filepath.Join(dir, "colB")

	for _, d := range []string{colA, colB} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("setup: mkdir %s: %v", d, err)
		}
	}

	// colA has 2 records, colB has 1 record
	for _, name := range []string{"a1", "a2"} {
		if err := os.Mkdir(filepath.Join(colA, name), 0o755); err != nil {
			t.Fatalf("setup: mkdir record %s: %v", name, err)
		}
	}
	if err := os.Mkdir(filepath.Join(colB, "b1"), 0o755); err != nil {
		t.Fatalf("setup: mkdir record b1: %v", err)
	}

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"colA": {DirPath: colA},
			"colB": {DirPath: colB},
		},
	}

	v := NewValidator()
	result, err := v.Validate(context.Background(), dir, def)
	if err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}

	countA := result.GetRecordCount("colA")
	if countA != 2 {
		t.Errorf("GetRecordCount(colA) = %d, want 2", countA)
	}
	countB := result.GetRecordCount("colB")
	if countB != 1 {
		t.Errorf("GetRecordCount(colB) = %d, want 1", countB)
	}
}

func TestValidate_EmptyDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	colDir := filepath.Join(dir, "empty")
	if err := os.Mkdir(colDir, 0o755); err != nil {
		t.Fatalf("setup: mkdir: %v", err)
	}

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"empty": {DirPath: colDir},
		},
	}

	v := NewValidator()
	result, err := v.Validate(context.Background(), dir, def)
	if err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}

	count := result.GetRecordCount("empty")
	if count != 0 {
		t.Errorf("GetRecordCount(empty) = %d, want 0", count)
	}
}

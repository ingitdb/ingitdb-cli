package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func TestTruncate_ReturnsCommand(t *testing.T) {
	t.Parallel()

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "/tmp", nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return nil, nil
	}
	logf := func(...any) {}

	cmd := Truncate(homeDir, getWd, readDef, logf)
	if cmd == nil {
		t.Fatal("Truncate() returned nil")
	}
	if cmd.Use != "truncate" {
		t.Errorf("expected name 'truncate', got %q", cmd.Name())
	}
	if cmd.RunE == nil {
		t.Fatal("expected Action to be set")
	}
}

func TestTruncate_SingleRecord_Success(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create a collection directory with $records/ containing record files.
	colDir := filepath.Join(tmpDir, "items")
	recordsDir := filepath.Join(colDir, "$records")
	mkdirErr := os.MkdirAll(recordsDir, 0o755)
	if mkdirErr != nil {
		t.Fatalf("setup: mkdir: %v", mkdirErr)
	}

	// Create some record files.
	for _, name := range []string{"alpha.yaml", "beta.yaml", "gamma.yaml"} {
		writeErr := os.WriteFile(filepath.Join(recordsDir, name), []byte("name: "+name+"\n"), 0o644)
		if writeErr != nil {
			t.Fatalf("setup: write %s: %v", name, writeErr)
		}
	}

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"test.items": {
				ID:      "test.items",
				DirPath: colDir,
				RecordFile: &ingitdb.RecordFileDef{
					Name:       "{key}.yaml",
					Format:     "yaml",
					RecordType: ingitdb.SingleRecord,
				},
				Columns: map[string]*ingitdb.ColumnDef{
					"name": {Type: ingitdb.ColumnTypeString},
				},
			},
		},
	}

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return tmpDir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return def, nil
	}
	logf := func(...any) {}

	cmd := Truncate(homeDir, getWd, readDef, logf)
	err := runCobraCommand(cmd, "--path="+tmpDir, "--collection=test.items")
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}

	// Verify records directory is now empty.
	entries, readErr := os.ReadDir(recordsDir)
	if readErr != nil {
		t.Fatalf("read records dir: %v", readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 records after truncate, got %d", len(entries))
	}
}

func TestTruncate_MapOfRecords_Success(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	colDir := filepath.Join(tmpDir, "tags")
	mkdirErr := os.MkdirAll(colDir, 0o755)
	if mkdirErr != nil {
		t.Fatalf("setup: mkdir: %v", mkdirErr)
	}

	// Write a map-of-records file.
	recordsFile := filepath.Join(colDir, "tags.yaml")
	writeErr := os.WriteFile(recordsFile, []byte("alpha:\n  name: Alpha\nbeta:\n  name: Beta\n"), 0o644)
	if writeErr != nil {
		t.Fatalf("setup: write records file: %v", writeErr)
	}

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"test.tags": {
				ID:      "test.tags",
				DirPath: colDir,
				RecordFile: &ingitdb.RecordFileDef{
					Name:       "tags.yaml",
					Format:     "yaml",
					RecordType: ingitdb.MapOfRecords,
				},
				Columns: map[string]*ingitdb.ColumnDef{
					"name": {Type: ingitdb.ColumnTypeString},
				},
			},
		},
	}

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return tmpDir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return def, nil
	}
	logf := func(...any) {}

	cmd := Truncate(homeDir, getWd, readDef, logf)
	err := runCobraCommand(cmd, "--path="+tmpDir, "--collection=test.tags")
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}

	// Verify the file now contains an empty map.
	content, readErr := os.ReadFile(recordsFile)
	if readErr != nil {
		t.Fatalf("read records file: %v", readErr)
	}
	if string(content) != "{}\n" {
		t.Fatalf("expected empty map, got %q", string(content))
	}
}

func TestTruncate_CollectionNotFound(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{},
	}

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return tmpDir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return def, nil
	}
	logf := func(...any) {}

	cmd := Truncate(homeDir, getWd, readDef, logf)
	err := runCobraCommand(cmd, "--path="+tmpDir, "--collection=nonexistent.col")
	if err == nil {
		t.Fatal("expected error for non-existent collection")
	}
}

func TestTruncate_RecordsDirNotExist(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Collection dir exists but has no $records/ subdirectory.
	colDir := filepath.Join(tmpDir, "items")
	mkdirErr := os.MkdirAll(colDir, 0o755)
	if mkdirErr != nil {
		t.Fatalf("setup: mkdir: %v", mkdirErr)
	}

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"test.items": {
				ID:      "test.items",
				DirPath: colDir,
				RecordFile: &ingitdb.RecordFileDef{
					Name:       "{key}.yaml",
					Format:     "yaml",
					RecordType: ingitdb.SingleRecord,
				},
				Columns: map[string]*ingitdb.ColumnDef{
					"name": {Type: ingitdb.ColumnTypeString},
				},
			},
		},
	}

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return tmpDir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return def, nil
	}
	logf := func(...any) {}

	cmd := Truncate(homeDir, getWd, readDef, logf)
	// Should succeed with 0 records removed when $records/ doesn't exist.
	err := runCobraCommand(cmd, "--path="+tmpDir, "--collection=test.items")
	if err != nil {
		t.Fatalf("expected no error when records dir doesn't exist, got: %v", err)
	}
}

func TestTruncate_PreservesCollectionDefinition(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	colDir := filepath.Join(tmpDir, "items")
	recordsDir := filepath.Join(colDir, "$records")
	mkdirErr := os.MkdirAll(recordsDir, 0o755)
	if mkdirErr != nil {
		t.Fatalf("setup: mkdir: %v", mkdirErr)
	}

	// Create a definition file in the collection directory.
	defContent := []byte("record_file:\n  name: \"{key}.yaml\"\n  format: yaml\n  type: \"map[string]any\"\n")
	defDir := filepath.Join(colDir, ".collections", "items")
	mkdirErr = os.MkdirAll(defDir, 0o755)
	if mkdirErr != nil {
		t.Fatalf("setup: mkdir def: %v", mkdirErr)
	}
	writeErr := os.WriteFile(filepath.Join(defDir, "definition.yaml"), defContent, 0o644)
	if writeErr != nil {
		t.Fatalf("setup: write definition: %v", writeErr)
	}

	// Create a record file.
	writeErr = os.WriteFile(filepath.Join(recordsDir, "item1.yaml"), []byte("name: Item1\n"), 0o644)
	if writeErr != nil {
		t.Fatalf("setup: write record: %v", writeErr)
	}

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"test.items": {
				ID:      "test.items",
				DirPath: colDir,
				RecordFile: &ingitdb.RecordFileDef{
					Name:       "{key}.yaml",
					Format:     "yaml",
					RecordType: ingitdb.SingleRecord,
				},
				Columns: map[string]*ingitdb.ColumnDef{
					"name": {Type: ingitdb.ColumnTypeString},
				},
			},
		},
	}

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return tmpDir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return def, nil
	}
	logf := func(...any) {}

	cmd := Truncate(homeDir, getWd, readDef, logf)
	err := runCobraCommand(cmd, "--path="+tmpDir, "--collection=test.items")
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}

	// Verify definition file still exists.
	_, statErr := os.Stat(filepath.Join(defDir, "definition.yaml"))
	if statErr != nil {
		t.Fatalf("definition file should still exist after truncate: %v", statErr)
	}

	// Verify records are gone.
	entries, readErr := os.ReadDir(recordsDir)
	if readErr != nil {
		t.Fatalf("read records dir: %v", readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 records, got %d", len(entries))
	}
}

func TestTruncateCollection_NilRecordFile(t *testing.T) {
	t.Parallel()

	colDef := &ingitdb.CollectionDef{
		ID:         "test.items",
		DirPath:    "/tmp/nonexistent",
		RecordFile: nil,
	}

	_, err := truncateCollection(colDef)
	if err == nil {
		t.Fatal("expected error for nil RecordFile")
	}
}

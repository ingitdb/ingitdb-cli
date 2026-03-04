package materializer

// Tests for the injected file-system seams in buildDefaultView and buildFKViews.
//
// Each test below stubs one of the three fsOps fields (mkdirAll, writeFile, readFile)
// to return a controlled error and verifies that the error surfaces correctly in the
// returned errs slice. This covers the previously-untested error branches:
//   - buildDefaultView: os.MkdirAll failure
//   - buildDefaultView: os.WriteFile failure
//   - buildFKViews:     os.MkdirAll failure
//   - buildFKViews:     os.WriteFile failure
//
// We also cover the unchanged-logf branch of both helpers by stubbing readFile to
// return the same content that formatExportBatch would produce.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// ---------------------------------------------------------------------------
// helpers shared by the tests in this file
// ---------------------------------------------------------------------------

// minimalCol returns a CollectionDef sufficient for buildDefaultView / buildFKViews.
func minimalCol(t *testing.T, tmpDir string) *ingitdb.CollectionDef {
	t.Helper()
	return &ingitdb.CollectionDef{
		ID:           "things",
		DirPath:      filepath.Join(tmpDir, "things"),
		ColumnsOrder: []string{"$ID", "name"},
	}
}

// minimalView returns a simple default ViewDef that uses JSON format.
func minimalView() *ingitdb.ViewDef {
	return &ingitdb.ViewDef{
		ID:        ingitdb.DefaultViewID,
		IsDefault: true,
		Format:    "json",
	}
}

// oneRecord returns a slice with a single record suitable for minimalCol.
func oneRecord() []ingitdb.IRecordEntry {
	return []ingitdb.IRecordEntry{
		ingitdb.NewMapRecordEntry("1", map[string]any{"$ID": "1", "name": "Widget"}),
	}
}

// ---------------------------------------------------------------------------
// buildDefaultView — mkdirAll error
// ---------------------------------------------------------------------------

func TestBuildDefaultView_MkdirAllError(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	col := minimalCol(t, tmpDir)
	view := minimalView()
	records := oneRecord()

	mkdirErr := errors.New("mkdir permission denied")
	fs := fsOps{
		mkdirAll:  func(string, os.FileMode) error { return mkdirErr },
		readFile:  os.ReadFile,
		writeFile: os.WriteFile,
	}

	created, updated, unchanged, errs := buildDefaultView(tmpDir, "", col, &ingitdb.Definition{}, view, records, nil, fs)

	if len(errs) == 0 {
		t.Fatal("expected error from mkdirAll stub, got none")
	}
	if !errors.Is(errs[0], mkdirErr) {
		t.Errorf("expected mkdirErr in errs[0], got: %v", errs[0])
	}
	if created+updated+unchanged != 0 {
		t.Errorf("expected no files counted when mkdirAll fails, got created=%d updated=%d unchanged=%d",
			created, updated, unchanged)
	}
}

// ---------------------------------------------------------------------------
// buildDefaultView — writeFile error
// ---------------------------------------------------------------------------

func TestBuildDefaultView_WriteFileError(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	col := minimalCol(t, tmpDir)
	view := minimalView()
	records := oneRecord()

	writeErr := errors.New("write permission denied")
	fs := fsOps{
		mkdirAll: os.MkdirAll,
		readFile: func(string) ([]byte, error) {
			// Simulate "file does not exist" so the unchanged check is skipped.
			return nil, os.ErrNotExist
		},
		writeFile: func(string, []byte, os.FileMode) error { return writeErr },
	}

	created, updated, unchanged, errs := buildDefaultView(tmpDir, "", col, &ingitdb.Definition{}, view, records, nil, fs)

	if len(errs) == 0 {
		t.Fatal("expected error from writeFile stub, got none")
	}
	if !errors.Is(errs[0], writeErr) {
		t.Errorf("expected writeErr in errs[0], got: %v", errs[0])
	}
	if created+updated+unchanged != 0 {
		t.Errorf("expected no files counted when writeFile fails, got created=%d updated=%d unchanged=%d",
			created, updated, unchanged)
	}
}

// ---------------------------------------------------------------------------
// buildDefaultView — unchanged branch with logf (readFile returns same bytes)
// ---------------------------------------------------------------------------

func TestBuildDefaultView_UnchangedLogfViaSeam(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	col := minimalCol(t, tmpDir)
	view := minimalView()
	records := oneRecord()

	// First, produce the content that formatExportBatch would generate so we can
	// return it from our readFile stub — making the unchanged check succeed.
	exportContent, err := formatExportBatch("json", col.ID+"/"+view.ID,
		determineColumns(col, view), records)
	if err != nil {
		t.Fatalf("setup: formatExportBatch: %v", err)
	}

	logCalls := 0
	logf := func(string, ...any) { logCalls++ }

	fs := fsOps{
		mkdirAll:  os.MkdirAll,
		readFile:  func(string) ([]byte, error) { return exportContent, nil },
		writeFile: os.WriteFile, // should never be reached
	}

	_, _, unchanged, errs := buildDefaultView(tmpDir, "", col, &ingitdb.Definition{}, view, records, logf, fs)

	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if unchanged != 1 {
		t.Errorf("expected 1 unchanged, got %d", unchanged)
	}
	if logCalls == 0 {
		t.Error("expected logf to be called on unchanged path")
	}
}

// ---------------------------------------------------------------------------
// buildFKViews — mkdirAll error
// ---------------------------------------------------------------------------

func TestBuildFKViews_MkdirAllError_ViaSeam(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	col := &ingitdb.CollectionDef{
		ID:           "orders",
		DirPath:      filepath.Join(tmpDir, "orders"),
		ColumnsOrder: []string{"$ID", "customer"},
		Columns: map[string]*ingitdb.ColumnDef{
			"customer": {Type: ingitdb.ColumnTypeString, ForeignKey: "customers"},
		},
	}
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"customers": {ID: "customers", DirPath: filepath.Join(tmpDir, "customers")},
		},
	}
	view := &ingitdb.ViewDef{
		ID:        ingitdb.DefaultViewID,
		IsDefault: true,
		Format:    "json",
	}
	records := []ingitdb.IRecordEntry{
		ingitdb.NewMapRecordEntry("1", map[string]any{"$ID": "1", "customer": "alice"}),
	}

	mkdirErr := errors.New("fk mkdir permission denied")
	fs := fsOps{
		mkdirAll:  func(string, os.FileMode) error { return mkdirErr },
		readFile:  os.ReadFile,
		writeFile: os.WriteFile,
	}

	created, updated, unchanged, errs := buildFKViews(tmpDir, "", col, def, view, records, nil, fs)

	if len(errs) == 0 {
		t.Fatal("expected error from mkdirAll stub in buildFKViews, got none")
	}
	if !errors.Is(errs[0], mkdirErr) {
		t.Errorf("expected mkdirErr in errs[0], got: %v", errs[0])
	}
	if created+updated+unchanged != 0 {
		t.Errorf("expected no files counted when mkdirAll fails, got created=%d updated=%d unchanged=%d",
			created, updated, unchanged)
	}
}

// ---------------------------------------------------------------------------
// buildFKViews — writeFile error
// ---------------------------------------------------------------------------

func TestBuildFKViews_WriteFileError_ViaSeam(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	col := &ingitdb.CollectionDef{
		ID:           "orders",
		DirPath:      filepath.Join(tmpDir, "orders"),
		ColumnsOrder: []string{"$ID", "customer"},
		Columns: map[string]*ingitdb.ColumnDef{
			"customer": {Type: ingitdb.ColumnTypeString, ForeignKey: "customers"},
		},
	}
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"customers": {ID: "customers", DirPath: filepath.Join(tmpDir, "customers")},
		},
	}
	view := &ingitdb.ViewDef{
		ID:        ingitdb.DefaultViewID,
		IsDefault: true,
		Format:    "json",
	}
	records := []ingitdb.IRecordEntry{
		ingitdb.NewMapRecordEntry("1", map[string]any{"$ID": "1", "customer": "alice"}),
	}

	writeErr := errors.New("fk write permission denied")
	fs := fsOps{
		mkdirAll: os.MkdirAll,
		readFile: func(string) ([]byte, error) {
			return nil, os.ErrNotExist
		},
		writeFile: func(string, []byte, os.FileMode) error { return writeErr },
	}

	created, updated, unchanged, errs := buildFKViews(tmpDir, "", col, def, view, records, nil, fs)

	if len(errs) == 0 {
		t.Fatal("expected error from writeFile stub in buildFKViews, got none")
	}
	if !errors.Is(errs[0], writeErr) {
		t.Errorf("expected writeErr in errs[0], got: %v", errs[0])
	}
	if created+updated+unchanged != 0 {
		t.Errorf("expected no files counted when writeFile fails, got created=%d updated=%d unchanged=%d",
			created, updated, unchanged)
	}
}

// ---------------------------------------------------------------------------
// buildFKViews — unchanged branch with logf (readFile returns same bytes)
// ---------------------------------------------------------------------------

func TestBuildFKViews_UnchangedLogfViaSeam(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	col := &ingitdb.CollectionDef{
		ID:           "orders",
		DirPath:      filepath.Join(tmpDir, "orders"),
		ColumnsOrder: []string{"$ID", "customer"},
		Columns: map[string]*ingitdb.ColumnDef{
			"customer": {Type: ingitdb.ColumnTypeString, ForeignKey: "customers"},
		},
	}
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"customers": {ID: "customers", DirPath: filepath.Join(tmpDir, "customers")},
		},
	}
	view := &ingitdb.ViewDef{
		ID:        ingitdb.DefaultViewID,
		IsDefault: true,
		Format:    "json",
	}
	records := []ingitdb.IRecordEntry{
		ingitdb.NewMapRecordEntry("1", map[string]any{"$ID": "1", "customer": "alice"}),
	}

	// Build the FK export columns (customer is excluded from FK view).
	exportCols := []string{"$ID"} // customer (the FK col) is stripped
	viewName := "customers/$fk/orders/customer/alice"
	fkRecords := []ingitdb.IRecordEntry{
		ingitdb.NewMapRecordEntry("1", map[string]any{"$ID": "1", "customer": "alice"}),
	}
	exportContent, err := formatExportBatch("json", viewName, exportCols, fkRecords,
		WithColumnTypes(col), WithRecordsDelimiter())
	if err != nil {
		t.Fatalf("setup: formatExportBatch: %v", err)
	}

	logCalls := 0
	logf := func(string, ...any) { logCalls++ }

	fs := fsOps{
		mkdirAll:  os.MkdirAll,
		readFile:  func(string) ([]byte, error) { return exportContent, nil },
		writeFile: os.WriteFile,
	}

	_, _, unchanged, errs := buildFKViews(tmpDir, "", col, def, view, records, logf, fs)

	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if unchanged != 1 {
		t.Errorf("expected 1 unchanged, got %d", unchanged)
	}
	if logCalls == 0 {
		t.Error("expected logf to be called on FK unchanged path")
	}
}

// ---------------------------------------------------------------------------
// SimpleViewBuilder.fsOpsOrDefault — verify defaults are wired up correctly
// ---------------------------------------------------------------------------

func TestSimpleViewBuilder_FsOpsOrDefault_UsesRealOS(t *testing.T) {
	t.Parallel()

	// A zero-value builder must resolve to real OS functions (not panic).
	b := SimpleViewBuilder{}
	fs := b.fsOpsOrDefault()

	if fs.mkdirAll == nil {
		t.Error("expected mkdirAll to be non-nil")
	}
	if fs.readFile == nil {
		t.Error("expected readFile to be non-nil")
	}
	if fs.writeFile == nil {
		t.Error("expected writeFile to be non-nil")
	}

	// Verify mkdirAll is actually os.MkdirAll by using it on a real temp path.
	tmpDir := t.TempDir()
	newDir := filepath.Join(tmpDir, "sub", "dir")
	if err := fs.mkdirAll(newDir, 0o755); err != nil {
		t.Fatalf("fsOpsOrDefault().mkdirAll failed: %v", err)
	}
	if _, err := os.Stat(newDir); err != nil {
		t.Fatalf("directory not created: %v", err)
	}

	// Verify writeFile and readFile round-trip.
	p := filepath.Join(newDir, "test.txt")
	content := []byte("hello fsOps")
	if err := fs.writeFile(p, content, 0o644); err != nil {
		t.Fatalf("fsOpsOrDefault().writeFile failed: %v", err)
	}
	got, err := fs.readFile(p)
	if err != nil {
		t.Fatalf("fsOpsOrDefault().readFile failed: %v", err)
	}
	if !strings.EqualFold(string(got), string(content)) {
		t.Errorf("readFile returned %q, want %q", got, content)
	}
}

// ---------------------------------------------------------------------------
// SimpleViewBuilder.fs field — injected stub survives BuildView / BuildViews
// ---------------------------------------------------------------------------

func TestSimpleViewBuilder_BuildView_DefaultView_MkdirError(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	col := &ingitdb.CollectionDef{
		ID:           "items",
		DirPath:      filepath.Join(tmpDir, "items"),
		ColumnsOrder: []string{"$ID"},
	}
	view := &ingitdb.ViewDef{
		ID:        ingitdb.DefaultViewID,
		IsDefault: true,
		Format:    "json",
	}
	records := []ingitdb.IRecordEntry{
		ingitdb.NewMapRecordEntry("1", map[string]any{"$ID": "1"}),
	}

	mkdirErr := errors.New("builder mkdir denied")
	builder := SimpleViewBuilder{
		RecordsReader: fakeRecordsReader{records: records},
		Writer:        &capturingWriter{},
		fs: fsOps{
			mkdirAll:  func(string, os.FileMode) error { return mkdirErr },
			readFile:  os.ReadFile,
			writeFile: os.WriteFile,
		},
	}

	result, err := builder.BuildView(context.Background(), tmpDir, "", col, &ingitdb.Definition{}, view)
	if err != nil {
		t.Fatalf("BuildView() should not return top-level error: %v", err)
	}
	if len(result.Errors) == 0 {
		t.Fatal("expected error in result.Errors from mkdirAll stub, got none")
	}
	if !errors.Is(result.Errors[0], mkdirErr) {
		t.Errorf("expected mkdirErr, got: %v", result.Errors[0])
	}
}

func TestSimpleViewBuilder_BuildViews_DefaultView_WriteFileError(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	col := &ingitdb.CollectionDef{
		ID:           "items",
		DirPath:      filepath.Join(tmpDir, "items"),
		ColumnsOrder: []string{"$ID"},
	}
	dv := &ingitdb.ViewDef{
		ID:        ingitdb.DefaultViewID,
		IsDefault: true,
		Format:    "json",
	}
	records := []ingitdb.IRecordEntry{
		ingitdb.NewMapRecordEntry("1", map[string]any{"$ID": "1"}),
	}

	writeErr := errors.New("builder write denied")
	builder := SimpleViewBuilder{
		DefReader:     fakeViewDefReader{views: map[string]*ingitdb.ViewDef{ingitdb.DefaultViewID: dv}},
		RecordsReader: fakeRecordsReader{records: records},
		Writer:        &capturingWriter{},
		fs: fsOps{
			mkdirAll:  os.MkdirAll,
			readFile:  func(string) ([]byte, error) { return nil, os.ErrNotExist },
			writeFile: func(string, []byte, os.FileMode) error { return writeErr },
		},
	}

	result, err := builder.BuildViews(context.Background(), tmpDir, "", col, &ingitdb.Definition{})
	if err != nil {
		t.Fatalf("BuildViews() should not return top-level error: %v", err)
	}
	if len(result.Errors) == 0 {
		t.Fatal("expected error in result.Errors from writeFile stub, got none")
	}
	if !errors.Is(result.Errors[0], writeErr) {
		t.Errorf("expected writeErr, got: %v", result.Errors[0])
	}
}

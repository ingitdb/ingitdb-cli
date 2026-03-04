package materializer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func TestSimpleViewBuilder_BuildViews_MissingDefReader(t *testing.T) {
	t.Parallel()

	builder := SimpleViewBuilder{
		DefReader:     nil,
		RecordsReader: fakeRecordsReader{},
		Writer:        &capturingWriter{},
	}
	_, err := builder.BuildViews(context.Background(), "/db", "", &ingitdb.CollectionDef{}, &ingitdb.Definition{})
	if err == nil {
		t.Fatal("expected error for missing DefReader")
	}
}

func TestSimpleViewBuilder_BuildViews_MissingRecordsReader(t *testing.T) {
	t.Parallel()

	builder := SimpleViewBuilder{
		DefReader:     fakeViewDefReader{},
		RecordsReader: nil,
		Writer:        &capturingWriter{},
	}
	_, err := builder.BuildViews(context.Background(), "/db", "", &ingitdb.CollectionDef{}, &ingitdb.Definition{})
	if err == nil {
		t.Fatal("expected error for missing RecordsReader")
	}
}

func TestSimpleViewBuilder_BuildViews_MissingWriter(t *testing.T) {
	t.Parallel()

	builder := SimpleViewBuilder{
		DefReader:     fakeViewDefReader{},
		RecordsReader: fakeRecordsReader{},
		Writer:        nil,
	}
	_, err := builder.BuildViews(context.Background(), "/db", "", &ingitdb.CollectionDef{}, &ingitdb.Definition{})
	if err == nil {
		t.Fatal("expected error for missing Writer")
	}
}

type errorViewDefReader struct {
	err error
}

func (e errorViewDefReader) ReadViewDefs(string) (map[string]*ingitdb.ViewDef, error) {
	return nil, e.err
}

func TestSimpleViewBuilder_BuildViews_DefReaderError(t *testing.T) {
	t.Parallel()

	defErr := errors.New("failed to read view defs")
	builder := SimpleViewBuilder{
		DefReader:     errorViewDefReader{err: defErr},
		RecordsReader: fakeRecordsReader{},
		Writer:        &capturingWriter{},
	}
	_, err := builder.BuildViews(context.Background(), "/db", "", &ingitdb.CollectionDef{}, &ingitdb.Definition{})
	if err == nil {
		t.Fatal("expected error from DefReader")
	}
	if !errors.Is(err, defErr) {
		t.Errorf("expected error to be def reader error, got: %v", err)
	}
}

type errorRecordsReader struct {
	err error
}

func (e errorRecordsReader) ReadRecords(
	ctx context.Context,
	dbPath string,
	col *ingitdb.CollectionDef,
	yield func(ingitdb.IRecordEntry) error,
) error {
	_ = ctx
	_ = dbPath
	_ = col
	_ = yield
	return e.err
}

func TestSimpleViewBuilder_BuildViews_RecordsReaderError(t *testing.T) {
	t.Parallel()

	readerErr := errors.New("failed to read records")
	view := &ingitdb.ViewDef{ID: "test", OrderBy: "", FileName: "test.md"}
	builder := SimpleViewBuilder{
		DefReader:     fakeViewDefReader{views: map[string]*ingitdb.ViewDef{"test": view}},
		RecordsReader: errorRecordsReader{err: readerErr},
		Writer:        &capturingWriter{},
	}
	_, err := builder.BuildViews(context.Background(), "/db", "", &ingitdb.CollectionDef{}, &ingitdb.Definition{})
	if err == nil {
		t.Fatal("expected error from RecordsReader")
	}
	if !errors.Is(err, readerErr) {
		t.Errorf("expected error to be records reader error, got: %v", err)
	}
}

type errorWriter struct {
	err error
}

func (w errorWriter) WriteView(
	ctx context.Context,
	col *ingitdb.CollectionDef,
	view *ingitdb.ViewDef,
	records []ingitdb.IRecordEntry,
	outPath string,
) (WriteOutcome, error) {
	_ = ctx
	_ = col
	_ = view
	_ = records
	_ = outPath
	return WriteOutcomeUnchanged, w.err
}

func TestSimpleViewBuilder_BuildViews_WriterError(t *testing.T) {
	t.Parallel()

	writerErr := errors.New("failed to write view")
	view := &ingitdb.ViewDef{ID: "test", OrderBy: "", FileName: "test.md"}
	builder := SimpleViewBuilder{
		DefReader:     fakeViewDefReader{views: map[string]*ingitdb.ViewDef{"test": view}},
		RecordsReader: fakeRecordsReader{},
		Writer:        errorWriter{err: writerErr},
	}
	result, err := builder.BuildViews(context.Background(), "/db", "", &ingitdb.CollectionDef{}, &ingitdb.Definition{})
	if err != nil {
		t.Fatalf("BuildViews should not return error on write failure: %v", err)
	}
	if len(result.Errors) != 1 {
		t.Fatalf("expected 1 error in result, got %d", len(result.Errors))
	}
	if !errors.Is(result.Errors[0], writerErr) {
		t.Errorf("expected error to be writer error, got: %v", result.Errors[0])
	}
	if result.FilesCreated+result.FilesUpdated != 0 {
		t.Errorf("expected 0 files created or updated, got created=%d updated=%d", result.FilesCreated, result.FilesUpdated)
	}
	if result.FilesUnchanged != 0 {
		t.Errorf("expected 0 files unchanged, got %d", result.FilesUnchanged)
	}
}

func TestSimpleViewBuilder_BuildViews_WriterReportsUnchanged(t *testing.T) {
	t.Parallel()

	view := &ingitdb.ViewDef{ID: "test", OrderBy: "", FileName: "test.md"}
	writer := &unchangedWriter{}
	builder := SimpleViewBuilder{
		DefReader:     fakeViewDefReader{views: map[string]*ingitdb.ViewDef{"test": view}},
		RecordsReader: fakeRecordsReader{},
		Writer:        writer,
	}
	result, err := builder.BuildViews(context.Background(), "/db", "", &ingitdb.CollectionDef{}, &ingitdb.Definition{})
	if err != nil {
		t.Fatalf("BuildViews: %v", err)
	}
	if result.FilesCreated+result.FilesUpdated != 0 {
		t.Errorf("expected 0 files created or updated, got created=%d updated=%d", result.FilesCreated, result.FilesUpdated)
	}
	if result.FilesUnchanged != 1 {
		t.Errorf("expected 1 file unchanged, got %d", result.FilesUnchanged)
	}
}

type unchangedWriter struct{}

func (w *unchangedWriter) WriteView(
	ctx context.Context,
	col *ingitdb.CollectionDef,
	view *ingitdb.ViewDef,
	records []ingitdb.IRecordEntry,
	outPath string,
) (WriteOutcome, error) {
	_ = ctx
	_ = col
	_ = view
	_ = records
	_ = outPath
	return WriteOutcomeUnchanged, nil
}

func TestResolveViewOutputPath_WithFileName(t *testing.T) {
	t.Parallel()

	col := &ingitdb.CollectionDef{DirPath: "/db/collection"}
	view := &ingitdb.ViewDef{ID: "test", FileName: "custom.md"}

	outPath := resolveViewOutputPath(col, view, "/db", "/db")
	expected := filepath.Join("/db", ingitdb.IngitdbDir, "collection", "custom.md")
	if outPath != expected {
		t.Errorf("expected %q, got %q", expected, outPath)
	}
}

func TestResolveViewOutputPath_WithoutFileNameWithID(t *testing.T) {
	t.Parallel()

	col := &ingitdb.CollectionDef{DirPath: "/db/collection"}
	view := &ingitdb.ViewDef{ID: "myview", FileName: ""}

	outPath := resolveViewOutputPath(col, view, "/db", "/db")
	expected := filepath.Join("/db", ingitdb.IngitdbDir, "collection", "myview.ingr")
	if outPath != expected {
		t.Errorf("expected %q, got %q", expected, outPath)
	}
}

func TestResolveViewOutputPath_WithoutFileNameWithoutID(t *testing.T) {
	t.Parallel()

	col := &ingitdb.CollectionDef{DirPath: "/db/collection"}
	view := &ingitdb.ViewDef{ID: "", FileName: ""}

	outPath := resolveViewOutputPath(col, view, "/db", "/db")
	expected := filepath.Join("/db", ingitdb.IngitdbDir, "collection", "view.ingr")
	if outPath != expected {
		t.Errorf("expected %q, got %q", expected, outPath)
	}
}

func TestReadAllRecords_YieldError(t *testing.T) {
	t.Parallel()

	yieldErr := errors.New("yield error")
	reader := errorRecordsReader{err: yieldErr}
	col := &ingitdb.CollectionDef{}

	_, err := readAllRecords(context.Background(), reader, "/db", col)
	if err == nil {
		t.Fatal("expected error from yield")
	}
	if !errors.Is(err, yieldErr) {
		t.Errorf("expected yield error, got: %v", err)
	}
}

func TestFilterColumns_NoColumns(t *testing.T) {
	t.Parallel()

	records := []ingitdb.IRecordEntry{
		ingitdb.RecordEntry{ID: "a", Data: map[string]any{"title": "A", "desc": "Description"}},
	}

	filtered := filterColumns(records, nil)
	if len(filtered) != 1 {
		t.Fatalf("expected 1 record, got %d", len(filtered))
	}
	d := filtered[0].GetData()
	if d["title"] != "A" {
		t.Errorf("expected all data to be preserved")
	}
	if d["desc"] != "Description" {
		t.Errorf("expected all data to be preserved")
	}
}

func TestFilterColumns_NilData(t *testing.T) {
	t.Parallel()

	records := []ingitdb.IRecordEntry{
		ingitdb.RecordEntry{ID: "a", Data: nil},
		ingitdb.RecordEntry{ID: "b", Data: map[string]any{"title": "B"}},
	}

	filtered := filterColumns(records, []string{"title"})
	if len(filtered) != 2 {
		t.Fatalf("expected 2 records, got %d", len(filtered))
	}
	if filtered[0].GetData() != nil {
		t.Errorf("expected nil data to remain nil")
	}
	if filtered[1].GetData()["title"] != "B" {
		t.Errorf("expected title to be preserved")
	}
}

// ---------------------------------------------------------------------------
// BuildView tests
// ---------------------------------------------------------------------------

func TestSimpleViewBuilder_BuildView_MissingRecordsReader(t *testing.T) {
	t.Parallel()

	builder := SimpleViewBuilder{
		RecordsReader: nil,
		Writer:        &capturingWriter{},
	}
	view := &ingitdb.ViewDef{ID: "test"}
	_, err := builder.BuildView(context.Background(), "/db", "", &ingitdb.CollectionDef{}, &ingitdb.Definition{}, view)
	if err == nil {
		t.Fatal("expected error for missing RecordsReader, got nil")
	}
}

func TestSimpleViewBuilder_BuildView_MissingWriter(t *testing.T) {
	t.Parallel()

	builder := SimpleViewBuilder{
		RecordsReader: fakeRecordsReader{},
		Writer:        nil,
	}
	view := &ingitdb.ViewDef{ID: "test"}
	_, err := builder.BuildView(context.Background(), "/db", "", &ingitdb.CollectionDef{}, &ingitdb.Definition{}, view)
	if err == nil {
		t.Fatal("expected error for missing Writer, got nil")
	}
}

func TestSimpleViewBuilder_BuildView_NonDefaultView(t *testing.T) {
	t.Parallel()

	record := ingitdb.NewMapRecordEntry("r1", map[string]any{"name": "Alice", "extra": "drop"})
	writer := &capturingWriter{}
	builder := SimpleViewBuilder{
		RecordsReader: fakeRecordsReader{records: []ingitdb.IRecordEntry{record}},
		Writer:        writer,
		Logf: func(format string, args ...any) {
			// log captured; no assertion needed
		},
	}

	col := &ingitdb.CollectionDef{
		ID:      "people",
		DirPath: "/db/people",
	}
	view := &ingitdb.ViewDef{
		ID:       "names",
		IsDefault: false,
		Columns:  []string{"name"},
		OrderBy:  "name asc",
		FileName: "names.json",
	}

	result, err := builder.BuildView(context.Background(), "/db", "/db", col, &ingitdb.Definition{}, view)
	if err != nil {
		t.Fatalf("BuildView() unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("BuildView() returned nil result")
	}
	if result.FilesCreated != 1 {
		t.Errorf("expected 1 file created, got %d", result.FilesCreated)
	}
	if writer.called != 1 {
		t.Errorf("expected writer called once, got %d", writer.called)
	}
	// column filtering: only "name" should remain
	if len(writer.lastRecords) != 1 {
		t.Fatalf("expected 1 record, got %d", len(writer.lastRecords))
	}
	data := writer.lastRecords[0].GetData()
	if _, ok := data["extra"]; ok {
		t.Error("expected 'extra' column to be filtered out")
	}
	if _, ok := data["name"]; !ok {
		t.Error("expected 'name' column to remain")
	}
}

func TestSimpleViewBuilder_BuildView_NonDefaultView_WriterError(t *testing.T) {
	t.Parallel()

	writerErr := errors.New("write failed")
	builder := SimpleViewBuilder{
		RecordsReader: fakeRecordsReader{},
		Writer:        errorWriter{err: writerErr},
	}
	col := &ingitdb.CollectionDef{ID: "col", DirPath: "/db/col"}
	view := &ingitdb.ViewDef{ID: "v", IsDefault: false, FileName: "out.json"}

	result, err := builder.BuildView(context.Background(), "/db", "/db", col, &ingitdb.Definition{}, view)
	if err != nil {
		t.Fatalf("BuildView() should not return error on writer error, got: %v", err)
	}
	if len(result.Errors) != 1 {
		t.Fatalf("expected 1 error in result, got %d", len(result.Errors))
	}
	if !errors.Is(result.Errors[0], writerErr) {
		t.Errorf("expected writerErr, got: %v", result.Errors[0])
	}
}

func TestSimpleViewBuilder_BuildView_NonDefaultView_TopTruncation(t *testing.T) {
	t.Parallel()

	records := []ingitdb.IRecordEntry{
		ingitdb.NewMapRecordEntry("1", map[string]any{"n": "A"}),
		ingitdb.NewMapRecordEntry("2", map[string]any{"n": "B"}),
		ingitdb.NewMapRecordEntry("3", map[string]any{"n": "C"}),
	}
	writer := &capturingWriter{}
	builder := SimpleViewBuilder{
		RecordsReader: fakeRecordsReader{records: records},
		Writer:        writer,
	}
	col := &ingitdb.CollectionDef{ID: "c", DirPath: "/db/c"}
	view := &ingitdb.ViewDef{ID: "v", Top: 2, FileName: "out.json"}

	result, err := builder.BuildView(context.Background(), "/db", "/db", col, &ingitdb.Definition{}, view)
	if err != nil {
		t.Fatalf("BuildView() unexpected error: %v", err)
	}
	if result.FilesCreated != 1 {
		t.Errorf("expected 1 file created, got %d", result.FilesCreated)
	}
	if len(writer.lastRecords) != 2 {
		t.Errorf("expected 2 records after Top=2, got %d", len(writer.lastRecords))
	}
}

func TestSimpleViewBuilder_BuildView_NonDefaultView_WriterUnchanged(t *testing.T) {
	t.Parallel()

	builder := SimpleViewBuilder{
		RecordsReader: fakeRecordsReader{},
		Writer:        &unchangedWriter{},
	}
	col := &ingitdb.CollectionDef{ID: "col", DirPath: "/db/col"}
	view := &ingitdb.ViewDef{ID: "v", FileName: "out.json"}

	result, err := builder.BuildView(context.Background(), "/db", "/db", col, &ingitdb.Definition{}, view)
	if err != nil {
		t.Fatalf("BuildView() unexpected error: %v", err)
	}
	if result.FilesUnchanged != 1 {
		t.Errorf("expected 1 unchanged, got %d", result.FilesUnchanged)
	}
}

func TestSimpleViewBuilder_BuildView_NonDefaultView_WriterUpdated(t *testing.T) {
	t.Parallel()

	updatedWriter := &updatedWriterImpl{}
	builder := SimpleViewBuilder{
		RecordsReader: fakeRecordsReader{},
		Writer:        updatedWriter,
	}
	col := &ingitdb.CollectionDef{ID: "col", DirPath: "/db/col"}
	view := &ingitdb.ViewDef{ID: "v", FileName: "out.json"}

	result, err := builder.BuildView(context.Background(), "/db", "/db", col, &ingitdb.Definition{}, view)
	if err != nil {
		t.Fatalf("BuildView() unexpected error: %v", err)
	}
	if result.FilesUpdated != 1 {
		t.Errorf("expected 1 updated, got %d", result.FilesUpdated)
	}
}

type updatedWriterImpl struct{}

func (w *updatedWriterImpl) WriteView(
	_ context.Context,
	_ *ingitdb.CollectionDef,
	_ *ingitdb.ViewDef,
	_ []ingitdb.IRecordEntry,
	_ string,
) (WriteOutcome, error) {
	return WriteOutcomeUpdated, nil
}

func TestSimpleViewBuilder_BuildView_DefaultView(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	col := &ingitdb.CollectionDef{
		ID:           "items",
		DirPath:      filepath.Join(tmpDir, "items"),
		ColumnsOrder: []string{"$ID", "name"},
	}
	view := &ingitdb.ViewDef{
		ID:        ingitdb.DefaultViewID,
		IsDefault: true,
		Format:    "json",
	}
	records := []ingitdb.IRecordEntry{
		ingitdb.NewMapRecordEntry("1", map[string]any{"$ID": "1", "name": "Widget"}),
	}

	builder := SimpleViewBuilder{
		RecordsReader: fakeRecordsReader{records: records},
		Writer:        &capturingWriter{},
	}

	result, err := builder.BuildView(context.Background(), tmpDir, "", col, &ingitdb.Definition{}, view)
	if err != nil {
		t.Fatalf("BuildView() unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("BuildView() returned nil result")
	}
	// Default view writes directly to disk via buildDefaultView
	if result.FilesCreated < 1 {
		t.Errorf("expected at least 1 file created for default view, got %d", result.FilesCreated)
	}
	if len(result.Errors) > 0 {
		t.Errorf("expected no errors, got: %v", result.Errors)
	}
}

func TestSimpleViewBuilder_BuildView_RecordsReaderError(t *testing.T) {
	t.Parallel()

	readerErr := errors.New("read failed")
	builder := SimpleViewBuilder{
		RecordsReader: errorRecordsReader{err: readerErr},
		Writer:        &capturingWriter{},
	}
	col := &ingitdb.CollectionDef{ID: "col", DirPath: "/db/col"}
	view := &ingitdb.ViewDef{ID: "v", FileName: "out.json"}

	_, err := builder.BuildView(context.Background(), "/db", "/db", col, &ingitdb.Definition{}, view)
	if err == nil {
		t.Fatal("expected error from records reader, got nil")
	}
	if !errors.Is(err, readerErr) {
		t.Errorf("expected readerErr, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// displayRelPath tests
// ---------------------------------------------------------------------------

func TestDisplayRelPath_EmptyRepoRoot(t *testing.T) {
	t.Parallel()

	got := displayRelPath("", "/some/absolute/path/file.json")
	want := "/some/absolute/path/file.json"
	if got != want {
		t.Errorf("displayRelPath(\"\", %q) = %q, want %q", want, got, want)
	}
}

func TestDisplayRelPath_ReturnsRelative(t *testing.T) {
	t.Parallel()

	got := displayRelPath("/repo/root", "/repo/root/sub/file.json")
	want := filepath.Join("sub", "file.json")
	if got != want {
		t.Errorf("displayRelPath = %q, want %q", got, want)
	}
}

func TestDisplayRelPath_SameDir(t *testing.T) {
	t.Parallel()

	got := displayRelPath("/repo/root", "/repo/root/file.json")
	want := "file.json"
	if got != want {
		t.Errorf("displayRelPath = %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// resolveViewOutputPath — missing branches
// ---------------------------------------------------------------------------

func TestResolveViewOutputPath_TemplateWithFileName(t *testing.T) {
	t.Parallel()

	col := &ingitdb.CollectionDef{ID: "docs", DirPath: "/db/docs"}
	view := &ingitdb.ViewDef{
		ID:       "readme",
		Template: "tmpl.gotmpl",
		FileName: "README.md",
	}

	got := resolveViewOutputPath(col, view, "/db", "/db")
	want := filepath.Join("/db/docs", "README.md")
	if got != want {
		t.Errorf("resolveViewOutputPath = %q, want %q", got, want)
	}
}

func TestResolveViewOutputPath_TemplateWithoutFileName(t *testing.T) {
	t.Parallel()

	col := &ingitdb.CollectionDef{ID: "docs", DirPath: "/db/docs"}
	view := &ingitdb.ViewDef{
		ID:       "readme",
		Template: "tmpl.gotmpl",
		FileName: "",
	}

	got := resolveViewOutputPath(col, view, "/db", "/db")
	want := filepath.Join("/db/docs", "readme.md")
	if got != want {
		t.Errorf("resolveViewOutputPath = %q, want %q", got, want)
	}
}

func TestResolveViewOutputPath_TemplateWithoutFileNameOrID(t *testing.T) {
	t.Parallel()

	col := &ingitdb.CollectionDef{ID: "docs", DirPath: "/db/docs"}
	view := &ingitdb.ViewDef{
		ID:       "",
		Template: "tmpl.gotmpl",
		FileName: "",
	}

	got := resolveViewOutputPath(col, view, "/db", "/db")
	want := filepath.Join("/db/docs", "view.md")
	if got != want {
		t.Errorf("resolveViewOutputPath = %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// compareAny, toComparableInt, toComparableFloat — table-driven tests
// ---------------------------------------------------------------------------

func TestCompareAny(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a    any
		b    any
		want int
	}{
		// nil cases
		{name: "nil_nil", a: nil, b: nil, want: 0},
		{name: "nil_string", a: nil, b: "x", want: -1},
		{name: "string_nil", a: "x", b: nil, want: 1},

		// string comparisons
		{name: "string_less", a: "apple", b: "banana", want: -1},
		{name: "string_equal", a: "apple", b: "apple", want: 0},
		{name: "string_greater", a: "zebra", b: "apple", want: 1},

		// int comparisons
		{name: "int_less", a: 1, b: 2, want: -1},
		{name: "int_equal", a: 42, b: 42, want: 0},
		{name: "int_greater", a: 10, b: 5, want: 1},

		// int64 comparisons
		{name: "int64_less", a: int64(1), b: int64(2), want: -1},
		{name: "int64_equal", a: int64(100), b: int64(100), want: 0},
		{name: "int64_greater", a: int64(99), b: int64(1), want: 1},

		// float64 comparisons
		{name: "float_less", a: 1.5, b: 2.5, want: -1},
		{name: "float_equal", a: 3.14, b: 3.14, want: 0},
		{name: "float_greater", a: 9.9, b: 1.1, want: 1},

		// cross-type: int vs int64
		{name: "int_vs_int64_less", a: 1, b: int64(2), want: -1},
		{name: "int_vs_int64_equal", a: 5, b: int64(5), want: 0},
		{name: "int_vs_int64_greater", a: 10, b: int64(3), want: 1},

		// cross-type: int64 vs int
		{name: "int64_vs_int_less", a: int64(1), b: 2, want: -1},
		{name: "int64_vs_int_equal", a: int64(7), b: 7, want: 0},

		// cross-type: float64 vs int
		{name: "float_vs_int_less", a: 1.0, b: 2, want: -1},
		{name: "float_vs_int_equal", a: 5.0, b: 5, want: 0},
		{name: "float_vs_int_greater", a: 9.0, b: 3, want: 1},

		// cross-type: float64 vs int64
		{name: "float_vs_int64", a: 2.0, b: int64(3), want: -1},

		// fallback string comparison (mismatched types)
		{name: "mixed_types_fallback_equal", a: true, b: true, want: 0},
		{name: "string_vs_int_fallback_less", a: "10", b: 20, want: -1},   // "10" < "20" lexicographically
		{name: "fallback_greater", a: true, b: false, want: 1},             // "true" > "false" lexicographically
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := compareAny(tc.a, tc.b)
			// Normalise to -1/0/1 for the check
			if tc.want < 0 && got >= 0 {
				t.Errorf("compareAny(%v, %v) = %d, want negative", tc.a, tc.b, got)
			} else if tc.want > 0 && got <= 0 {
				t.Errorf("compareAny(%v, %v) = %d, want positive", tc.a, tc.b, got)
			} else if tc.want == 0 && got != 0 {
				t.Errorf("compareAny(%v, %v) = %d, want 0", tc.a, tc.b, got)
			}
		})
	}
}

func TestToComparableInt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   any
		wantVal int64
		wantOK  bool
	}{
		{name: "int", input: int(42), wantVal: 42, wantOK: true},
		{name: "int64", input: int64(100), wantVal: 100, wantOK: true},
		{name: "float64", input: float64(3.7), wantVal: 3, wantOK: true},
		{name: "string_not_ok", input: "42", wantVal: 0, wantOK: false},
		{name: "nil_not_ok", input: nil, wantVal: 0, wantOK: false},
		{name: "bool_not_ok", input: true, wantVal: 0, wantOK: false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := toComparableInt(tc.input)
			if ok != tc.wantOK {
				t.Errorf("toComparableInt(%v) ok = %v, want %v", tc.input, ok, tc.wantOK)
			}
			if ok && got != tc.wantVal {
				t.Errorf("toComparableInt(%v) = %d, want %d", tc.input, got, tc.wantVal)
			}
		})
	}
}

func TestToComparableFloat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   any
		wantVal float64
		wantOK  bool
	}{
		{name: "float64", input: float64(3.14), wantVal: 3.14, wantOK: true},
		{name: "int", input: int(5), wantVal: 5.0, wantOK: true},
		{name: "int64", input: int64(7), wantVal: 7.0, wantOK: true},
		{name: "string_not_ok", input: "3.14", wantVal: 0, wantOK: false},
		{name: "nil_not_ok", input: nil, wantVal: 0, wantOK: false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := toComparableFloat(tc.input)
			if ok != tc.wantOK {
				t.Errorf("toComparableFloat(%v) ok = %v, want %v", tc.input, ok, tc.wantOK)
			}
			if ok && got != tc.wantVal {
				t.Errorf("toComparableFloat(%v) = %f, want %f", tc.input, got, tc.wantVal)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// sortRecordsByOrderBy — missing branch: recordFieldValue nil data
// ---------------------------------------------------------------------------

func TestSortRecordsByOrderBy_NilDataRecord(t *testing.T) {
	t.Parallel()

	records := []ingitdb.IRecordEntry{
		ingitdb.RecordEntry{ID: "nodata", Data: nil},
		ingitdb.NewMapRecordEntry("b", map[string]any{"score": 10}),
	}

	// Should not panic; nil-data record gets nil field value
	sortRecordsByOrderBy(records, "score asc")
}

func TestSortRecordsByOrderBy_EmptyOrderBy(t *testing.T) {
	t.Parallel()

	records := []ingitdb.IRecordEntry{
		ingitdb.NewMapRecordEntry("b", map[string]any{"score": 2}),
		ingitdb.NewMapRecordEntry("a", map[string]any{"score": 1}),
	}

	// Empty orderBy is a no-op — order should remain unchanged
	sortRecordsByOrderBy(records, "")
	if records[0].GetID() != "b" {
		t.Errorf("expected order unchanged, got first record ID %q", records[0].GetID())
	}
}

// ---------------------------------------------------------------------------
// BuildViews — FilesUpdated and Logf for non-default views
// ---------------------------------------------------------------------------

func TestBuildViews_NonDefaultView_Updated(t *testing.T) {
	t.Parallel()

	view := &ingitdb.ViewDef{ID: "report", FileName: "report.md"}
	builder := SimpleViewBuilder{
		DefReader:     fakeViewDefReader{views: map[string]*ingitdb.ViewDef{"report": view}},
		RecordsReader: fakeRecordsReader{},
		Writer:        &updatedWriterImpl{},
	}
	col := &ingitdb.CollectionDef{ID: "col", DirPath: "/db/col"}
	result, err := builder.BuildViews(context.Background(), "/db", "/db", col, &ingitdb.Definition{})
	if err != nil {
		t.Fatalf("BuildViews: %v", err)
	}
	if result.FilesUpdated != 1 {
		t.Errorf("expected FilesUpdated=1, got %d", result.FilesUpdated)
	}
	if result.FilesCreated != 0 || result.FilesUnchanged != 0 {
		t.Errorf("expected only FilesUpdated=1, got created=%d unchanged=%d",
			result.FilesCreated, result.FilesUnchanged)
	}
}

func TestBuildViews_NonDefaultView_Logf(t *testing.T) {
	t.Parallel()

	view := &ingitdb.ViewDef{ID: "report", FileName: "report.md"}
	logCalls := 0
	builder := SimpleViewBuilder{
		DefReader:     fakeViewDefReader{views: map[string]*ingitdb.ViewDef{"report": view}},
		RecordsReader: fakeRecordsReader{},
		Writer:        &capturingWriter{},
		Logf: func(format string, args ...any) {
			logCalls++
		},
	}
	col := &ingitdb.CollectionDef{ID: "col", DirPath: "/db/col"}
	_, err := builder.BuildViews(context.Background(), "/db", "/db", col, &ingitdb.Definition{})
	if err != nil {
		t.Fatalf("BuildViews: %v", err)
	}
	if logCalls == 0 {
		t.Error("expected Logf to be called for non-default view write")
	}
}

// ---------------------------------------------------------------------------
// buildDefaultView — IncludeHash and logf paths
// ---------------------------------------------------------------------------

func TestBuildDefaultView_IncludeHash(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	col := &ingitdb.CollectionDef{
		ID:           "items",
		DirPath:      filepath.Join(tmpDir, "items"),
		ColumnsOrder: []string{"$ID", "name"},
	}
	view := &ingitdb.ViewDef{
		ID:          ingitdb.DefaultViewID,
		IsDefault:   true,
		Format:      "ingr",
		IncludeHash: true,
	}
	records := []ingitdb.IRecordEntry{
		ingitdb.NewMapRecordEntry("1", map[string]any{"$ID": "1", "name": "Widget"}),
	}

	created, _, _, errs := buildDefaultView(tmpDir, "", col, &ingitdb.Definition{}, view, records, nil, defaultFSops())
	if len(errs) > 0 {
		t.Fatalf("buildDefaultView: %v", errs)
	}
	if created != 1 {
		t.Fatalf("expected 1 created, got %d", created)
	}

	outPath := filepath.Join(tmpDir, ingitdb.IngitdbDir, "items", "items.ingr")
	content, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if !strings.Contains(string(content), "# sha256:") {
		t.Error("expected '# sha256:' hash line in INGR output when IncludeHash=true")
	}
}

func TestBuildDefaultView_LogfCalledOnCreated(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	col := &ingitdb.CollectionDef{
		ID:           "data",
		DirPath:      filepath.Join(tmpDir, "data"),
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

	logCalls := 0
	logf := func(format string, args ...any) { logCalls++ }

	created, _, _, errs := buildDefaultView(tmpDir, "", col, &ingitdb.Definition{}, view, records, logf, defaultFSops())
	if len(errs) > 0 {
		t.Fatalf("buildDefaultView: %v", errs)
	}
	if created != 1 {
		t.Fatalf("expected 1 created, got %d", created)
	}
	if logCalls == 0 {
		t.Error("expected logf to be called when a new file is created")
	}
}

func TestBuildDefaultView_LogfCalledOnUnchanged(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	col := &ingitdb.CollectionDef{
		ID:           "data",
		DirPath:      filepath.Join(tmpDir, "data"),
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

	// First run: create the file (no logf needed).
	_, _, _, errs := buildDefaultView(tmpDir, "", col, &ingitdb.Definition{}, view, records, nil, defaultFSops())
	if len(errs) > 0 {
		t.Fatalf("first buildDefaultView: %v", errs)
	}

	// Second run with identical records: unchanged path — logf should be called.
	logCalls := 0
	logf := func(format string, args ...any) { logCalls++ }

	_, _, unchanged, errs := buildDefaultView(tmpDir, "", col, &ingitdb.Definition{}, view, records, logf, defaultFSops())
	if len(errs) > 0 {
		t.Fatalf("second buildDefaultView: %v", errs)
	}
	if unchanged != 1 {
		t.Fatalf("expected 1 unchanged, got %d", unchanged)
	}
	if logCalls == 0 {
		t.Error("expected logf to be called when file content is unchanged")
	}
}

func TestBuildDefaultView_LogfCalledOnUpdated(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	col := &ingitdb.CollectionDef{
		ID:           "data",
		DirPath:      filepath.Join(tmpDir, "data"),
		ColumnsOrder: []string{"$ID", "value"},
	}
	view := &ingitdb.ViewDef{
		ID:        ingitdb.DefaultViewID,
		IsDefault: true,
		Format:    "json",
	}
	records1 := []ingitdb.IRecordEntry{
		ingitdb.NewMapRecordEntry("1", map[string]any{"$ID": "1", "value": "original"}),
	}
	records2 := []ingitdb.IRecordEntry{
		ingitdb.NewMapRecordEntry("1", map[string]any{"$ID": "1", "value": "changed"}),
	}

	// First run: create the file.
	_, _, _, errs := buildDefaultView(tmpDir, "", col, &ingitdb.Definition{}, view, records1, nil, defaultFSops())
	if len(errs) > 0 {
		t.Fatalf("first buildDefaultView: %v", errs)
	}

	// Second run with different records: updated path — logf should be called.
	logCalls := 0
	logf := func(format string, args ...any) { logCalls++ }

	_, updated, _, errs := buildDefaultView(tmpDir, "", col, &ingitdb.Definition{}, view, records2, logf, defaultFSops())
	if len(errs) > 0 {
		t.Fatalf("second buildDefaultView: %v", errs)
	}
	if updated != 1 {
		t.Fatalf("expected 1 updated, got %d", updated)
	}
	if logCalls == 0 {
		t.Error("expected logf to be called when file content is updated")
	}
}

// ---------------------------------------------------------------------------
// buildDefaultView — format error path (formatExportBatch returns an error)
// ---------------------------------------------------------------------------

func TestBuildDefaultView_FormatError(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	col := &ingitdb.CollectionDef{
		ID:           "orders",
		DirPath:      filepath.Join(tmpDir, "orders"),
		ColumnsOrder: []string{"$ID", "bad"},
	}
	view := &ingitdb.ViewDef{
		ID:        ingitdb.DefaultViewID,
		IsDefault: true,
		Format:    "jsonl", // formatJSONL will call json.Marshal which fails on channels
	}
	// A channel value cannot be JSON-marshalled → triggers the format error branch.
	records := []ingitdb.IRecordEntry{
		ingitdb.RecordEntry{ID: "1", Data: map[string]any{"$ID": "1", "bad": make(chan int)}},
	}

	_, _, _, errs := buildDefaultView(tmpDir, "", col, &ingitdb.Definition{}, view, records, nil, defaultFSops())
	if len(errs) == 0 {
		t.Fatal("expected format error for unmarshalable channel value, got none")
	}
}

// ---------------------------------------------------------------------------
// buildFKViews — format error path
// ---------------------------------------------------------------------------

func TestBuildFKViews_FormatError(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	col := &ingitdb.CollectionDef{
		ID:           "orders",
		DirPath:      filepath.Join(tmpDir, "orders"),
		ColumnsOrder: []string{"$ID", "customer", "bad"},
		Columns: map[string]*ingitdb.ColumnDef{
			"customer": {Type: ingitdb.ColumnTypeString, ForeignKey: "customers"},
			"bad":      {Type: ingitdb.ColumnTypeString},
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
		Format:    "jsonl", // formatJSONL → json.Marshal fails on channel value
	}
	// The "bad" column (a channel) is NOT the FK column, so it ends up in fkExportColumns.
	// formatExportBatch("jsonl",...) then fails to marshal the channel value.
	records := []ingitdb.IRecordEntry{
		ingitdb.RecordEntry{ID: "1", Data: map[string]any{"$ID": "1", "customer": "alice", "bad": make(chan int)}},
	}

	_, _, _, errs := buildFKViews(tmpDir, "", col, def, view, records, nil, defaultFSops())
	if len(errs) == 0 {
		t.Fatal("expected format error for unmarshalable channel value in FK view, got none")
	}
}

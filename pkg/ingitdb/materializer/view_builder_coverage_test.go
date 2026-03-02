package materializer

import (
	"context"
	"errors"
	"path/filepath"
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

	records := []ingitdb.RecordEntry{
		{ID: "a", Data: map[string]any{"title": "A", "desc": "Description"}},
	}

	filtered := filterColumns(records, nil)
	if len(filtered) != 1 {
		t.Fatalf("expected 1 record, got %d", len(filtered))
	}
	if filtered[0].Data["title"] != "A" {
		t.Errorf("expected all data to be preserved")
	}
	if filtered[0].Data["desc"] != "Description" {
		t.Errorf("expected all data to be preserved")
	}
}

func TestFilterColumns_NilData(t *testing.T) {
	t.Parallel()

	records := []ingitdb.RecordEntry{
		{ID: "a", Data: nil},
		{ID: "b", Data: map[string]any{"title": "B"}},
	}

	filtered := filterColumns(records, []string{"title"})
	if len(filtered) != 2 {
		t.Fatalf("expected 2 records, got %d", len(filtered))
	}
	if filtered[0].Data != nil {
		t.Errorf("expected nil data to remain nil")
	}
	if filtered[1].Data["title"] != "B" {
		t.Errorf("expected title to be preserved")
	}
}

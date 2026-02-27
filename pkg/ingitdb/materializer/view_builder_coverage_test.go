package materializer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

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
	yield func(ingitdb.RecordEntry) error,
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
	records []ingitdb.RecordEntry,
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
	records []ingitdb.RecordEntry,
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
		{Key: "a", Data: map[string]any{"title": "A", "desc": "Description"}},
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
		{Key: "a", Data: nil},
		{Key: "b", Data: map[string]any{"title": "B"}},
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

func TestOrderRecords_EmptyOrderBy(t *testing.T) {
	t.Parallel()

	records := []ingitdb.RecordEntry{
		{Key: "b", Data: map[string]any{"title": "B"}},
		{Key: "a", Data: map[string]any{"title": "A"}},
	}

	err := orderRecords(records, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Order should remain unchanged
	if records[0].Key != "b" {
		t.Errorf("expected order to remain unchanged")
	}
}

func TestOrderRecords_LastModified_Asc(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file1 := filepath.Join(dir, "file1.json")
	file2 := filepath.Join(dir, "file2.json")

	// Create file1 first
	if err := os.WriteFile(file1, []byte("{}"), 0o644); err != nil {
		t.Fatalf("write file1: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	// Create file2 later
	if err := os.WriteFile(file2, []byte("{}"), 0o644); err != nil {
		t.Fatalf("write file2: %v", err)
	}

	records := []ingitdb.RecordEntry{
		{Key: "b", FilePath: file2, Data: map[string]any{"title": "B"}},
		{Key: "a", FilePath: file1, Data: map[string]any{"title": "A"}},
	}

	err := orderRecords(records, "$last_modified asc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// file1 was created first, so it should be first
	if records[0].Key != "a" {
		t.Errorf("expected record a to be first, got %s", records[0].Key)
	}
	if records[1].Key != "b" {
		t.Errorf("expected record b to be second, got %s", records[1].Key)
	}
}

func TestOrderRecords_LastModified_Desc(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file1 := filepath.Join(dir, "file1.json")
	file2 := filepath.Join(dir, "file2.json")

	// Create file1 first
	if err := os.WriteFile(file1, []byte("{}"), 0o644); err != nil {
		t.Fatalf("write file1: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	// Create file2 later
	if err := os.WriteFile(file2, []byte("{}"), 0o644); err != nil {
		t.Fatalf("write file2: %v", err)
	}

	records := []ingitdb.RecordEntry{
		{Key: "a", FilePath: file1, Data: map[string]any{"title": "A"}},
		{Key: "b", FilePath: file2, Data: map[string]any{"title": "B"}},
	}

	err := orderRecords(records, "$last_modified desc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// file2 was created last, so it should be first in desc order
	if records[0].Key != "b" {
		t.Errorf("expected record b to be first, got %s", records[0].Key)
	}
	if records[1].Key != "a" {
		t.Errorf("expected record a to be second, got %s", records[1].Key)
	}
}

func TestOrderRecords_LastModified_StatError(t *testing.T) {
	t.Parallel()

	records := []ingitdb.RecordEntry{
		{Key: "a", FilePath: "/nonexistent/file.json", Data: map[string]any{"title": "A"}},
	}

	err := orderRecords(records, "$last_modified")
	if err == nil {
		t.Fatal("expected error when stat fails")
	}
}

func TestOrderRecords_FieldAsc(t *testing.T) {
	t.Parallel()

	records := []ingitdb.RecordEntry{
		{Key: "c", Data: map[string]any{"priority": 3}},
		{Key: "a", Data: map[string]any{"priority": 1}},
		{Key: "b", Data: map[string]any{"priority": 2}},
	}

	err := orderRecords(records, "priority")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if records[0].Key != "a" || records[1].Key != "b" || records[2].Key != "c" {
		t.Errorf("expected records ordered by priority asc, got: %v", []string{records[0].Key, records[1].Key, records[2].Key})
	}
}

func TestOrderRecords_FieldDesc(t *testing.T) {
	t.Parallel()

	records := []ingitdb.RecordEntry{
		{Key: "a", Data: map[string]any{"priority": 1}},
		{Key: "b", Data: map[string]any{"priority": 2}},
		{Key: "c", Data: map[string]any{"priority": 3}},
	}

	err := orderRecords(records, "priority DESC")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if records[0].Key != "c" || records[1].Key != "b" || records[2].Key != "a" {
		t.Errorf("expected records ordered by priority desc, got: %v", []string{records[0].Key, records[1].Key, records[2].Key})
	}
}

func TestOrderKey_LastModified(t *testing.T) {
	t.Parallel()

	now := time.Now()
	record := ingitdb.RecordEntry{
		Key:  "test",
		Data: map[string]any{"title": "Test"},
	}
	spec := orderBySpec{Field: "$last_modified"}
	lastModified := []time.Time{now}

	key := orderKey(record, spec, lastModified, 0)
	if key != now {
		t.Errorf("expected time key, got %v", key)
	}
}

func TestOrderKey_Field(t *testing.T) {
	t.Parallel()

	record := ingitdb.RecordEntry{
		Key:  "test",
		Data: map[string]any{"title": "Test Title"},
	}
	spec := orderBySpec{Field: "title"}

	key := orderKey(record, spec, nil, 0)
	if key != "Test Title" {
		t.Errorf("expected title value, got %v", key)
	}
}

func TestOrderKey_NilData(t *testing.T) {
	t.Parallel()

	record := ingitdb.RecordEntry{
		Key:  "test",
		Data: nil,
	}
	spec := orderBySpec{Field: "title"}

	key := orderKey(record, spec, nil, 0)
	if key != nil {
		t.Errorf("expected nil key, got %v", key)
	}
}

func TestParseOrderBy_EmptyString(t *testing.T) {
	t.Parallel()

	spec := parseOrderBy("")
	if spec.Field != "" {
		t.Errorf("expected empty field, got %q", spec.Field)
	}
	if spec.Desc {
		t.Errorf("expected Desc to be false")
	}
}

func TestParseOrderBy_OnlyField(t *testing.T) {
	t.Parallel()

	spec := parseOrderBy("title")
	if spec.Field != "title" {
		t.Errorf("expected field title, got %q", spec.Field)
	}
	if spec.Desc {
		t.Errorf("expected Desc to be false")
	}
}

func TestParseOrderBy_FieldDesc(t *testing.T) {
	t.Parallel()

	spec := parseOrderBy("title desc")
	if spec.Field != "title" {
		t.Errorf("expected field title, got %q", spec.Field)
	}
	if !spec.Desc {
		t.Errorf("expected Desc to be true")
	}
}

func TestParseOrderBy_FieldDescMixedCase(t *testing.T) {
	t.Parallel()

	spec := parseOrderBy("title DeSc")
	if spec.Field != "title" {
		t.Errorf("expected field title, got %q", spec.Field)
	}
	if !spec.Desc {
		t.Errorf("expected Desc to be true for mixed case DESC")
	}
}

func TestParseOrderBy_FieldAsc(t *testing.T) {
	t.Parallel()

	spec := parseOrderBy("title asc")
	if spec.Field != "title" {
		t.Errorf("expected field title, got %q", spec.Field)
	}
	if spec.Desc {
		t.Errorf("expected Desc to be false for explicit asc")
	}
}

func TestCompareValues_TimeTimes(t *testing.T) {
	t.Parallel()

	now := time.Now()
	later := now.Add(time.Hour)

	if compareValues(now, later) != -1 {
		t.Error("expected now to be less than later")
	}
	if compareValues(later, now) != 1 {
		t.Error("expected later to be greater than now")
	}
	if compareValues(now, now) != 0 {
		t.Error("expected equal times to compare equal")
	}
}

func TestCompareValues_TimeToNonTime(t *testing.T) {
	t.Parallel()

	now := time.Now()
	result := compareValues(now, "string")
	if result != 1 {
		t.Errorf("expected time to be greater than non-time, got %d", result)
	}
}

func TestCompareValues_Strings(t *testing.T) {
	t.Parallel()

	if compareValues("apple", "banana") != -1 {
		t.Error("expected apple < banana")
	}
	if compareValues("banana", "apple") != 1 {
		t.Error("expected banana > apple")
	}
	if compareValues("apple", "apple") != 0 {
		t.Error("expected apple == apple")
	}
}

func TestCompareValues_StringToNonString(t *testing.T) {
	t.Parallel()

	result := compareValues("string", 123)
	if result != 1 {
		t.Errorf("expected string to be greater than int, got %d", result)
	}
}

func TestCompareValues_IntToNonInt(t *testing.T) {
	t.Parallel()

	result := compareValues(123, "string")
	if result != 1 {
		t.Errorf("expected int to be greater than non-numeric string, got %d", result)
	}
}

func TestCompareValues_Int64ToNonInt64(t *testing.T) {
	t.Parallel()

	result := compareValues(int64(123), "string")
	if result != 1 {
		t.Errorf("expected int64 to be greater than non-numeric string, got %d", result)
	}
}

func TestCompareValues_Float64ToNonFloat64(t *testing.T) {
	t.Parallel()

	result := compareValues(123.45, "string")
	if result != 1 {
		t.Errorf("expected float64 to be greater than non-numeric string, got %d", result)
	}
}

func TestCompareValues_DefaultCase(t *testing.T) {
	t.Parallel()

	type customType struct {
		value int
	}

	a := customType{value: 1}
	b := customType{value: 2}

	// Default case uses fmt.Sprint comparison
	result := compareValues(a, b)
	// "{1}" vs "{2}" string comparison
	aStr := fmt.Sprint(a)
	bStr := fmt.Sprint(b)
	if aStr < bStr && result >= 0 {
		t.Errorf("expected string comparison to work for custom types")
	}
}

func TestCompareValues_NilValues(t *testing.T) {
	t.Parallel()

	// Both nil should compare as equal strings
	result := compareValues(nil, nil)
	if result != 0 {
		t.Errorf("expected nil == nil, got %d", result)
	}
}

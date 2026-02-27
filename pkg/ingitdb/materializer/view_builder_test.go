package materializer

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

type fakeViewDefReader struct {
	views map[string]*ingitdb.ViewDef
}

func (f fakeViewDefReader) ReadViewDefs(string) (map[string]*ingitdb.ViewDef, error) {
	return f.views, nil
}

type fakeRecordsReader struct {
	records []ingitdb.RecordEntry
}

func (f fakeRecordsReader) ReadRecords(
	ctx context.Context,
	dbPath string,
	col *ingitdb.CollectionDef,
	yield func(ingitdb.RecordEntry) error,
) error {
	_ = ctx
	_ = dbPath
	_ = col
	for _, record := range f.records {
		if err := yield(record); err != nil {
			return err
		}
	}
	return nil
}

type capturingWriter struct {
	lastOutPath string
	lastRecords []ingitdb.RecordEntry
	called      int
}

func (w *capturingWriter) WriteView(
	ctx context.Context,
	col *ingitdb.CollectionDef,
	view *ingitdb.ViewDef,
	records []ingitdb.RecordEntry,
	outPath string,
) (bool, error) {
	_ = ctx
	_ = col
	_ = view
	w.called++
	w.lastOutPath = outPath
	w.lastRecords = make([]ingitdb.RecordEntry, len(records))
	copy(w.lastRecords, records)
	return true, nil
}

func TestSimpleViewBuilder_BuildViewsOrdersRecords(t *testing.T) {
	t.Parallel()

	col := &ingitdb.CollectionDef{
		ID:      "todo.tags",
		DirPath: "/tmp/tags",
	}
	view := &ingitdb.ViewDef{
		ID:       "README",
		OrderBy:  "title desc",
		Top:      2,
		Columns:  []string{"title"},
		FileName: "README.md",
	}
	writer := &capturingWriter{}
	builder := SimpleViewBuilder{
		DefReader: fakeViewDefReader{views: map[string]*ingitdb.ViewDef{"README": view}},
		RecordsReader: fakeRecordsReader{records: []ingitdb.RecordEntry{
			{Key: "a", Data: map[string]any{"title": "Alpha", "extra": "x"}},
			{Key: "c", Data: map[string]any{"title": "Charlie", "extra": "y"}},
			{Key: "b", Data: map[string]any{"title": "Bravo", "extra": "z"}},
		}},
		Writer: writer,
	}

	result, err := builder.BuildViews(context.Background(), "/db", "", col, &ingitdb.Definition{})
	if err != nil {
		t.Fatalf("BuildViews: %v", err)
	}
	if result.FilesWritten != 1 {
		t.Fatalf("expected 1 file written, got %d", result.FilesWritten)
	}
	if writer.called != 1 {
		t.Fatalf("expected writer called once, got %d", writer.called)
	}
	expectedPath := filepath.Join(col.DirPath, "README.md")
	if writer.lastOutPath != expectedPath {
		t.Fatalf("expected out path %q, got %q", expectedPath, writer.lastOutPath)
	}
	if len(writer.lastRecords) != 2 {
		t.Fatalf("expected 2 records after top filter, got %d", len(writer.lastRecords))
	}
	order := []string{
		writer.lastRecords[0].Data["title"].(string),
		writer.lastRecords[1].Data["title"].(string),
	}
	if !reflect.DeepEqual(order, []string{"Charlie", "Bravo"}) {
		t.Fatalf("unexpected order: %v", order)
	}
	for _, record := range writer.lastRecords {
		if _, ok := record.Data["extra"]; ok {
			t.Fatalf("expected extra column to be filtered out")
		}
		if _, ok := record.Data["title"]; !ok {
			t.Fatalf("expected title column to remain")
		}
	}
}

func TestSimpleViewBuilder_MissingDependencies(t *testing.T) {
	t.Parallel()

	builder := SimpleViewBuilder{}
	_, err := builder.BuildViews(context.Background(), "/db", "", &ingitdb.CollectionDef{}, &ingitdb.Definition{})
	if err == nil {
		t.Fatalf("expected error for missing dependencies")
	}
}

func TestCompareInt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a    int
		b    int
		want int
	}{
		{"less than", 1, 2, -1},
		{"greater than", 2, 1, 1},
		{"equal", 1, 1, 0},
		{"negative numbers", -5, -3, -1},
		{"mixed signs", -1, 1, -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compareInt(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("compareInt(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestCompareInt64(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a    int64
		b    int64
		want int
	}{
		{"less than", int64(1), int64(2), -1},
		{"greater than", int64(2), int64(1), 1},
		{"equal", int64(1), int64(1), 0},
		{"large numbers", int64(1e15), int64(1e15 + 1), -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compareInt64(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("compareInt64(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestCompareFloat64(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a    float64
		b    float64
		want int
	}{
		{"less than", 1.5, 2.5, -1},
		{"greater than", 2.5, 1.5, 1},
		{"equal", 1.5, 1.5, 0},
		{"negative floats", -1.5, -0.5, -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compareFloat64(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("compareFloat64(%f, %f) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestToInt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input any
		want  int
		ok    bool
	}{
		{"int", 42, 42, true},
		{"int64", int64(100), 100, true},
		{"float64", float64(3.14), 3, true},
		{"string", "not a number", 0, false},
		{"nil", nil, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := toInt(tt.input)
			if ok != tt.ok {
				t.Errorf("toInt(%v) ok = %v, want %v", tt.input, ok, tt.ok)
			}
			if got != tt.want {
				t.Errorf("toInt(%v) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestToInt64(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input any
		want  int64
		ok    bool
	}{
		{"int", 42, int64(42), true},
		{"int64", int64(100), int64(100), true},
		{"float64", float64(3.14), int64(3), true},
		{"string", "not a number", int64(0), false},
		{"bool", true, int64(0), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := toInt64(tt.input)
			if ok != tt.ok {
				t.Errorf("toInt64(%v) ok = %v, want %v", tt.input, ok, tt.ok)
			}
			if got != tt.want {
				t.Errorf("toInt64(%v) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestToFloat64(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input any
		want  float64
		ok    bool
	}{
		{"float64", float64(3.14), 3.14, true},
		{"float32", float32(2.5), 2.5, true},
		{"int", 42, float64(42), true},
		{"int64", int64(100), float64(100), true},
		{"string", "not a number", 0, false},
		{"bool", false, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := toFloat64(tt.input)
			if ok != tt.ok {
				t.Errorf("toFloat64(%v) ok = %v, want %v", tt.input, ok, tt.ok)
			}
			if got != tt.want {
				t.Errorf("toFloat64(%v) = %f, want %f", tt.input, got, tt.want)
			}
		})
	}
}

func TestCompareValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		left  any
		right any
		want  int
	}{
		{"both nil", nil, nil, 0},
		{"left nil", nil, "x", -1},
		{"right nil", "x", nil, 1},
		{"int values", 5, 10, -1},
		{"int64 values", int64(20), int64(10), 1},
		{"float64 values", 3.14, 2.71, 1},
		{"string values", "apple", "banana", -1},
		{"mixed int and int64", 5, int64(5), 0},
		{"mixed int and float64 equal after truncation", 3, 3.14, 0},
		{"mixed int and float64 different", 2, 3.14, -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compareValues(tt.left, tt.right)
			if got != tt.want {
				t.Errorf("compareValues(%v, %v) = %d, want %d", tt.left, tt.right, got, tt.want)
			}
		})
	}
}

func TestResolveViewOutputPath_DefaultView(t *testing.T) {
	t.Parallel()

	col := &ingitdb.CollectionDef{
		ID:      "articles",
		DirPath: "/db/articles",
	}
	view := &ingitdb.ViewDef{
		ID:        ingitdb.DefaultViewID,
		IsDefault: true,
		Format:    "csv",
		FileName:  "export",
	}

	outPath := resolveViewOutputPath(col, view, "/db", "/db")
	// Expected: /db/$ingitdb/articles/export.csv
	if !strings.Contains(outPath, "$ingitdb") {
		t.Errorf("expected $ingitdb in path, got %q", outPath)
	}
	if !strings.HasSuffix(outPath, "export.csv") {
		t.Errorf("expected path to end with export.csv, got %q", outPath)
	}
}

func TestResolveViewOutputPath_DefaultViewNoFileName(t *testing.T) {
	t.Parallel()

	col := &ingitdb.CollectionDef{
		ID:      "articles",
		DirPath: "/db/articles",
	}
	view := &ingitdb.ViewDef{
		ID:        ingitdb.DefaultViewID,
		IsDefault: true,
		Format:    "json",
	}

	outPath := resolveViewOutputPath(col, view, "/db", "/db")
	// Expected: /db/$ingitdb/articles/articles.json (uses col.ID)
	if !strings.Contains(outPath, "$ingitdb") {
		t.Errorf("expected $ingitdb in path, got %q", outPath)
	}
	if !strings.HasSuffix(outPath, "articles.json") {
		t.Errorf("expected path to end with articles.json, got %q", outPath)
	}
}

func TestResolveViewOutputPath_RegularView(t *testing.T) {
	t.Parallel()

	col := &ingitdb.CollectionDef{
		ID:      "articles",
		DirPath: "/db/articles",
	}
	view := &ingitdb.ViewDef{
		ID:       "README",
		IsDefault: false,
		FileName: "README.md",
	}

	outPath := resolveViewOutputPath(col, view, "/db", "/db")
	expected := filepath.Join(col.DirPath, "README.md")
	if outPath != expected {
		t.Errorf("expected %q, got %q", expected, outPath)
	}
}

func TestSimpleViewBuilder_BuildDefaultView_SingleBatch(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	col := &ingitdb.CollectionDef{
		ID:           "products",
		DirPath:      filepath.Join(tmpDir, "products"),
		ColumnsOrder: []string{"id", "name", "price"},
	}
	view := &ingitdb.ViewDef{
		ID:        ingitdb.DefaultViewID,
		IsDefault: true,
		Format:    "tsv",
	}

	records := []ingitdb.RecordEntry{
		{Key: "1", Data: map[string]any{"id": "1", "name": "Widget", "price": 9.99}},
		{Key: "2", Data: map[string]any{"id": "2", "name": "Gadget", "price": 19.99}},
	}

	writer := &capturingWriter{}
	builder := SimpleViewBuilder{
		DefReader: fakeViewDefReader{views: map[string]*ingitdb.ViewDef{ingitdb.DefaultViewID: view}},
		RecordsReader: fakeRecordsReader{records: records},
		Writer:        writer,
	}

	result, err := builder.BuildViews(context.Background(), tmpDir, "", col, &ingitdb.Definition{})
	if err != nil {
		t.Fatalf("BuildViews: %v", err)
	}

	if result.FilesWritten != 1 {
		t.Errorf("expected 1 file written, got %d", result.FilesWritten)
	}
	if result.FilesUnchanged != 0 {
		t.Errorf("expected 0 files unchanged, got %d", result.FilesUnchanged)
	}
	if len(result.Errors) > 0 {
		t.Errorf("expected no errors, got %v", result.Errors)
	}
}

func TestSimpleViewBuilder_BuildDefaultView_MultiBatch(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	col := &ingitdb.CollectionDef{
		ID:           "items",
		DirPath:      filepath.Join(tmpDir, "items"),
		ColumnsOrder: []string{"id", "value"},
	}
	view := &ingitdb.ViewDef{
		ID:           ingitdb.DefaultViewID,
		IsDefault:    true,
		Format:       "json",
		MaxBatchSize: 2,
	}

	records := make([]ingitdb.RecordEntry, 5)
	for i := 1; i <= 5; i++ {
		records[i-1] = ingitdb.RecordEntry{
			Key:  string(rune(i + 48)),
			Data: map[string]any{"id": string(rune(i + 48)), "value": i * 10},
		}
	}

	writer := &capturingWriter{}
	builder := SimpleViewBuilder{
		DefReader: fakeViewDefReader{views: map[string]*ingitdb.ViewDef{ingitdb.DefaultViewID: view}},
		RecordsReader: fakeRecordsReader{records: records},
		Writer:        writer,
	}

	result, err := builder.BuildViews(context.Background(), tmpDir, "", col, &ingitdb.Definition{})
	if err != nil {
		t.Fatalf("BuildViews: %v", err)
	}

	// With 5 records and batch size 2, we expect 3 batches (2, 2, 1)
	if result.FilesWritten != 3 {
		t.Errorf("expected 3 files written, got %d", result.FilesWritten)
	}
}

func TestSimpleViewBuilder_BuildDefaultView_Idempotent(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	col := &ingitdb.CollectionDef{
		ID:           "data",
		DirPath:      filepath.Join(tmpDir, "data"),
		ColumnsOrder: []string{"id", "name"},
	}
	view := &ingitdb.ViewDef{
		ID:        ingitdb.DefaultViewID,
		IsDefault: true,
		Format:    "csv",
	}

	records := []ingitdb.RecordEntry{
		{Key: "1", Data: map[string]any{"id": "1", "name": "Alice"}},
	}

	writer := &capturingWriter{}
	builder := SimpleViewBuilder{
		DefReader: fakeViewDefReader{views: map[string]*ingitdb.ViewDef{ingitdb.DefaultViewID: view}},
		RecordsReader: fakeRecordsReader{records: records},
		Writer:        writer,
	}

	// First build
	result1, err := builder.BuildViews(context.Background(), tmpDir, "", col, &ingitdb.Definition{})
	if err != nil {
		t.Fatalf("first BuildViews: %v", err)
	}
	if result1.FilesWritten != 1 {
		t.Fatalf("expected 1 file written in first run, got %d", result1.FilesWritten)
	}

	// Second build (idempotent)
	result2, err := builder.BuildViews(context.Background(), tmpDir, "", col, &ingitdb.Definition{})
	if err != nil {
		t.Fatalf("second BuildViews: %v", err)
	}
	if result2.FilesWritten != 0 {
		t.Errorf("expected 0 files written in second run, got %d", result2.FilesWritten)
	}
	if result2.FilesUnchanged != 1 {
		t.Errorf("expected 1 file unchanged in second run, got %d", result2.FilesUnchanged)
	}
}

func TestResolveViewOutputPath_WithSubcollection(t *testing.T) {
	t.Parallel()

	col := &ingitdb.CollectionDef{
		ID:      "tags",
		DirPath: "/db/todo/.collection/subcollections/tags",
	}
	view := &ingitdb.ViewDef{
		ID:        ingitdb.DefaultViewID,
		IsDefault: true,
		Format:    "tsv",
	}

	outPath := resolveViewOutputPath(col, view, "/db", "/db")
	// Expected to contain the subcollection path structure
	if !strings.Contains(outPath, "$ingitdb") {
		t.Errorf("expected $ingitdb in path, got %q", outPath)
	}
	if !strings.HasSuffix(outPath, "tags.tsv") {
		t.Errorf("expected path to end with tags.tsv, got %q", outPath)
	}
}

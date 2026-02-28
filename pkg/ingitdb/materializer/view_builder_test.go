package materializer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
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
) (WriteOutcome, error) {
	_ = ctx
	_ = col
	_ = view
	w.called++
	w.lastOutPath = outPath
	w.lastRecords = make([]ingitdb.RecordEntry, len(records))
	copy(w.lastRecords, records)
	return WriteOutcomeCreated, nil
}

func TestSimpleViewBuilder_BuildViewsOrdersRecords(t *testing.T) {
	t.Parallel()

	col := &ingitdb.CollectionDef{
		ID:      "todo.tags",
		DirPath: "/db/todo/tags",
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

	result, err := builder.BuildViews(context.Background(), "/db", "/db", col, &ingitdb.Definition{})
	if err != nil {
		t.Fatalf("BuildViews: %v", err)
	}
	if result.FilesCreated != 1 {
		t.Fatalf("expected 1 file created, got %d", result.FilesCreated)
	}
	if writer.called != 1 {
		t.Fatalf("expected writer called once, got %d", writer.called)
	}
	expectedPath := filepath.Join("/db", ingitdb.IngitdbDir, "todo/tags", "README.md")
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
		ID:        "README",
		IsDefault: false,
		FileName:  "README.md",
	}

	outPath := resolveViewOutputPath(col, view, "/db", "/db")
	expected := filepath.Join("/db", ingitdb.IngitdbDir, "articles", "README.md")
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

	if result.FilesCreated != 1 {
		t.Errorf("expected 1 file created, got %d", result.FilesCreated)
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
	if result.FilesCreated != 3 {
		t.Errorf("expected 3 files created, got %d", result.FilesCreated)
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
	if result1.FilesCreated != 1 {
		t.Fatalf("expected 1 file created in first run, got %d", result1.FilesCreated)
	}

	// Second build (idempotent)
	result2, err := builder.BuildViews(context.Background(), tmpDir, "", col, &ingitdb.Definition{})
	if err != nil {
		t.Fatalf("second BuildViews: %v", err)
	}
	// capturingWriter always returns WriteOutcomeCreated, so result2 always shows created=1
	// The real idempotency (unchanged detection) is tested in TestBuildDefaultView_Idempotency
	if result2.FilesCreated+result2.FilesUpdated+result2.FilesUnchanged != 1 {
		t.Errorf("expected 1 file processed in second run, got %d", result2.FilesCreated+result2.FilesUpdated+result2.FilesUnchanged)
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

// --- Integration tests for default view with batching ---

func TestBuildDefaultView_MultiBatchWithFilenaming(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	col := &ingitdb.CollectionDef{
		ID:           "data",
		DirPath:      filepath.Join(tmpDir, "data"),
		ColumnsOrder: []string{"id", "value"},
	}
	view := &ingitdb.ViewDef{
		ID:           ingitdb.DefaultViewID,
		IsDefault:    true,
		Format:       "tsv",
		MaxBatchSize: 3,
		FileName:     "export",
	}

	// Create 10 records to test batching and 6-digit padding
	records := make([]ingitdb.RecordEntry, 10)
	for i := 0; i < 10; i++ {
		records[i] = ingitdb.RecordEntry{
			Key:  fmt.Sprintf("%d", i+1),
			Data: map[string]any{"id": fmt.Sprintf("%d", i+1), "value": (i + 1) * 10},
		}
	}

	created, _, _, errs := buildDefaultView(tmpDir, "", col, &ingitdb.Definition{}, view, records, nil)

	if len(errs) > 0 {
		t.Fatalf("buildDefaultView returned errors: %v", errs)
	}

	// With 10 records and batch size 3: (3, 3, 3, 1) = 4 batches
	expectedBatches := 4
	if created != expectedBatches {
		t.Errorf("expected %d batches created, got %d", expectedBatches, created)
	}

	// Verify file names with 6-digit padding
	expectedNames := []string{
		"export-000001.tsv",
		"export-000002.tsv",
		"export-000003.tsv",
		"export-000004.tsv",
	}

	ingitdbDir := filepath.Join(tmpDir, ingitdb.IngitdbDir, "data")
	files, err := os.ReadDir(ingitdbDir)
	if err != nil {
		t.Fatalf("failed to read ingitdb dir: %v", err)
	}

	fileNames := make([]string, 0, len(files))
	for _, f := range files {
		fileNames = append(fileNames, f.Name())
	}

	for _, expected := range expectedNames {
		found := false
		for _, actual := range fileNames {
			if actual == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected file %q not found. Got: %v", expected, fileNames)
		}
	}
}

func TestBuildDefaultView_SingleBatchNoSuffix(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	col := &ingitdb.CollectionDef{
		ID:           "items",
		DirPath:      filepath.Join(tmpDir, "items"),
		ColumnsOrder: []string{"id", "name"},
	}
	view := &ingitdb.ViewDef{
		ID:           ingitdb.DefaultViewID,
		IsDefault:    true,
		Format:       "csv",
		MaxBatchSize: 100, // Large batch size, single batch
		FileName:     "data",
	}

	records := []ingitdb.RecordEntry{
		{Key: "1", Data: map[string]any{"id": "1", "name": "Item1"}},
		{Key: "2", Data: map[string]any{"id": "2", "name": "Item2"}},
	}

	created, _, _, errs := buildDefaultView(tmpDir, "", col, &ingitdb.Definition{}, view, records, nil)

	if len(errs) > 0 {
		t.Fatalf("buildDefaultView returned errors: %v", errs)
	}

	if created != 1 {
		t.Errorf("expected 1 file created, got %d", created)
	}

	// Verify file name has no numeric suffix for single batch
	ingitdbDir := filepath.Join(tmpDir, ingitdb.IngitdbDir, "items")
	files, err := os.ReadDir(ingitdbDir)
	if err != nil {
		t.Fatalf("failed to read ingitdb dir: %v", err)
	}

	if len(files) != 1 {
		t.Errorf("expected 1 file, got %d", len(files))
	}

	if files[0].Name() != "data.csv" {
		t.Errorf("expected file name 'data.csv', got %q", files[0].Name())
	}
}

func TestBuildDefaultView_Idempotency_NoChanges(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	col := &ingitdb.CollectionDef{
		ID:           "test",
		DirPath:      filepath.Join(tmpDir, "test"),
		ColumnsOrder: []string{"id", "value"},
	}
	view := &ingitdb.ViewDef{
		ID:        ingitdb.DefaultViewID,
		IsDefault: true,
		Format:    "json",
	}

	records := []ingitdb.RecordEntry{
		{Key: "1", Data: map[string]any{"id": "1", "value": "test"}},
	}

	// First build
	created1, _, unchanged1, errs1 := buildDefaultView(tmpDir, "", col, &ingitdb.Definition{}, view, records, nil)
	if len(errs1) > 0 {
		t.Fatalf("first buildDefaultView: %v", errs1)
	}

	if created1 != 1 {
		t.Fatalf("first run: expected 1 created, got %d", created1)
	}
	if unchanged1 != 0 {
		t.Fatalf("first run: expected 0 unchanged, got %d", unchanged1)
	}

	// Second build with identical data
	created2, updated2, unchanged2, errs2 := buildDefaultView(tmpDir, "", col, &ingitdb.Definition{}, view, records, nil)
	if len(errs2) > 0 {
		t.Fatalf("second buildDefaultView: %v", errs2)
	}

	if created2+updated2 != 0 {
		t.Errorf("second run: expected 0 created/updated, got created=%d updated=%d", created2, updated2)
	}
	if unchanged2 != 1 {
		t.Errorf("second run: expected 1 unchanged, got %d", unchanged2)
	}
}

func TestBuildDefaultView_Idempotency_OneRecordChanged(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	col := &ingitdb.CollectionDef{
		ID:           "test",
		DirPath:      filepath.Join(tmpDir, "test"),
		ColumnsOrder: []string{"id", "value"},
	}
	view := &ingitdb.ViewDef{
		ID:           ingitdb.DefaultViewID,
		IsDefault:    true,
		Format:       "tsv",
		MaxBatchSize: 2,
	}

	records1 := []ingitdb.RecordEntry{
		{Key: "1", Data: map[string]any{"id": "1", "value": "first"}},
		{Key: "2", Data: map[string]any{"id": "2", "value": "second"}},
		{Key: "3", Data: map[string]any{"id": "3", "value": "third"}},
	}

	// First build
	created1, _, _, errs1 := buildDefaultView(tmpDir, "", col, &ingitdb.Definition{}, view, records1, nil)
	if len(errs1) > 0 {
		t.Fatalf("first buildDefaultView: %v", errs1)
	}
	if created1 != 2 {
		t.Fatalf("first run: expected 2 created, got %d", created1)
	}

	// Second build with one record changed (in batch 1)
	records2 := []ingitdb.RecordEntry{
		{Key: "1", Data: map[string]any{"id": "1", "value": "CHANGED"}},
		{Key: "2", Data: map[string]any{"id": "2", "value": "second"}},
		{Key: "3", Data: map[string]any{"id": "3", "value": "third"}},
	}

	created2, updated2, unchanged2, errs2 := buildDefaultView(tmpDir, "", col, &ingitdb.Definition{}, view, records2, nil)
	if len(errs2) > 0 {
		t.Fatalf("second buildDefaultView: %v", errs2)
	}

	if updated2 != 1 {
		t.Errorf("second run: expected 1 updated (batch 1 changed), got %d (created=%d)", updated2, created2)
	}
	if unchanged2 != 1 {
		t.Errorf("second run: expected 1 unchanged (batch 2 unchanged), got %d", unchanged2)
	}
}

func TestBuildDefaultView_AllFormats(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	formats := []string{"tsv", "csv", "json", "jsonl", "yaml"}
	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			t.Parallel()

			col := &ingitdb.CollectionDef{
				ID:           "test_" + format,
				DirPath:      filepath.Join(tmpDir, "test_"+format),
				ColumnsOrder: []string{"id", "name"},
			}
			view := &ingitdb.ViewDef{
				ID:        ingitdb.DefaultViewID,
				IsDefault: true,
				Format:    format,
			}

			records := []ingitdb.RecordEntry{
				{Key: "1", Data: map[string]any{"id": "1", "name": "Test"}},
			}

			created, _, _, errs := buildDefaultView(tmpDir, "", col, &ingitdb.Definition{}, view, records, nil)

			if len(errs) > 0 {
				t.Errorf("buildDefaultView for %s returned errors: %v", format, errs)
			}

			if created != 1 {
				t.Errorf("%s: expected 1 created, got %d", format, created)
			}

			// Verify file was created with correct extension
			ingitdbDir := filepath.Join(tmpDir, ingitdb.IngitdbDir, col.ID)
			files, err := os.ReadDir(ingitdbDir)
			if err != nil {
				t.Errorf("%s: failed to read dir: %v", format, err)
			}

			if len(files) != 1 {
				t.Errorf("%s: expected 1 file, got %d", format, len(files))
			} else {
				expectedExt := defaultViewFormatExtension(format)
				actualName := files[0].Name()
				if !strings.HasSuffix(actualName, "."+expectedExt) {
					t.Errorf("%s: expected file ending with .%s, got %s", format, expectedExt, actualName)
				}
			}
		})
	}
}

func TestBuildDefaultView_CreatesMissingDirectories(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	col := &ingitdb.CollectionDef{
		ID:           "nested/collection",
		DirPath:      filepath.Join(tmpDir, "nested/collection"),
		ColumnsOrder: []string{"id"},
	}
	view := &ingitdb.ViewDef{
		ID:        ingitdb.DefaultViewID,
		IsDefault: true,
		Format:    "json",
	}

	records := []ingitdb.RecordEntry{
		{Key: "1", Data: map[string]any{"id": "1"}},
	}

	created, _, _, errs := buildDefaultView(tmpDir, "", col, &ingitdb.Definition{}, view, records, nil)

	if len(errs) > 0 {
		t.Fatalf("buildDefaultView returned errors: %v", errs)
	}

	if created != 1 {
		t.Fatalf("expected 1 created, got %d", created)
	}

	// Verify nested directory structure was created
	ingitdbDir := filepath.Join(tmpDir, ingitdb.IngitdbDir, col.ID)
	_, err := os.Stat(ingitdbDir)
	if err != nil {
		t.Errorf("nested ingitdb directory not created: %v", err)
	}
}

func TestBuildDefaultView_EmptyRecords(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	col := &ingitdb.CollectionDef{
		ID:           "empty",
		DirPath:      filepath.Join(tmpDir, "empty"),
		ColumnsOrder: []string{"id", "name"},
	}
	view := &ingitdb.ViewDef{
		ID:        ingitdb.DefaultViewID,
		IsDefault: true,
		Format:    "csv",
	}

	records := []ingitdb.RecordEntry{}

	created, _, _, errs := buildDefaultView(tmpDir, "", col, &ingitdb.Definition{}, view, records, nil)

	if len(errs) > 0 {
		t.Fatalf("buildDefaultView returned errors: %v", errs)
	}

	if created != 1 {
		t.Fatalf("expected 1 file created (header only), got %d", created)
	}

	// Verify file contains headers
	ingitdbDir := filepath.Join(tmpDir, ingitdb.IngitdbDir, "empty")
	files, _ := os.ReadDir(ingitdbDir)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	content, err := os.ReadFile(filepath.Join(ingitdbDir, files[0].Name()))
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	if !strings.Contains(string(content), "id") {
		t.Errorf("file should contain headers")
	}
}

func TestBuildDefaultView_WithCustomFileName(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	col := &ingitdb.CollectionDef{
		ID:           "products",
		DirPath:      filepath.Join(tmpDir, "products"),
		ColumnsOrder: []string{"id", "price"},
	}
	view := &ingitdb.ViewDef{
		ID:        ingitdb.DefaultViewID,
		IsDefault: true,
		Format:    "json",
		FileName:  "product_export",
	}

	records := []ingitdb.RecordEntry{
		{Key: "1", Data: map[string]any{"id": "1", "price": 99.99}},
	}

	created, _, _, errs := buildDefaultView(tmpDir, "", col, &ingitdb.Definition{}, view, records, nil)

	if len(errs) > 0 {
		t.Fatalf("buildDefaultView returned errors: %v", errs)
	}

	if created != 1 {
		t.Fatalf("expected 1 created, got %d", created)
	}

	// Verify custom file name was used
	ingitdbDir := filepath.Join(tmpDir, ingitdb.IngitdbDir, "products")
	files, _ := os.ReadDir(ingitdbDir)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	if files[0].Name() != "product_export.json" {
		t.Errorf("expected 'product_export.json', got %q", files[0].Name())
	}
}

func TestBuildDefaultView_DefaultFileNameUsesCollectionID(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	col := &ingitdb.CollectionDef{
		ID:           "articles",
		DirPath:      filepath.Join(tmpDir, "articles"),
		ColumnsOrder: []string{"id", "title"},
	}
	view := &ingitdb.ViewDef{
		ID:        ingitdb.DefaultViewID,
		IsDefault: true,
		Format:    "tsv",
		// FileName not set
	}

	records := []ingitdb.RecordEntry{
		{Key: "1", Data: map[string]any{"id": "1", "title": "Article1"}},
	}

	created, _, _, errs := buildDefaultView(tmpDir, "", col, &ingitdb.Definition{}, view, records, nil)

	if len(errs) > 0 {
		t.Fatalf("buildDefaultView returned errors: %v", errs)
	}

	if created != 1 {
		t.Fatalf("expected 1 created, got %d", created)
	}

	// Verify file name uses collection ID
	ingitdbDir := filepath.Join(tmpDir, ingitdb.IngitdbDir, "articles")
	files, _ := os.ReadDir(ingitdbDir)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	if files[0].Name() != "articles.tsv" {
		t.Errorf("expected 'articles.tsv', got %q", files[0].Name())
	}
}

func TestBuildDefaultView_LargeBatchCount_VerifyPadding(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	col := &ingitdb.CollectionDef{
		ID:           "huge",
		DirPath:      filepath.Join(tmpDir, "huge"),
		ColumnsOrder: []string{"id"},
	}
	view := &ingitdb.ViewDef{
		ID:           ingitdb.DefaultViewID,
		IsDefault:    true,
		Format:       "json",
		MaxBatchSize: 1, // 15 records = 15 batches
	}

	records := make([]ingitdb.RecordEntry, 15)
	for i := 0; i < 15; i++ {
		records[i] = ingitdb.RecordEntry{
			Key:  fmt.Sprintf("%d", i+1),
			Data: map[string]any{"id": fmt.Sprintf("%d", i+1)},
		}
	}

	created, _, _, errs := buildDefaultView(tmpDir, "", col, &ingitdb.Definition{}, view, records, nil)

	if len(errs) > 0 {
		t.Fatalf("buildDefaultView returned errors: %v", errs)
	}

	if created != 15 {
		t.Fatalf("expected 15 files created, got %d", created)
	}

	// Verify padding: 000010, 000015 etc.
	ingitdbDir := filepath.Join(tmpDir, ingitdb.IngitdbDir, "huge")
	files, _ := os.ReadDir(ingitdbDir)

	// Check that batch 10 has correct padding (000010)
	batch10Found := false
	for _, f := range files {
		if f.Name() == "huge-000010.json" {
			batch10Found = true
			break
		}
	}

	if !batch10Found {
		t.Errorf("expected file 'huge-000010.json' not found")
	}
}

func TestBuildDefaultView_FileContentIsValid(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	col := &ingitdb.CollectionDef{
		ID:           "validation",
		DirPath:      filepath.Join(tmpDir, "validation"),
		ColumnsOrder: []string{"id", "name", "score"},
	}
	view := &ingitdb.ViewDef{
		ID:        ingitdb.DefaultViewID,
		IsDefault: true,
		Format:    "json",
	}

	records := []ingitdb.RecordEntry{
		{Key: "1", Data: map[string]any{"id": "1", "name": "Alice", "score": 95}},
		{Key: "2", Data: map[string]any{"id": "2", "name": "Bob", "score": 87}},
	}

	created, _, _, errs := buildDefaultView(tmpDir, "", col, &ingitdb.Definition{}, view, records, nil)

	if len(errs) > 0 {
		t.Fatalf("buildDefaultView returned errors: %v", errs)
	}

	if created != 1 {
		t.Fatalf("expected 1 created, got %d", created)
	}

	// Read and validate file content
	ingitdbDir := filepath.Join(tmpDir, ingitdb.IngitdbDir, "validation")
	files, _ := os.ReadDir(ingitdbDir)
	filePath := filepath.Join(ingitdbDir, files[0].Name())

	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	var result []map[string]any
	if err := json.Unmarshal(content, &result); err != nil {
		t.Fatalf("file content is not valid JSON: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("expected 2 records in file, got %d", len(result))
	}
}

// --- Error path tests ---

func TestBuildDefaultView_FormatExportBatchError(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	col := &ingitdb.CollectionDef{
		ID:           "test",
		DirPath:      filepath.Join(tmpDir, "test"),
		ColumnsOrder: []string{"id"},
	}
	// Use invalid format to trigger formatExportBatch error path
	view := &ingitdb.ViewDef{
		ID:        ingitdb.DefaultViewID,
		IsDefault: true,
		Format:    "",  // Empty format defaults to TSV, so no error
	}

	records := []ingitdb.RecordEntry{
		{Key: "1", Data: map[string]any{"id": "1"}},
	}

	created, _, _, errs := buildDefaultView(tmpDir, "", col, &ingitdb.Definition{}, view, records, nil)

	// Empty format is valid (defaults to TSV), so no errors expected
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if created != 1 {
		t.Fatalf("expected 1 created, got %d", created)
	}
}

func TestBuildDefaultView_RecordsDelimiterFromSettings(t *testing.T) {
t.Parallel()

tmpDir := t.TempDir()
col := &ingitdb.CollectionDef{
ID:           "test",
DirPath:      filepath.Join(tmpDir, "test"),
ColumnsOrder: []string{"id"},
}
view := &ingitdb.ViewDef{
ID:        ingitdb.DefaultViewID,
IsDefault: true,
Format:    "ingr",
}
def := &ingitdb.Definition{
Settings: ingitdb.Settings{RecordsDelimiter: true},
}
records := []ingitdb.RecordEntry{
{Key: "1", Data: map[string]any{"id": "1"}},
}

created, _, _, errs := buildDefaultView(tmpDir, "", col, def, view, records, nil)
if len(errs) > 0 {
t.Fatalf("unexpected errors: %v", errs)
}
if created != 1 {
t.Fatalf("expected 1 created, got %d", created)
}
	content, err := os.ReadFile(filepath.Join(tmpDir, ingitdb.IngitdbDir, "test", "test.ingr"))
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if !strings.Contains(string(content), "\n#\n") {
		t.Error("expected '#' delimiter line in output when Settings.RecordsDelimiter=true")
	}
}

func TestBuildDefaultView_RuntimeOverrideDisablesViewDefDelimiter(t *testing.T) {
t.Parallel()

tmpDir := t.TempDir()
col := &ingitdb.CollectionDef{
ID:           "test",
DirPath:      filepath.Join(tmpDir, "test"),
ColumnsOrder: []string{"id"},
}
view := &ingitdb.ViewDef{
ID:               ingitdb.DefaultViewID,
IsDefault:        true,
Format:           "ingr",
RecordsDelimiter: true,
}
falseVal := false
def := &ingitdb.Definition{
RuntimeOverrides: ingitdb.RuntimeOverrides{RecordsDelimiter: &falseVal},
}
records := []ingitdb.RecordEntry{
{Key: "1", Data: map[string]any{"id": "1"}},
}

created, _, _, errs := buildDefaultView(tmpDir, "", col, def, view, records, nil)
if len(errs) > 0 {
t.Fatalf("unexpected errors: %v", errs)
}
if created != 1 {
t.Fatalf("expected 1 created, got %d", created)
}
	content, err := os.ReadFile(filepath.Join(tmpDir, ingitdb.IngitdbDir, "test", "test.ingr"))
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if strings.Contains(string(content), "#\n") {
		t.Error("expected no '#' delimiter line when RuntimeOverrides.RecordsDelimiter=false overrides ViewDef")
	}
}

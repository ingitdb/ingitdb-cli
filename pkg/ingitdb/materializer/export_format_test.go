package materializer

import (
	"encoding/csv"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"gopkg.in/yaml.v3"
)

func TestDefaultViewFormatExtension(t *testing.T) {
	t.Parallel()

	tests := []struct {
		format string
		want   string
	}{
		{"", "tsv"},
		{"tsv", "tsv"},
		{"TSV", "tsv"},
		{"csv", "csv"},
		{"CSV", "csv"},
		{"json", "json"},
		{"JSON", "json"},
		{"jsonl", "jsonl"},
		{"JSONL", "jsonl"},
		{"yaml", "yaml"},
		{"YAML", "yaml"},
		{"unknown", "tsv"},
		{"txt", "tsv"},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			t.Parallel()
			got := defaultViewFormatExtension(tt.format)
			if got != tt.want {
				t.Errorf("defaultViewFormatExtension(%q) = %q, want %q", tt.format, got, tt.want)
			}
		})
	}
}

func TestFormatBatchFileName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		base         string
		ext          string
		batchNum     int
		totalBatches int
		want         string
	}{
		{"data", "tsv", 1, 1, "data.tsv"},
		{"data", "tsv", 1, 0, "data.tsv"},
		{"data", "csv", 1, 2, "data-000001.csv"},
		{"data", "json", 5, 10, "data-000005.json"},
		{"items", "jsonl", 10, 100, "items-000010.jsonl"},
		{"records", "yaml", 999999, 1000000, "records-999999.yaml"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			got := formatBatchFileName(tt.base, tt.ext, tt.batchNum, tt.totalBatches)
			if got != tt.want {
				t.Errorf("formatBatchFileName(%q, %q, %d, %d) = %q, want %q",
					tt.base, tt.ext, tt.batchNum, tt.totalBatches, got, tt.want)
			}
		})
	}
}

func TestFormatExportBatch_TSV(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		headers []string
		records []ingitdb.RecordEntry
		want    string
	}{
		{
			"single record",
			[]string{"id", "name"},
			[]ingitdb.RecordEntry{
				{Key: "1", Data: map[string]any{"id": "1", "name": "Alice"}},
			},
			"id\tname\n1\tAlice\n",
		},
		{
			"multiple records",
			[]string{"id", "name", "age"},
			[]ingitdb.RecordEntry{
				{Key: "1", Data: map[string]any{"id": "1", "name": "Alice", "age": 30}},
				{Key: "2", Data: map[string]any{"id": "2", "name": "Bob", "age": 25}},
			},
			"id\tname\tage\n1\tAlice\t30\n2\tBob\t25\n",
		},
		{
			"nil value",
			[]string{"id", "name"},
			[]ingitdb.RecordEntry{
				{Key: "1", Data: map[string]any{"id": "1", "name": nil}},
			},
			"id\tname\n1\t\n",
		},
		{
			"missing field",
			[]string{"id", "name"},
			[]ingitdb.RecordEntry{
				{Key: "1", Data: map[string]any{"id": "1"}},
			},
			"id\tname\n1\t\n",
		},
		{
			"escape tab",
			[]string{"id", "text"},
			[]ingitdb.RecordEntry{
				{Key: "1", Data: map[string]any{"id": "1", "text": "hello\tworld"}},
			},
			"id\ttext\n1\thello\\tworld\n",
		},
		{
			"escape newline",
			[]string{"id", "text"},
			[]ingitdb.RecordEntry{
				{Key: "1", Data: map[string]any{"id": "1", "text": "line1\nline2"}},
			},
			"id\ttext\n1\tline1\\nline2\n",
		},
		{
			"escape backslash",
			[]string{"id", "text"},
			[]ingitdb.RecordEntry{
				{Key: "1", Data: map[string]any{"id": "1", "text": "path\\to\\file"}},
			},
			"id\ttext\n1\tpath\\\\to\\\\file\n",
		},
		{
			"escape carriage return",
			[]string{"id", "text"},
			[]ingitdb.RecordEntry{
				{Key: "1", Data: map[string]any{"id": "1", "text": "hello\rworld"}},
			},
			"id\ttext\n1\thello\\rworld\n",
		},
		{
			"empty records",
			[]string{"id", "name"},
			[]ingitdb.RecordEntry{},
			"id\tname\n",
		},
		{
			"nil data",
			[]string{"id", "name"},
			[]ingitdb.RecordEntry{
				{Key: "1", Data: nil},
			},
			"id\tname\n\t\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := formatExportBatch("tsv", tt.headers, tt.records)
			if err != nil {
				t.Fatalf("formatExportBatch: %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("formatExportBatch(...) = %q, want %q", string(got), tt.want)
			}
		})
	}
}

func TestFormatExportBatch_CSV(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		headers []string
		records []ingitdb.RecordEntry
		check   func(string) bool
	}{
		{
			"single record",
			[]string{"id", "name"},
			[]ingitdb.RecordEntry{
				{Key: "1", Data: map[string]any{"id": "1", "name": "Alice"}},
			},
			func(s string) bool {
				r := csv.NewReader(strings.NewReader(s))
				records, err := r.ReadAll()
				return err == nil && len(records) == 2 && records[0][0] == "id" && records[1][0] == "1"
			},
		},
		{
			"field with comma",
			[]string{"id", "name"},
			[]ingitdb.RecordEntry{
				{Key: "1", Data: map[string]any{"id": "1", "name": "Smith, John"}},
			},
			func(s string) bool {
				r := csv.NewReader(strings.NewReader(s))
				records, err := r.ReadAll()
				return err == nil && len(records) == 2 && records[1][1] == "Smith, John"
			},
		},
		{
			"field with quote",
			[]string{"id", "name"},
			[]ingitdb.RecordEntry{
				{Key: "1", Data: map[string]any{"id": "1", "name": `He said "hello"`}},
			},
			func(s string) bool {
				r := csv.NewReader(strings.NewReader(s))
				records, err := r.ReadAll()
				return err == nil && len(records) == 2 && records[1][1] == `He said "hello"`
			},
		},
		{
			"field with newline",
			[]string{"id", "description"},
			[]ingitdb.RecordEntry{
				{Key: "1", Data: map[string]any{"id": "1", "description": "Line 1\nLine 2"}},
			},
			func(s string) bool {
				r := csv.NewReader(strings.NewReader(s))
				records, err := r.ReadAll()
				return err == nil && len(records) == 2 && records[1][1] == "Line 1\nLine 2"
			},
		},
		{
			"empty records",
			[]string{"id", "name"},
			[]ingitdb.RecordEntry{},
			func(s string) bool {
				r := csv.NewReader(strings.NewReader(s))
				records, err := r.ReadAll()
				return err == nil && len(records) == 1 && records[0][0] == "id"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := formatExportBatch("csv", tt.headers, tt.records)
			if err != nil {
				t.Fatalf("formatExportBatch: %v", err)
			}
			if !tt.check(string(got)) {
				t.Errorf("CSV validation failed for: %s", string(got))
			}
		})
	}
}

func TestFormatExportBatch_JSON(t *testing.T) {
	t.Parallel()

	headers := []string{"id", "name", "age"}
	records := []ingitdb.RecordEntry{
		{Key: "1", Data: map[string]any{"id": "1", "name": "Alice", "age": 30}},
		{Key: "2", Data: map[string]any{"id": "2", "name": "Bob", "age": 25}},
	}

	got, err := formatExportBatch("json", headers, records)
	if err != nil {
		t.Fatalf("formatExportBatch: %v", err)
	}

	var result []map[string]any
	if err := json.Unmarshal(got, &result); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("expected 2 records, got %d", len(result))
	}
	if result[0]["id"] != "1" || result[0]["name"] != "Alice" {
		t.Errorf("first record mismatch: %v", result[0])
	}
	if result[1]["id"] != "2" || result[1]["name"] != "Bob" {
		t.Errorf("second record mismatch: %v", result[1])
	}
}

func TestFormatExportBatch_JSONL(t *testing.T) {
	t.Parallel()

	headers := []string{"id", "name"}
	records := []ingitdb.RecordEntry{
		{Key: "1", Data: map[string]any{"id": "1", "name": "Alice"}},
		{Key: "2", Data: map[string]any{"id": "2", "name": "Bob"}},
	}

	got, err := formatExportBatch("jsonl", headers, records)
	if err != nil {
		t.Fatalf("formatExportBatch: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(got)), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d", len(lines))
	}

	var obj1, obj2 map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &obj1); err != nil {
		t.Fatalf("failed to unmarshal first line: %v", err)
	}
	if err := json.Unmarshal([]byte(lines[1]), &obj2); err != nil {
		t.Fatalf("failed to unmarshal second line: %v", err)
	}

	if obj1["id"] != "1" || obj2["id"] != "2" {
		t.Errorf("JSONL records mismatch")
	}
}

func TestFormatExportBatch_YAML(t *testing.T) {
	t.Parallel()

	headers := []string{"id", "name"}
	records := []ingitdb.RecordEntry{
		{Key: "1", Data: map[string]any{"id": "1", "name": "Alice"}},
	}

	got, err := formatExportBatch("yaml", headers, records)
	if err != nil {
		t.Fatalf("formatExportBatch: %v", err)
	}

	var result []map[string]any
	if err := yaml.Unmarshal(got, &result); err != nil {
		t.Fatalf("failed to unmarshal YAML: %v", err)
	}

	if len(result) != 1 {
		t.Errorf("expected 1 record, got %d", len(result))
	}
	if result[0]["id"] != "1" || result[0]["name"] != "Alice" {
		t.Errorf("record mismatch: %v", result[0])
	}
}

func TestFormatExportBatch_EmptyRecords(t *testing.T) {
	t.Parallel()

	headers := []string{"id", "name"}
	records := []ingitdb.RecordEntry{}

	tests := []struct {
		format string
		check  func(string) bool
	}{
		{"tsv", func(s string) bool {
			// TSV with just headers
			return strings.Contains(s, "id\tname")
		}},
		{"csv", func(s string) bool {
			// CSV with just headers
			return strings.Contains(s, "id")
		}},
		{"json", func(s string) bool {
			// JSON empty array
			return s == "[]"
		}},
		{"jsonl", func(s string) bool {
			// JSONL empty (no lines)
			return len(strings.TrimSpace(s)) == 0
		}},
		{"yaml", func(s string) bool {
			// YAML empty array or null
			return strings.Contains(s, "[]") || strings.Contains(s, "null")
		}},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			t.Parallel()
			got, err := formatExportBatch(tt.format, headers, records)
			if err != nil {
				t.Fatalf("formatExportBatch: %v", err)
			}
			if !tt.check(string(got)) {
				t.Errorf("CSV validation failed for format %s: %s", tt.format, string(got))
			}
		})
	}
}

func TestDetermineColumns(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		col      *ingitdb.CollectionDef
		view     *ingitdb.ViewDef
		expected []string
	}{
		{
			"view columns with id at index 0",
			&ingitdb.CollectionDef{ID: "col1", ColumnsOrder: []string{"id", "x", "y"}},
			&ingitdb.ViewDef{Columns: []string{"id", "x"}},
			[]string{"id", "x"},
		},
		{
			"view columns without id",
			&ingitdb.CollectionDef{ID: "col1", ColumnsOrder: []string{"id", "x", "y"}},
			&ingitdb.ViewDef{Columns: []string{"x", "y"}},
			[]string{"id", "x", "y"},
		},
		{
			"view columns with id in middle",
			&ingitdb.CollectionDef{ID: "col1", ColumnsOrder: []string{"id", "x", "y"}},
			&ingitdb.ViewDef{Columns: []string{"x", "id", "y"}},
			[]string{"id", "x", "y"},
		},
		{
			"use collection columns order",
			&ingitdb.CollectionDef{ID: "col1", ColumnsOrder: []string{"id", "a", "b", "c"}},
			&ingitdb.ViewDef{Columns: []string{}},
			[]string{"id", "a", "b", "c"},
		},
		{
			"empty columns",
			&ingitdb.CollectionDef{ID: "col1", ColumnsOrder: []string{}},
			&ingitdb.ViewDef{Columns: []string{}},
			[]string{"id"},
		},
		{
			"single column id",
			&ingitdb.CollectionDef{ID: "col1", ColumnsOrder: []string{"id"}},
			&ingitdb.ViewDef{Columns: []string{}},
			[]string{"id"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := determineColumns(tt.col, tt.view)
			if !slicesEqual(got, tt.expected) {
				t.Errorf("determineColumns() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestEscapeTSV(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{"hello\tworld", "hello\\tworld"},
		{"line1\nline2", "line1\\nline2"},
		{"path\\to\\file", "path\\\\to\\\\file"},
		{"hello\rworld", "hello\\rworld"},
		{"a\tb\nc\rd", "a\\tb\\nc\\rd"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := escapeTSV(tt.input)
			if got != tt.want {
				t.Errorf("escapeTSV(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestRecordsToMaps(t *testing.T) {
	t.Parallel()

	headers := []string{"id", "name", "age"}
	records := []ingitdb.RecordEntry{
		{Key: "1", Data: map[string]any{"id": "1", "name": "Alice", "age": 30}},
		{Key: "2", Data: map[string]any{"id": "2", "name": "Bob"}},
		{Key: "3", Data: nil},
	}

	result := recordsToMaps(headers, records)

	if len(result) != 3 {
		t.Errorf("expected 3 records, got %d", len(result))
	}

	if result[0]["id"] != "1" || result[0]["name"] != "Alice" || result[0]["age"] != 30 {
		t.Errorf("first record mismatch: %v", result[0])
	}

	if result[1]["id"] != "2" || result[1]["name"] != "Bob" {
		t.Errorf("second record mismatch: %v", result[1])
	}

	if result[2]["id"] != nil {
		t.Errorf("third record should have nil values, got %v", result[2])
	}
}

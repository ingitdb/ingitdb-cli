package materializer

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"fmt"
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
		{"", "ingr"},
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
		{"ingr", "ingr"},
		{"INGR", "ingr"},
		{"unknown", "ingr"},
		{"txt", "ingr"},
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
			got, err := formatExportBatch("tsv", "", tt.headers, tt.records)
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
			got, err := formatExportBatch("csv", "", tt.headers, tt.records)
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

	got, err := formatExportBatch("json", "", headers, records)
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

	got, err := formatExportBatch("jsonl", "", headers, records)
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

	got, err := formatExportBatch("yaml", "", headers, records)
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
			got, err := formatExportBatch(tt.format, "test/view", headers, records)
			if err != nil {
				t.Fatalf("formatExportBatch: %v", err)
			}
			if !tt.check(string(got)) {
				t.Errorf("CSV validation failed for format %s: %s", tt.format, string(got))
			}
		})
	}
}

func TestFormatExportBatch_INGR(t *testing.T) {
	t.Parallel()

	headers := []string{"id", "name", "age"}
	records := []ingitdb.RecordEntry{
		{Key: "1", Data: map[string]any{"id": "1", "name": "Alice", "age": 30}},
		{Key: "2", Data: map[string]any{"id": "2", "name": "Bob", "age": 25}},
	}

	got, err := formatExportBatch("ingr", "test/view", headers, records)
	if err != nil {
		t.Fatalf("formatExportBatch: %v", err)
	}

	// INGR: header line + 3 fields per record, 2 records; strings are JSON-quoted; two-line footer, no trailing newline
	want := "#INGR: test/view: $ID, name, age\n" +
		`"1"` + "\n" + `"Alice"` + "\n" + `30` + "\n" +
		`"2"` + "\n" + `"Bob"` + "\n" + `25` + "\n" +
		"# 2 records\n" +
		"# sha256:efbc9a977adbf6a18ec5264b1b31590045ef2662b49d665fed24e8040f991649"
	if string(got) != want {
		t.Errorf("formatExportBatch(ingr) = %q, want %q", string(got), want)
	}
}

func TestFormatINGR_EmptyRecords(t *testing.T) {
	t.Parallel()

	got, err := formatExportBatch("ingr", "test/view", []string{"id", "name"}, []ingitdb.RecordEntry{})
	if err != nil {
		t.Fatalf("formatExportBatch: %v", err)
	}
	// empty records: header, count footer with newline, hash footer without trailing newline
	want := "#INGR: test/view: $ID, name\n# 0 records\n# sha256:a87f64c6e3487f35107f66a61c69c4501c7cd29fc51e7e3c587ce472337a6517"
	if string(got) != want {
		t.Errorf("expected only header for empty records, got %q", string(got))
	}
}

func TestFormatINGR_NilAndMissingFields(t *testing.T) {
	t.Parallel()

	headers := []string{"id", "name", "age"}
	records := []ingitdb.RecordEntry{
		{Key: "1", Data: map[string]any{"id": "1", "name": nil}}, // missing age, nil name
	}

	got, err := formatExportBatch("ingr", "test/view", headers, records)
	if err != nil {
		t.Fatalf("formatExportBatch: %v", err)
	}

	// nil name → JSON null, missing age → JSON null; two-line footer, no trailing newline
	want := "#INGR: test/view: $ID, name, age\n\"1\"\nnull\nnull\n# 1 record\n# sha256:ef7a0b65cfb9927343f31d4fad1ce170ac12737a3e9b982d4b761acc19c48442"
	if string(got) != want {
		t.Errorf("formatExportBatch(ingr) = %q, want %q", string(got), want)
	}
}

func TestFormatINGR_DefaultFormatIsINGR(t *testing.T) {
	t.Parallel()

	headers := []string{"id"}
	records := []ingitdb.RecordEntry{
		{Key: "1", Data: map[string]any{"id": "hello"}},
	}

	// empty format string should use INGR (the default)
	got, err := formatExportBatch("", "test/view", headers, records)
	if err != nil {
		t.Fatalf("formatExportBatch: %v", err)
	}
	want := "#INGR: test/view: $ID\n\"hello\"\n# 1 record\n# sha256:8a17ca12db7fbee8ddb9be2aea8c24b225ca9fbe509667a957f338bbc82680b6"
	if string(got) != want {
		t.Errorf("default format output = %q, want %q", string(got), want)
	}
}

func TestFormatINGR_HashCoversHeaderAndRecordsAndCountLine(t *testing.T) {
	t.Parallel()

	headers := []string{"id", "name"}
	records := []ingitdb.RecordEntry{
		{Key: "a", Data: map[string]any{"id": "a", "name": "Alice"}},
	}

	got, err := formatExportBatch("ingr", "test/view", headers, records)
	if err != nil {
		t.Fatalf("formatExportBatch: %v", err)
	}
	output := string(got)

	// Split into lines; last line must be the hash line
	lines := strings.Split(output, "\n")
	hashLine := lines[len(lines)-1]
	if !strings.HasPrefix(hashLine, "# sha256:") {
		t.Fatalf("last line is not a hash line: %q", hashLine)
	}
	gotHash := strings.TrimPrefix(hashLine, "# sha256:")

	// The hash must cover everything before the hash line (including the trailing \n of the count line)
	body := strings.TrimSuffix(output, hashLine)
	sum := sha256.Sum256([]byte(body))
	wantHash := fmt.Sprintf("%x", sum)

	if gotHash != wantHash {
		t.Errorf("hash mismatch: got %q, want %q", gotHash, wantHash)
	}

	// Sanity: file must not end with a newline
	if strings.HasSuffix(output, "\n") {
		t.Errorf("file must not end with a newline")
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

// --- Edge case tests for formatExportBatch ---

func TestFormatExportBatch_EmptyColumnsSlice(t *testing.T) {
	t.Parallel()

	headers := []string{}
	records := []ingitdb.RecordEntry{
		{Key: "1", Data: map[string]any{"id": "1", "name": "Alice"}},
	}

	got, err := formatExportBatch("tsv", "", headers, records)
	if err != nil {
		t.Fatalf("formatExportBatch: %v", err)
	}

	if string(got) != "\n\n" {
		t.Errorf("formatExportBatch with empty columns = %q, want %q", string(got), "\n\n")
	}
}

func TestFormatExportBatch_RecordsWithNilData(t *testing.T) {
	t.Parallel()

	headers := []string{"id", "name"}
	records := []ingitdb.RecordEntry{
		{Key: "1", Data: nil},
		{Key: "2", Data: map[string]any{"id": "2", "name": "Bob"}},
	}

	tests := []struct {
		format string
		check  func([]byte) bool
	}{
		{"tsv", func(b []byte) bool {
			// Should handle nil data gracefully
			return len(b) > 0
		}},
		{"csv", func(b []byte) bool {
			// Should handle nil data gracefully
			return len(b) > 0
		}},
		{"json", func(b []byte) bool {
			var result []map[string]any
			err := json.Unmarshal(b, &result)
			return err == nil && len(result) == 2
		}},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			t.Parallel()
			got, err := formatExportBatch(tt.format, "test/view", headers, records)
			if err != nil {
				t.Fatalf("formatExportBatch: %v", err)
			}
			if !tt.check(got) {
				t.Errorf("check failed for format %s", tt.format)
			}
		})
	}
}

func TestFormatExportBatch_UnicodeCharacters(t *testing.T) {
	t.Parallel()

	headers := []string{"id", "text"}
	records := []ingitdb.RecordEntry{
		{Key: "1", Data: map[string]any{"id": "1", "text": "Hello 你好 مرحبا 🎉"}},
		{Key: "2", Data: map[string]any{"id": "2", "text": "Café résumé naïve"}},
	}

	tests := []struct {
		format string
		check  func([]byte) bool
	}{
		{"tsv", func(b []byte) bool {
			content := string(b)
			return strings.Contains(content, "你好") && strings.Contains(content, "Café")
		}},
		{"csv", func(b []byte) bool {
			r := csv.NewReader(strings.NewReader(string(b)))
			records, err := r.ReadAll()
			return err == nil && len(records) == 3 && strings.Contains(records[1][1], "你好")
		}},
		{"json", func(b []byte) bool {
			var result []map[string]any
			err := json.Unmarshal(b, &result)
			return err == nil && len(result) == 2 && strings.Contains(result[0]["text"].(string), "你好")
		}},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			t.Parallel()
			got, err := formatExportBatch(tt.format, "test/view", headers, records)
			if err != nil {
				t.Fatalf("formatExportBatch: %v", err)
			}
			if !tt.check(got) {
				t.Errorf("unicode check failed for format %s", tt.format)
			}
		})
	}
}

func TestFormatExportBatch_VeryLongLines(t *testing.T) {
	t.Parallel()

	longString := strings.Repeat("x", 10000)
	headers := []string{"id", "longtext"}
	records := []ingitdb.RecordEntry{
		{Key: "1", Data: map[string]any{"id": "1", "longtext": longString}},
	}

	got, err := formatExportBatch("tsv", "", headers, records)
	if err != nil {
		t.Fatalf("formatExportBatch: %v", err)
	}

	if !strings.Contains(string(got), longString) {
		t.Errorf("very long line not preserved")
	}
	if len(got) < 10000 {
		t.Errorf("expected at least 10000 bytes, got %d", len(got))
	}
}

// --- TSV format escaping edge cases ---

func TestEscapeTSV_BackslashFollowedByCharacter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"backslash-t", `\t`, `\\t`},
		{"backslash-n", `\n`, `\\n`},
		{"backslash-r", `\r`, `\\r`},
		{"backslash-backslash", `\\`, `\\\\`},
		{"double backslash", `\\\\`, `\\\\\\\\`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := escapeTSV(tt.input)
			if got != tt.want {
				t.Errorf("escapeTSV(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestEscapeTSV_MultipleEscapesInValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		// Backslash followed by file path separators
		{`path\to\file\with\tabs`, `path\\to\\file\\with\\tabs`},
		// Actual tab and newline characters (not escaped sequences)
		{"quoted\tvalue", `quoted\tvalue`},
		{"line1\nline2\nline3", `line1\nline2\nline3`},
		// Mixed: backslashes and actual control characters
		{`path\to` + "\t" + `file`, `path\\to\tfile`},
		{"mixed\t\n\r\tescapes", `mixed\t\n\r\tescapes`},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%q", tt.input), func(t *testing.T) {
			t.Parallel()
			got := escapeTSV(tt.input)
			if got != tt.want {
				t.Errorf("escapeTSV(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFormatTSV_TabSeparationAndNoTrailingNewline(t *testing.T) {
	t.Parallel()

	headers := []string{"id", "name", "value"}
	records := []ingitdb.RecordEntry{
		{Key: "1", Data: map[string]any{"id": "1", "name": "Alice", "value": 100}},
		{Key: "2", Data: map[string]any{"id": "2", "name": "Bob", "value": 200}},
	}

	got, err := formatExportBatch("tsv", "", headers, records)
	if err != nil {
		t.Fatalf("formatExportBatch: %v", err)
	}

	content := string(got)
	lines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")

	// Check that headers are properly separated by tabs
	if lines[0] != "id\tname\tvalue" {
		t.Errorf("header line = %q, want %q", lines[0], "id\tname\tvalue")
	}

	// Check that data rows have proper tab separation
	if !strings.Contains(lines[1], "\t") {
		t.Errorf("data row should contain tabs, got %q", lines[1])
	}

	// Verify exactly 3 columns per row (separated by 2 tabs)
	for i, line := range lines {
		tabCount := strings.Count(line, "\t")
		if tabCount != 2 {
			t.Errorf("line %d has %d tabs, want 2", i, tabCount)
		}
	}
}

// --- CSV format edge cases ---

func TestFormatCSV_CRLF_LineEndings(t *testing.T) {
	t.Parallel()

	headers := []string{"id", "description"}
	records := []ingitdb.RecordEntry{
		{Key: "1", Data: map[string]any{"id": "1", "description": "Line 1\r\nLine 2"}},
	}

	got, err := formatExportBatch("csv", "", headers, records)
	if err != nil {
		t.Fatalf("formatExportBatch: %v", err)
	}

	r := csv.NewReader(strings.NewReader(string(got)))
	records_parsed, err := r.ReadAll()
	if err != nil {
		t.Fatalf("failed to parse CSV: %v", err)
	}

	if len(records_parsed) != 2 {
		t.Errorf("expected 2 rows (header + data), got %d", len(records_parsed))
	}

	// The CSV reader normalizes CRLF to LF, so we check for LF
	if !strings.Contains(records_parsed[1][1], "\n") {
		t.Errorf("newline not preserved in CSV value: %q", records_parsed[1][1])
	}
}

func TestFormatCSV_EmptyStringVsNil(t *testing.T) {
	t.Parallel()

	headers := []string{"id", "value"}
	records := []ingitdb.RecordEntry{
		{Key: "1", Data: map[string]any{"id": "1", "value": ""}},
		{Key: "2", Data: map[string]any{"id": "2", "value": nil}},
		{Key: "3", Data: map[string]any{"id": "3"}}, // Missing field
	}

	got, err := formatExportBatch("csv", "", headers, records)
	if err != nil {
		t.Fatalf("formatExportBatch: %v", err)
	}

	r := csv.NewReader(strings.NewReader(string(got)))
	parsed, err := r.ReadAll()
	if err != nil {
		t.Fatalf("failed to parse CSV: %v", err)
	}

	if len(parsed) != 4 {
		t.Errorf("expected 4 rows, got %d", len(parsed))
	}

	// Row 1: empty string
	if parsed[1][1] != "" {
		t.Errorf("row 1 value should be empty string, got %q", parsed[1][1])
	}

	// Row 2 & 3: nil / missing should both be empty
	if parsed[2][1] != "" {
		t.Errorf("row 2 value should be empty, got %q", parsed[2][1])
	}
	if parsed[3][1] != "" {
		t.Errorf("row 3 value should be empty, got %q", parsed[3][1])
	}
}

func TestFormatCSV_SingleAndDoubleQuoteCombinations(t *testing.T) {
	t.Parallel()

	headers := []string{"id", "text"}
	records := []ingitdb.RecordEntry{
		{Key: "1", Data: map[string]any{"id": "1", "text": `He said "hello"`}},
		{Key: "2", Data: map[string]any{"id": "2", "text": `It's a test`}},
		{Key: "3", Data: map[string]any{"id": "3", "text": `"Quoted" and 'apostrophe'`}},
	}

	got, err := formatExportBatch("csv", "", headers, records)
	if err != nil {
		t.Fatalf("formatExportBatch: %v", err)
	}

	r := csv.NewReader(strings.NewReader(string(got)))
	parsed, err := r.ReadAll()
	if err != nil {
		t.Fatalf("failed to parse CSV: %v", err)
	}

	if len(parsed) != 4 {
		t.Errorf("expected 4 rows, got %d", len(parsed))
	}

	if parsed[1][1] != `He said "hello"` {
		t.Errorf("row 1 value mismatch: got %q", parsed[1][1])
	}
	if parsed[2][1] != `It's a test` {
		t.Errorf("row 2 value mismatch: got %q", parsed[2][1])
	}
	if parsed[3][1] != `"Quoted" and 'apostrophe'` {
		t.Errorf("row 3 value mismatch: got %q", parsed[3][1])
	}
}

func TestFormatCSV_NumbersAndBooleans(t *testing.T) {
	t.Parallel()

	headers := []string{"id", "count", "enabled"}
	records := []ingitdb.RecordEntry{
		{Key: "1", Data: map[string]any{"id": "1", "count": 42, "enabled": true}},
		{Key: "2", Data: map[string]any{"id": "2", "count": 0, "enabled": false}},
		{Key: "3", Data: map[string]any{"id": "3", "count": 3.14159, "enabled": nil}},
	}

	got, err := formatExportBatch("csv", "", headers, records)
	if err != nil {
		t.Fatalf("formatExportBatch: %v", err)
	}

	r := csv.NewReader(strings.NewReader(string(got)))
	parsed, err := r.ReadAll()
	if err != nil {
		t.Fatalf("failed to parse CSV: %v", err)
	}

	if len(parsed) != 4 {
		t.Errorf("expected 4 rows, got %d", len(parsed))
	}

	// Verify numbers are formatted correctly
	if parsed[1][1] != "42" {
		t.Errorf("row 1 count should be 42, got %q", parsed[1][1])
	}
	if parsed[1][2] != "true" {
		t.Errorf("row 1 enabled should be true, got %q", parsed[1][2])
	}

	if parsed[2][1] != "0" {
		t.Errorf("row 2 count should be 0, got %q", parsed[2][1])
	}
	if parsed[2][2] != "false" {
		t.Errorf("row 2 enabled should be false, got %q", parsed[2][2])
	}
}

// --- JSONL format edge cases ---

func TestFormatJSONL_EachLineIsValidJSON(t *testing.T) {
	t.Parallel()

	headers := []string{"id", "name"}
	records := []ingitdb.RecordEntry{
		{Key: "1", Data: map[string]any{"id": "1", "name": "Alice"}},
		{Key: "2", Data: map[string]any{"id": "2", "name": "Bob"}},
		{Key: "3", Data: map[string]any{"id": "3", "name": "Charlie"}},
	}

	got, err := formatExportBatch("jsonl", "", headers, records)
	if err != nil {
		t.Fatalf("formatExportBatch: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(got)), "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d", len(lines))
	}

	for i, line := range lines {
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			t.Errorf("line %d is not valid JSON: %v", i, err)
		}
		if obj["id"] == "" {
			t.Errorf("line %d missing id field", i)
		}
	}
}

func TestFormatJSONL_WithSpecialCharactersAndUnicode(t *testing.T) {
	t.Parallel()

	headers := []string{"id", "text"}
	records := []ingitdb.RecordEntry{
		{Key: "1", Data: map[string]any{"id": "1", "text": "Hello\nWorld\t\u0000"}},
		{Key: "2", Data: map[string]any{"id": "2", "text": "日本語テスト"}},
	}

	got, err := formatExportBatch("jsonl", "", headers, records)
	if err != nil {
		t.Fatalf("formatExportBatch: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(got)), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d", len(lines))
	}

	// Verify each line is valid JSON
	var obj1, obj2 map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &obj1); err != nil {
		t.Fatalf("failed to unmarshal line 0: %v", err)
	}
	if err := json.Unmarshal([]byte(lines[1]), &obj2); err != nil {
		t.Fatalf("failed to unmarshal line 1: %v", err)
	}

	if obj1["id"] != "1" || obj2["id"] != "2" {
		t.Errorf("IDs mismatch")
	}
}

// --- Column ordering edge cases ---

func TestDetermineColumns_IdNotAtIndex0_ViewColumns(t *testing.T) {
	t.Parallel()

	col := &ingitdb.CollectionDef{
		ID:           "col1",
		ColumnsOrder: []string{"id", "x", "y"},
	}
	view := &ingitdb.ViewDef{
		Columns: []string{"x", "y", "id"},
	}

	got := determineColumns(col, view)
	expected := []string{"id", "x", "y"}

	if !slicesEqual(got, expected) {
		t.Errorf("determineColumns() = %v, want %v", got, expected)
	}
}

func TestDetermineColumns_IdAlreadyAtIndex0_ViewColumns(t *testing.T) {
	t.Parallel()

	col := &ingitdb.CollectionDef{
		ID:           "col1",
		ColumnsOrder: []string{"id", "x", "y"},
	}
	view := &ingitdb.ViewDef{
		Columns: []string{"id", "x", "y"},
	}

	got := determineColumns(col, view)
	expected := []string{"id", "x", "y"}

	if !slicesEqual(got, expected) {
		t.Errorf("determineColumns() = %v, want %v", got, expected)
	}
}

func TestDetermineColumns_IdNotInViewColumns(t *testing.T) {
	t.Parallel()

	col := &ingitdb.CollectionDef{
		ID:           "col1",
		ColumnsOrder: []string{"id", "a", "b", "c"},
	}
	view := &ingitdb.ViewDef{
		Columns: []string{"a", "b"},
	}

	got := determineColumns(col, view)
	expected := []string{"id", "a", "b"}

	if !slicesEqual(got, expected) {
		t.Errorf("determineColumns() = %v, want %v", got, expected)
	}
}

func TestDetermineColumns_UseCollectionColumnsOrder_IdNotAtIndex0(t *testing.T) {
	t.Parallel()

	col := &ingitdb.CollectionDef{
		ID:           "col1",
		ColumnsOrder: []string{"name", "id", "email"},
	}
	view := &ingitdb.ViewDef{
		Columns: []string{},
	}

	got := determineColumns(col, view)
	expected := []string{"id", "name", "email"}

	if !slicesEqual(got, expected) {
		t.Errorf("determineColumns() = %v, want %v", got, expected)
	}
}

func TestDetermineColumns_EmptyCollectionColumnsOrder(t *testing.T) {
	t.Parallel()

	col := &ingitdb.CollectionDef{
		ID:           "col1",
		ColumnsOrder: []string{},
	}
	view := &ingitdb.ViewDef{
		Columns: []string{},
	}

	got := determineColumns(col, view)
	expected := []string{"id"}

	if !slicesEqual(got, expected) {
		t.Errorf("determineColumns() = %v, want %v", got, expected)
	}
}

func TestDetermineColumns_FallbackToColumnsSortedByName(t *testing.T) {
	t.Parallel()

	col := &ingitdb.CollectionDef{
		ID: "col1",
		Columns: map[string]*ingitdb.ColumnDef{
			"zebra":      {Type: "string"},
			"apple":      {Type: "string"},
			"mango":      {Type: "number"},
		},
		// ColumnsOrder intentionally empty — should fall back to sorted Columns keys
	}
	view := &ingitdb.ViewDef{}

	got := determineColumns(col, view)
	expected := []string{"id", "apple", "mango", "zebra"}

	if !slicesEqual(got, expected) {
		t.Errorf("determineColumns() = %v, want %v", got, expected)
	}
}

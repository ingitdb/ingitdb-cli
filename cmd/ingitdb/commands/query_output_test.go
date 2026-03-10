package commands

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

var testRecords = []map[string]any{
	{"$id": "us", "name": "United States", "population": 330000000},
	{"$id": "de", "name": "Germany", "population": 83000000},
}

var testColumns = []string{"$id", "name", "population"}

var testRecordsWithComplex = []map[string]any{
	{
		"$id":   "fr",
		"name":  "France",
		"tags":  []any{"europe", "g7"},
		"meta":  map[string]any{"capital": "Paris", "continent": "Europe"},
		"score": 42,
	},
}

func TestWriteCSV(t *testing.T) {
	t.Parallel()

	t.Run("header and data rows", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		if err := writeCSV(&buf, testRecords, testColumns); err != nil {
			t.Fatalf("writeCSV: %v", err)
		}
		lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
		if len(lines) != 3 { // header + 2 data rows
			t.Fatalf("expected 3 lines, got %d:\n%s", len(lines), buf.String())
		}
		if !strings.HasPrefix(lines[0], "$id") {
			t.Errorf("expected header to start with $id, got %q", lines[0])
		}
		if !strings.Contains(lines[0], "name") {
			t.Errorf("expected header to contain 'name', got %q", lines[0])
		}
	})

	t.Run("empty result set", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		if err := writeCSV(&buf, nil, testColumns); err != nil {
			t.Fatalf("writeCSV: %v", err)
		}
		lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
		if len(lines) != 1 { // header only
			t.Fatalf("expected 1 line (header only), got %d:\n%s", len(lines), buf.String())
		}
	})

	t.Run("auto-detect columns when none specified", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		if err := writeCSV(&buf, testRecords, nil); err != nil {
			t.Fatalf("writeCSV: %v", err)
		}
		if buf.Len() == 0 {
			t.Error("expected non-empty output")
		}
	})
}

func TestFormatCSVCell(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input any
		want  string
	}{
		{name: "nil", input: nil, want: ""},
		{name: "string", input: "hello", want: "hello"},
		{name: "int", input: 42, want: "42"},
		{name: "float", input: 3.14, want: "3.14"},
		{name: "bool", input: true, want: "true"},
		{name: "map", input: map[string]any{"capital": "Paris"}, want: `{"capital":"Paris"}`},
		{name: "slice", input: []any{"europe", "g7"}, want: `["europe","g7"]`},
		{name: "empty map", input: map[string]any{}, want: `{}`},
		{name: "empty slice", input: []any{}, want: `[]`},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := formatCSVCell(tt.input)
			if got != tt.want {
				t.Errorf("formatCSVCell(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestWriteCSV_ComplexFields(t *testing.T) {
	t.Parallel()

	cols := []string{"$id", "name", "tags", "meta", "score"}
	var buf bytes.Buffer
	if err := writeCSV(&buf, testRecordsWithComplex, cols); err != nil {
		t.Fatalf("writeCSV: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 { // header + 1 data row
		t.Fatalf("expected 2 lines, got %d:\n%s", len(lines), buf.String())
	}

	// Parse the data row as CSV to get properly unquoted cell values.
	r := csv.NewReader(strings.NewReader(lines[1]))
	cells, err := r.Read()
	if err != nil {
		t.Fatalf("failed to parse CSV data row: %v", err)
	}
	if len(cells) != len(cols) {
		t.Fatalf("expected %d cells, got %d: %v", len(cols), len(cells), cells)
	}

	// tags ([]any) must be JSON array.
	tagsCell := cells[2]
	var tags []string
	if err = json.Unmarshal([]byte(tagsCell), &tags); err != nil {
		t.Errorf("tags cell %q is not valid JSON array: %v", tagsCell, err)
	} else if len(tags) != 2 || tags[0] != "europe" {
		t.Errorf("unexpected tags: %v", tags)
	}

	// meta (map[string]any) must be JSON object.
	metaCell := cells[3]
	var meta map[string]any
	if err = json.Unmarshal([]byte(metaCell), &meta); err != nil {
		t.Errorf("meta cell %q is not valid JSON object: %v", metaCell, err)
	} else if meta["capital"] != "Paris" {
		t.Errorf("unexpected meta: %v", meta)
	}

	// score (int) must be plain scalar.
	if cells[4] != "42" {
		t.Errorf("expected score cell '42', got %q", cells[4])
	}
}

func TestWriteJSON(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	if err := writeJSON(&buf, testRecords); err != nil {
		t.Fatalf("writeJSON: %v", err)
	}
	var parsed []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	if len(parsed) != len(testRecords) {
		t.Errorf("expected %d records, got %d", len(testRecords), len(parsed))
	}
}

func TestWriteJSON_Empty(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	if err := writeJSON(&buf, nil); err != nil {
		t.Fatalf("writeJSON: %v", err)
	}
	var parsed []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
}

func TestWriteYAML(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	if err := writeYAML(&buf, testRecords); err != nil {
		t.Fatalf("writeYAML: %v", err)
	}
	var parsed []map[string]any
	if err := yaml.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("failed to parse YAML output: %v", err)
	}
	if len(parsed) != len(testRecords) {
		t.Errorf("expected %d records, got %d", len(testRecords), len(parsed))
	}
}

func TestWriteMarkdown(t *testing.T) {
	t.Parallel()

	t.Run("header separator and data rows", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		if err := writeMarkdown(&buf, testRecords, testColumns); err != nil {
			t.Fatalf("writeMarkdown: %v", err)
		}
		lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
		if len(lines) != 4 { // header + separator + 2 data rows
			t.Fatalf("expected 4 lines, got %d:\n%s", len(lines), buf.String())
		}
		// Separator row must contain ---
		if !strings.Contains(lines[1], "---") {
			t.Errorf("expected separator row to contain ---, got %q", lines[1])
		}
		// Header must be a pipe-separated row
		if !strings.HasPrefix(lines[0], "|") {
			t.Errorf("expected header to start with |, got %q", lines[0])
		}
	})

	t.Run("empty result", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		if err := writeMarkdown(&buf, nil, testColumns); err != nil {
			t.Fatalf("writeMarkdown: %v", err)
		}
		lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
		if len(lines) != 2 { // header + separator only
			t.Fatalf("expected 2 lines (header + separator), got %d:\n%s", len(lines), buf.String())
		}
	})
}

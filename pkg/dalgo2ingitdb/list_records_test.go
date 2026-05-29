package dalgo2ingitdb

import (
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func TestParseListOfRecordsContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  string
		format   ingitdb.RecordFormat
		wantLen  int
		wantErr  bool
		wantKey0 string // value of row[0]["$id"] when wantLen > 0
	}{
		{
			name:     "yaml sequence",
			content:  "- $id: a\n  v: 1\n- $id: b\n  v: 2\n",
			format:   ingitdb.RecordFormatYAML,
			wantLen:  2,
			wantKey0: "a",
		},
		{name: "yaml empty", content: "  \n", format: ingitdb.RecordFormatYAML, wantLen: 0},
		{name: "yaml invalid", content: "- a:\n\t- bad\n", format: ingitdb.RecordFormatYAML, wantErr: true},
		{
			name:     "json array",
			content:  `[{"$id":"a","v":1},{"$id":"b","v":2}]`,
			format:   ingitdb.RecordFormatJSON,
			wantLen:  2,
			wantKey0: "a",
		},
		{name: "json empty", content: "   ", format: ingitdb.RecordFormatJSON, wantLen: 0},
		{name: "json invalid", content: "[not json", format: ingitdb.RecordFormatJSON, wantErr: true},
		{
			name:     "jsonl stream with blank lines",
			content:  "{\"$id\":\"a\"}\n\n{\"$id\":\"b\"}\n",
			format:   ingitdb.RecordFormatJSONL,
			wantLen:  2,
			wantKey0: "a",
		},
		{name: "jsonl empty", content: "\n\n", format: ingitdb.RecordFormatJSONL, wantLen: 0},
		{name: "jsonl invalid line", content: "{\"$id\":\"a\"}\nnope\n", format: ingitdb.RecordFormatJSONL, wantErr: true},
		{name: "yml alias", content: "- $id: a\n", format: ingitdb.RecordFormatYML, wantLen: 1, wantKey0: "a"},
		{name: "unsupported format", content: "x", format: ingitdb.RecordFormatTOML, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rows, err := ParseListOfRecordsContent([]byte(tt.content), tt.format)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(rows) != tt.wantLen {
				t.Fatalf("len = %d, want %d (%v)", len(rows), tt.wantLen, rows)
			}
			if tt.wantLen > 0 && rows[0]["$id"] != tt.wantKey0 {
				t.Errorf("row[0][$id] = %v, want %v", rows[0]["$id"], tt.wantKey0)
			}
		})
	}
}

func TestResolveListRecordKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		row     map[string]any
		col     *ingitdb.CollectionDef
		wantKey string
		wantOK  bool
	}{
		{
			name:    "primary key single",
			row:     map[string]any{"code": "X1", "v": 1},
			col:     &ingitdb.CollectionDef{PrimaryKey: []string{"code"}},
			wantKey: "X1",
			wantOK:  true,
		},
		{
			name:    "primary key composite",
			row:     map[string]any{"a": "1", "b": "2"},
			col:     &ingitdb.CollectionDef{PrimaryKey: []string{"a", "b"}},
			wantKey: "1\x1f2",
			wantOK:  true,
		},
		{
			name:    "dollar id when no primary key",
			row:     map[string]any{"$id": "z", "id": "ignored"},
			col:     &ingitdb.CollectionDef{},
			wantKey: "z",
			wantOK:  true,
		},
		{
			name:    "id fallback",
			row:     map[string]any{"id": "k"},
			col:     &ingitdb.CollectionDef{},
			wantKey: "k",
			wantOK:  true,
		},
		{
			name:    "nil collection uses id fields",
			row:     map[string]any{"$id": "n"},
			col:     nil,
			wantKey: "n",
			wantOK:  true,
		},
		{
			name:   "no key available",
			row:    map[string]any{"name": "Alex"},
			col:    &ingitdb.CollectionDef{},
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			key, ok := ResolveListRecordKey(tt.row, tt.col)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && key != tt.wantKey {
				t.Errorf("key = %q, want %q", key, tt.wantKey)
			}
		})
	}
}

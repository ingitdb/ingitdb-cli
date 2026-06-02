package ingitdb

import (
	"strings"
	"testing"
)

func collectionWithColumns(columns map[string]*ColumnDef) *CollectionDef {
	return &CollectionDef{
		ID:      "people",
		Columns: columns,
		RecordFile: &RecordFileDef{
			Format:     "JSON",
			Name:       "{key}.json",
			RecordType: SingleRecord,
		},
	}
}

func TestValidateComputedColumn(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		columns map[string]*ColumnDef
		wantErr []string // substrings that must all be present; empty => expect success
	}{
		{
			name: "valid_single_expression",
			columns: map[string]*ColumnDef{
				"first_name": {Type: ColumnTypeString},
				"last_name":  {Type: ColumnTypeString},
				"full_name":  {Type: ColumnTypeString, Formula: "first_name + ' ' + last_name"},
			},
		},
		{
			name: "type_string",
			columns: map[string]*ColumnDef{
				"a":      {Type: ColumnTypeString},
				"result": {Type: ColumnTypeString, Formula: "a"},
			},
		},
		{
			name: "type_int",
			columns: map[string]*ColumnDef{
				"a":      {Type: ColumnTypeInt},
				"result": {Type: ColumnTypeInt, Formula: "a + 1"},
			},
		},
		{
			name: "type_float",
			columns: map[string]*ColumnDef{
				"a":      {Type: ColumnTypeFloat},
				"result": {Type: ColumnTypeFloat, Formula: "a * 2.0"},
			},
		},
		{
			name: "type_bool",
			columns: map[string]*ColumnDef{
				"a":      {Type: ColumnTypeInt},
				"result": {Type: ColumnTypeBool, Formula: "a > 0"},
			},
		},
		{
			name: "type_any",
			columns: map[string]*ColumnDef{
				"a":      {Type: ColumnTypeString},
				"result": {Type: ColumnTypeAny, Formula: "a"},
			},
		},
		{
			name: "references_stored_sibling_accepted",
			columns: map[string]*ColumnDef{
				"first_name": {Type: ColumnTypeString},
				"greeting":   {Type: ColumnTypeString, Formula: "'Hi ' + first_name"},
			},
		},
		{
			name: "syntax_error_incomplete_expression",
			columns: map[string]*ColumnDef{
				"first_name": {Type: ColumnTypeString},
				"full_name":  {Type: ColumnTypeString, Formula: "first_name +"},
			},
			wantErr: []string{"people", "full_name", "invalid formula"},
		},
		{
			name: "statement_body_rejected",
			columns: map[string]*ColumnDef{
				"x": {Type: ColumnTypeInt, Formula: "x = 1"},
			},
			wantErr: []string{"people", "x", "invalid formula"},
		},
		{
			name: "unsupported_type_datetime",
			columns: map[string]*ColumnDef{
				"a":         {Type: ColumnTypeString},
				"timestamp": {Type: ColumnTypeDateTime, Formula: "a"},
			},
			wantErr: []string{"people", "timestamp", "unsupported type"},
		},
		{
			name: "references_computed_sibling_rejected",
			columns: map[string]*ColumnDef{
				"first_name": {Type: ColumnTypeString},
				"last_name":  {Type: ColumnTypeString},
				"full_name":  {Type: ColumnTypeString, Formula: "first_name + ' ' + last_name"},
				"greeting":   {Type: ColumnTypeString, Formula: "'Hi ' + full_name"},
			},
			wantErr: []string{"people", "greeting", "full_name", "stored fields"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			def := collectionWithColumns(tt.columns)
			err := def.Validate()

			if len(tt.wantErr) == 0 {
				if err != nil {
					t.Fatalf("expected no error, got %s", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("expected error containing %v, got nil", tt.wantErr)
			}
			errMsg := err.Error()
			for _, want := range tt.wantErr {
				if !strings.Contains(errMsg, want) {
					t.Fatalf("expected error to contain %q, got %q", want, errMsg)
				}
			}
		})
	}
}

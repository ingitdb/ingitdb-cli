package ingitdb

import (
	"strings"
	"testing"
)

func TestCollectionDefValidate_Errors(t *testing.T) {
	t.Parallel()

	columns := map[string]*ColumnDef{
		"name": {Type: "string"},
	}
	recordFile := &RecordFileDef{
		Format:     "JSON",
		Name:       "{key}.json",
		RecordType: SingleRecord,
	}

	tests := []struct {
		name string
		def  *CollectionDef
		err  string
	}{
		{
			name: "missing_columns",
			def: &CollectionDef{
				ID:         "test_id",
				Columns:    map[string]*ColumnDef{},
				RecordFile: recordFile,
			},
			err: "missing 'columns' in collection definition",
		},
		{
			name: "missing_column_type",
			def: &CollectionDef{
				ID: "test_id",
				Columns: map[string]*ColumnDef{
					"name": {},
				},
				RecordFile: recordFile,
			},
			err: "invalid column 'name': missing 'type' in column definition",
		},
		{
			name: "columns_order_unknown_column",
			def: &CollectionDef{
				ID:           "test_id",
				Columns:      columns,
				ColumnsOrder: []string{"age"},
				RecordFile:   recordFile,
			},
			err: "columns_order[0] references unspecified column: age",
		},
		{
			name: "columns_order_duplicate",
			def: &CollectionDef{
				ID: "test_id",
				Columns: map[string]*ColumnDef{
					"name": {Type: "string"},
					"age":  {Type: "int"},
				},
				ColumnsOrder: []string{"name", "age", "name"},
				RecordFile:   recordFile,
			},
			err: "duplicate value in columns_order at indexes 0 and 2: name",
		},
		{
			name: "missing_record_file",
			def: &CollectionDef{
				ID:      "test_id",
				Columns: columns,
			},
			err: "missing 'record_file' in collection definition",
		},
		{
			name: "invalid_record_file",
			def: &CollectionDef{
				ID:         "test_id",
				Columns:    columns,
				RecordFile: &RecordFileDef{},
			},
			err: "invalid record_file definition",
		},
		{
			name: "invalid_view",
			def: &CollectionDef{
				ID:         "test_id",
				Columns:    columns,
				RecordFile: recordFile,
				Views: map[string]*ViewDef{
					"readme": {},
				},
			},
			err: "invalid view 'readme'",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.def.Validate()
			if err == nil {
				t.Fatalf("got nil instead of expected error: %s", tt.err)
			}
			errMsg := err.Error()
			if !strings.Contains(errMsg, tt.err) {
				t.Fatalf("expected error to contain %q, got %q", tt.err, errMsg)
			}
		})
	}
}

func TestCollectionDefValidate_Success(t *testing.T) {
	t.Parallel()

	def := &CollectionDef{
		ID: "test_id",
		Columns: map[string]*ColumnDef{
			"name": {Type: "string"},
		},
		ColumnsOrder: []string{"name"},
		RecordFile: &RecordFileDef{
			Format:     "JSON",
			RecordType: "map[string]any",
			Name:       "{key}.json",
		},
		Views: map[string]*ViewDef{
			"readme": {ID: "readme", OrderBy: "title"},
		},
		Readme: &CollectionReadmeDef{
			DataPreview: &ViewDef{
				ID: "preview",
			},
		},
	}

	err := def.Validate()
	if err != nil {
		errMsg := err.Error()
		t.Fatalf("expected no error, got %s", errMsg)
	}

	if def.Readme.DataPreview.Template != "md-table" {
		t.Fatalf("expected DataPreview.Template to be defaulted to 'md-table'")
	}
}

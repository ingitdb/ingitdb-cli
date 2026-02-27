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

func TestCollectionDefValidate_DefaultView(t *testing.T) {
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
		name        string
		defaultView *ViewDef
		wantErr     bool
		errMsg      string
	}{
		{
			name:        "valid_default_view",
			defaultView: &ViewDef{},
			wantErr:     false,
		},
		{
			name: "invalid_default_view_format",
			defaultView: &ViewDef{
				Format: "invalid",
			},
			wantErr: true,
			errMsg:  "invalid default_view",
		},
		{
			name: "invalid_default_view_batch_size",
			defaultView: &ViewDef{
				MaxBatchSize: -5,
			},
			wantErr: true,
			errMsg:  "invalid default_view",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			def := &CollectionDef{
				ID:          "test_id",
				Columns:     columns,
				RecordFile:  recordFile,
				DefaultView: tt.defaultView,
			}

			err := def.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("got error %v, want error %v", err, tt.wantErr)
			}

			if tt.wantErr && !strings.Contains(err.Error(), tt.errMsg) {
				t.Fatalf("expected error to contain %q, got %q", tt.errMsg, err.Error())
			}

			// If no error, verify DefaultView.ID was set
			if !tt.wantErr && tt.defaultView != nil {
				if def.DefaultView.ID != "default_view" {
					t.Fatalf("expected DefaultView.ID to be 'default_view', got %q", def.DefaultView.ID)
				}
			}
		})
	}
}

func TestCollectionDefValidate_MultipleIsDefault(t *testing.T) {
	t.Parallel()

	columns := map[string]*ColumnDef{
		"name": {Type: "string"},
	}
	recordFile := &RecordFileDef{
		Format:     "JSON",
		Name:       "{key}.json",
		RecordType: SingleRecord,
	}

	def := &CollectionDef{
		ID:         "test_id",
		Columns:    columns,
		RecordFile: recordFile,
		Views: map[string]*ViewDef{
			"view1": {ID: "view1", IsDefault: true},
			"view2": {ID: "view2", IsDefault: true},
		},
	}

	err := def.Validate()
	if err == nil {
		t.Fatal("expected error for multiple views with IsDefault set, got nil")
	}

	if !strings.Contains(err.Error(), "multiple views with IsDefault set") {
		t.Fatalf("expected error to contain 'multiple views with IsDefault set', got %q", err.Error())
	}
}

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
			name: "missing_id",
			def: &CollectionDef{
				ID:         "",
				Columns:    columns,
				RecordFile: recordFile,
			},
			err: "missing 'id' in collection definition",
		},
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
				if def.DefaultView.ID != DefaultViewID {
					t.Fatalf("expected DefaultView.ID to be %q, got %q", DefaultViewID, def.DefaultView.ID)
				}
			}
		})
	}
}

func TestCollectionDefValidate_InvalidSubCollection(t *testing.T) {
	t.Parallel()

	columns := map[string]*ColumnDef{
		"name": {Type: "string"},
	}
	recordFile := &RecordFileDef{
		Format:     "JSON",
		Name:       "{key}.json",
		RecordType: SingleRecord,
	}

	// Subcollection has no ID → Validate() on it returns "missing 'id'".
	def := &CollectionDef{
		ID:         "parent",
		Columns:    columns,
		RecordFile: recordFile,
		SubCollections: map[string]*CollectionDef{
			"child": {
				// ID intentionally left empty so child.Validate() fails.
				Columns:    columns,
				RecordFile: recordFile,
			},
		},
	}

	err := def.Validate()
	if err == nil {
		t.Fatal("expected error for invalid subcollection, got nil")
	}
	if !strings.Contains(err.Error(), "invalid subcollection 'child'") {
		t.Fatalf("expected error to contain %q, got %q", "invalid subcollection 'child'", err.Error())
	}
}

func TestCollectionDefValidate_InvalidReadme(t *testing.T) {
	t.Parallel()

	columns := map[string]*ColumnDef{
		"name": {Type: "string"},
	}
	recordFile := &RecordFileDef{
		Format:     "JSON",
		Name:       "{key}.json",
		RecordType: SingleRecord,
	}

	// DataPreview with no ID causes ViewDef.Validate() to fail, which bubbles
	// through CollectionReadmeDef.Validate() → CollectionDef.Validate().
	def := &CollectionDef{
		ID:         "test_id",
		Columns:    columns,
		RecordFile: recordFile,
		Readme: &CollectionReadmeDef{
			DataPreview: &ViewDef{
				// ID deliberately empty → ViewDef.Validate() returns error.
			},
		},
	}

	err := def.Validate()
	if err == nil {
		t.Fatal("expected error for invalid readme DataPreview, got nil")
	}
	if !strings.Contains(err.Error(), "invalid readme") {
		t.Fatalf("expected error to contain %q, got %q", "invalid readme", err.Error())
	}
}

func TestCollectionReadmeDefValidate_InvalidDataPreview(t *testing.T) {
	t.Parallel()

	// Test CollectionReadmeDef.Validate() directly so the error branch is
	// exercised independently of CollectionDef.Validate().
	readme := &CollectionReadmeDef{
		DataPreview: &ViewDef{
			// ID empty → ViewDef.Validate() returns "missing 'id' in view definition".
		},
	}

	err := readme.Validate()
	if err == nil {
		t.Fatal("expected error for DataPreview with missing ID, got nil")
	}
	if !strings.Contains(err.Error(), "invalid data_preview") {
		t.Fatalf("expected error to contain %q, got %q", "invalid data_preview", err.Error())
	}
}

func TestCollectionReadmeDefValidate_NilDataPreview(t *testing.T) {
	t.Parallel()

	readme := &CollectionReadmeDef{HideColumns: true}

	err := readme.Validate()
	if err != nil {
		t.Fatalf("expected no error for nil DataPreview, got %v", err)
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

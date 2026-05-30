package ingitdb

import (
	"strings"
	"testing"

	"github.com/dal-go/dalgo/dal"
)

func TestRecordFileDefValidate_MissingBranches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		def     RecordFileDef
		wantErr string
	}{
		{
			name: "markdown_requires_single_record",
			def: RecordFileDef{
				Name:       "{key}.md",
				Format:     RecordFormatMarkdown,
				RecordType: ListOfRecords,
			},
			wantErr: "format \"markdown\" requires record type",
		},
		{
			name: "content_field_only_for_markdown",
			def: RecordFileDef{
				Name:         "{key}.yaml",
				Format:       RecordFormatYAML,
				RecordType:   SingleRecord,
				ContentField: "body",
			},
			wantErr: "content_field is only valid for format",
		},
		{
			name: "ingr_rejects_single_record",
			def: RecordFileDef{
				Name:       "records.ingr",
				Format:     RecordFormatINGR,
				RecordType: SingleRecord,
			},
			wantErr: "format \"ingr\" does not support record type",
		},
		{
			name: "invalid_exclude_regex",
			def: RecordFileDef{
				Name:         "{key}.yaml",
				Format:       RecordFormatYAML,
				RecordType:   SingleRecord,
				ExcludeRegex: "[invalid",
			},
			wantErr: "invalid exclude_regex",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.def.Validate()
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestRecordFileDef_IsExcluded(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		excludeRegex string
		filename     string
		want         bool
	}{
		{
			name:         "empty_regex_never_excludes",
			excludeRegex: "",
			filename:     "README.md",
			want:         false,
		},
		{
			name:         "matching_regex_excludes",
			excludeRegex: `^README\.md$`,
			filename:     "README.md",
			want:         true,
		},
		{
			name:         "non_matching_regex_does_not_exclude",
			excludeRegex: `^README\.md$`,
			filename:     "record-1.md",
			want:         false,
		},
		{
			name:         "invalid_regex_returns_false",
			excludeRegex: "[invalid",
			filename:     "README.md",
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rfd := RecordFileDef{ExcludeRegex: tt.excludeRegex}
			got := rfd.IsExcluded(tt.filename)
			if got != tt.want {
				t.Errorf("IsExcluded(%q) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}

func TestRecordFileDef_ResolvedContentField(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		contentField string
		want         string
	}{
		{
			name:         "empty_returns_default",
			contentField: "",
			want:         DefaultMarkdownContentField,
		},
		{
			name:         "custom_field_returned_as_is",
			contentField: "body",
			want:         "body",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rfd := RecordFileDef{ContentField: tt.contentField}
			got := rfd.ResolvedContentField()
			if got != tt.want {
				t.Errorf("ResolvedContentField() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRecordFileDef_RecordsBasePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		rfd  RecordFileDef
		want string
	}{
		{
			name: "name_with_key_placeholder_returns_records",
			rfd:  RecordFileDef{Name: "{key}.yaml"},
			want: "$records",
		},
		{
			name: "name_without_key_placeholder_returns_empty",
			rfd:  RecordFileDef{Name: "records.yaml"},
			want: "",
		},
		{
			name: "static_name_returns_empty",
			rfd:  RecordFileDef{Name: "data.json"},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.rfd.RecordsBasePath()
			if got != tt.want {
				t.Errorf("RecordsBasePath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRecordFileDefValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		def  RecordFileDef
		err  string
	}{
		{
			name: "missing_format",
			def:  RecordFileDef{Name: "file.json", RecordType: "map[string]any"},
			err:  "record file format cannot be empty",
		},
		{
			name: "missing_name",
			def:  RecordFileDef{Format: "JSON", RecordType: "map[string]any"},
			err:  "record file name cannot be empty",
		},
		{
			name: "missing_record_type",
			def:  RecordFileDef{Name: "file.json", Format: "JSON"},
			err:  "invalid record type",
		},
		{
			name: "valid",
			def: RecordFileDef{
				Name:       "file.json",
				Format:     "JSON",
				RecordType: "map[string]any",
			},
			err: "",
		},
		{
			name: "valid_map_of_id_records",
			def: RecordFileDef{
				Name:       "records.json",
				Format:     "json",
				RecordType: "map[$record_id]map[$field_name]any",
			},
			err: "",
		},
		{
			name: "csv_rejects_single_record",
			def:  RecordFileDef{Name: "records.csv", Format: RecordFormatCSV, RecordType: SingleRecord},
			err:  "format \"csv\" requires record type \"[]map[string]any\"",
		},
		{
			name: "csv_rejects_map_of_records",
			def:  RecordFileDef{Name: "records.csv", Format: RecordFormatCSV, RecordType: MapOfRecords},
			err:  "format \"csv\" requires record type \"[]map[string]any\"",
		},
		{
			name: "csv_accepts_list_of_records",
			def:  RecordFileDef{Name: "records.csv", Format: RecordFormatCSV, RecordType: ListOfRecords},
			err:  "",
		},
		{
			name: "jsonl_rejects_single_record",
			def:  RecordFileDef{Name: "records.jsonl", Format: RecordFormatJSONL, RecordType: SingleRecord},
			err:  "format \"jsonl\" requires record type \"[]map[string]any\"",
		},
		{
			name: "jsonl_rejects_map_of_records",
			def:  RecordFileDef{Name: "records.jsonl", Format: RecordFormatJSONL, RecordType: MapOfRecords},
			err:  "format \"jsonl\" requires record type \"[]map[string]any\"",
		},
		{
			name: "jsonl_accepts_list_of_records",
			def:  RecordFileDef{Name: "records.jsonl", Format: RecordFormatJSONL, RecordType: ListOfRecords},
			err:  "",
		},
		{
			name: "yaml_accepts_list_of_records",
			def:  RecordFileDef{Name: "records.yaml", Format: RecordFormatYAML, RecordType: ListOfRecords},
			err:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.def.Validate()
			if tt.err == "" && err != nil {
				errMsg := err.Error()
				t.Fatalf("expected no error, got %s", errMsg)
			}
			if tt.err != "" {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				errMsg := err.Error()
				if !strings.Contains(errMsg, tt.err) {
					t.Fatalf("expected error to contain %q, got %q", tt.err, errMsg)
				}
			}
		})
	}
}

func TestRecordFileDefGetRecordFileName(t *testing.T) {
	t.Parallel()

	key := dal.NewKeyWithID("tasks", "task-1")
	keyString := key.String()
	data := map[string]any{
		"":    "val",
		"foo": "bar",
	}
	record := dal.NewRecordWithData(key, data)
	record.SetError(nil)

	tests := []struct {
		name string
		def  RecordFileDef
		want string
	}{
		{
			name: "key_and_empty_placeholder",
			def:  RecordFileDef{Format: "JSON", Name: "file-{}-{key}.json"},
			want: "file-" + "val" + "-" + keyString + ".json",
		},
		{
			name: "replaces_only_first_key_placeholder",
			def:  RecordFileDef{Format: "JSON", Name: "{key}-second-{key}.json"},
			want: keyString + "-second-{key}.json",
		},
		{
			name: "replaces_only_first_value_placeholder",
			def:  RecordFileDef{Format: "JSON", Name: "file-{}-again-{}.json"},
			want: "file-" + "val" + "-again-{}.json",
		},
		{
			name: "static_name",
			def:  RecordFileDef{Format: "JSON", Name: "static.json"},
			want: "static.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.def.GetRecordFileName(record)
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

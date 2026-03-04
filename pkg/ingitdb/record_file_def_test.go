package ingitdb

import (
	"strings"
	"testing"

	"github.com/dal-go/dalgo/dal"
)

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
	}

	for _, tt := range tests {
		tt := tt
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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.def.GetRecordFileName(record)
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

package ingitdb

import (
	"testing"
)

func TestNewMapRecordEntry_GetID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		entry IRecordEntry
		want  string
	}{
		{
			name:  "string_key",
			entry: NewMapRecordEntry("task-42", map[string]any{"title": "Buy milk"}),
			want:  "task-42",
		},
		{
			name:  "int_key",
			entry: NewMapRecordEntry(99, map[string]any{"value": 99}),
			want:  "99",
		},
		{
			name:  "empty_string_key",
			entry: NewMapRecordEntry("", map[string]any{}),
			want:  "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := tc.entry.GetID()
			if got != tc.want {
				t.Errorf("GetID() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestNewMapRecordEntry_GetData(t *testing.T) {
	t.Parallel()

	data := map[string]any{"title": "Buy milk", "done": false}
	entry := NewMapRecordEntry("task-1", data)

	got := entry.GetData()
	if len(got) != len(data) {
		t.Fatalf("GetData() returned %d keys, want %d", len(got), len(data))
	}
	if got["title"] != "Buy milk" {
		t.Errorf("GetData()[\"title\"] = %v, want %q", got["title"], "Buy milk")
	}
	if got["done"] != false {
		t.Errorf("GetData()[\"done\"] = %v, want false", got["done"])
	}
}

func TestNewMapRecordEntry_NilData(t *testing.T) {
	t.Parallel()

	entry := NewMapRecordEntry("key-1", nil)

	if got := entry.GetID(); got != "key-1" {
		t.Errorf("GetID() = %q, want %q", got, "key-1")
	}
	if got := entry.GetData(); got != nil {
		t.Errorf("GetData() = %v, want nil", got)
	}
}

func TestRecordEntry_GetID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		entry RecordEntry
		want  string
	}{
		{
			name:  "non_empty_id",
			entry: RecordEntry{ID: "rec-7", Data: map[string]any{"x": 1}},
			want:  "rec-7",
		},
		{
			name:  "empty_id_list_type_file",
			entry: RecordEntry{ID: "", Data: map[string]any{"y": 2}},
			want:  "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := tc.entry.GetID()
			if got != tc.want {
				t.Errorf("GetID() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRecordEntry_GetData(t *testing.T) {
	t.Parallel()

	data := map[string]any{"field": "value", "count": 42}
	entry := RecordEntry{ID: "rec-1", Data: data}

	got := entry.GetData()
	if len(got) != len(data) {
		t.Fatalf("GetData() returned %d keys, want %d", len(got), len(data))
	}
	if got["field"] != "value" {
		t.Errorf("GetData()[\"field\"] = %v, want %q", got["field"], "value")
	}
	if got["count"] != 42 {
		t.Errorf("GetData()[\"count\"] = %v, want 42", got["count"])
	}
}

func TestRecordEntry_ImplementsIRecordEntry(t *testing.T) {
	t.Parallel()

	// Compile-time assertion is already in record_entry.go via var _ IRecordEntry = (*RecordEntry)(nil).
	// This test confirms the interface is satisfied at runtime with a value receiver.
	var entry IRecordEntry = RecordEntry{ID: "r", Data: map[string]any{"k": "v"}}

	if entry.GetID() != "r" {
		t.Errorf("GetID() = %q, want %q", entry.GetID(), "r")
	}
	data := entry.GetData()
	if data["k"] != "v" {
		t.Errorf("GetData()[\"k\"] = %v, want %q", data["k"], "v")
	}
}

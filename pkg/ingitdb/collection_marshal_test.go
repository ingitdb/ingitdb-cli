package ingitdb

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestCollectionDef_MarshalYAML_HonorsColumnsOrder(t *testing.T) {
	t.Parallel()
	def := &CollectionDef{
		Columns: map[string]*ColumnDef{
			"id":    {Type: ColumnTypeString},
			"email": {Type: ColumnTypeString},
			"name":  {Type: ColumnTypeString},
		},
		ColumnsOrder: []string{"email", "id", "name"},
	}
	out, err := yaml.Marshal(def)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(out)
	emailIdx := strings.Index(got, "email:")
	idIdx := strings.Index(got, "id:")
	nameIdx := strings.Index(got, "name:")
	if emailIdx >= idIdx || idIdx >= nameIdx {
		t.Errorf("expected columns_order [email, id, name]; got:\n%s", got)
	}
}

func TestCollectionDef_MarshalYAML_AlphabeticalFallback(t *testing.T) {
	t.Parallel()
	def := &CollectionDef{
		Columns: map[string]*ColumnDef{
			"name":  {Type: ColumnTypeString},
			"email": {Type: ColumnTypeString},
			"id":    {Type: ColumnTypeString},
		},
	}
	out, err := yaml.Marshal(def)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(out)
	emailIdx := strings.Index(got, "email:")
	idIdx := strings.Index(got, "id:")
	nameIdx := strings.Index(got, "name:")
	if emailIdx >= idIdx || idIdx >= nameIdx {
		t.Errorf("expected alphabetical fallback (email, id, name); got:\n%s", got)
	}
}

func TestCollectionDef_MarshalYAML_DeterministicAcrossRuns(t *testing.T) {
	t.Parallel()
	def := &CollectionDef{
		RecordFile: &RecordFileDef{Name: "{key}.yaml", Format: "yaml", RecordType: SingleRecord},
		Columns: map[string]*ColumnDef{
			"a": {Type: ColumnTypeString},
			"b": {Type: ColumnTypeString},
			"c": {Type: ColumnTypeString},
		},
	}
	first, err := yaml.Marshal(def)
	if err != nil {
		t.Fatalf("marshal 1: %v", err)
	}
	for i := 0; i < 50; i++ {
		next, err := yaml.Marshal(def)
		if err != nil {
			t.Fatalf("marshal iter %d: %v", i, err)
		}
		if string(next) != string(first) {
			t.Fatalf("non-deterministic output at iter %d", i)
		}
	}
}

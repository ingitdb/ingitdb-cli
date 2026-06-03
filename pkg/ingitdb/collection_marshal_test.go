package ingitdb

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestCollectionDef_MarshalYAML_NilReceiver(t *testing.T) {
	t.Parallel()

	// yaml.Marshal skips MarshalYAML for nil pointers, so call directly.
	var c *CollectionDef
	result, err := c.MarshalYAML()
	if err != nil {
		t.Fatalf("MarshalYAML on nil receiver: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result from nil receiver, got %v", result)
	}
}

func TestCollectionDef_MarshalYAML_WithTitles(t *testing.T) {
	t.Parallel()

	def := &CollectionDef{
		Titles: map[string]string{
			"en": "My Collection",
		},
		Columns: map[string]*ColumnDef{
			"id": {Type: ColumnTypeString},
		},
	}
	out, err := yaml.Marshal(def)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(out)
	if !strings.Contains(got, "titles") {
		t.Errorf("expected 'titles' key in output, got:\n%s", got)
	}
	if !strings.Contains(got, "My Collection") {
		t.Errorf("expected title value in output, got:\n%s", got)
	}
}

func TestCollectionDef_MarshalYAML_WithPrimaryKey(t *testing.T) {
	t.Parallel()

	def := &CollectionDef{
		PrimaryKey: []string{"id", "tenant"},
		Columns: map[string]*ColumnDef{
			"id": {Type: ColumnTypeString},
		},
	}
	out, err := yaml.Marshal(def)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(out)
	if !strings.Contains(got, "primary_key") {
		t.Errorf("expected 'primary_key' key in output, got:\n%s", got)
	}
	if !strings.Contains(got, "tenant") {
		t.Errorf("expected 'tenant' in primary_key output, got:\n%s", got)
	}
}

func TestCollectionDef_MarshalYAML_WithDefaultView(t *testing.T) {
	t.Parallel()

	def := &CollectionDef{
		DefaultView: &ViewDef{
			OrderBy: "name",
		},
		Columns: map[string]*ColumnDef{
			"name": {Type: ColumnTypeString},
		},
	}
	out, err := yaml.Marshal(def)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(out)
	if !strings.Contains(got, "default_view") {
		t.Errorf("expected 'default_view' key in output, got:\n%s", got)
	}
}

func TestCollectionDef_MarshalYAML_WithReadme(t *testing.T) {
	t.Parallel()

	def := &CollectionDef{
		Readme: &CollectionReadmeDef{
			HideColumns: true,
		},
		Columns: map[string]*ColumnDef{
			"name": {Type: ColumnTypeString},
		},
	}
	out, err := yaml.Marshal(def)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(out)
	if !strings.Contains(got, "readme") {
		t.Errorf("expected 'readme' key in output, got:\n%s", got)
	}
}

func TestOrderedColumnsNode_SkipsUnknownColumnsOrderKeys(t *testing.T) {
	t.Parallel()

	// columnsOrder references "ghost" which does not exist in columns.
	// orderedColumnsNode must skip it rather than panicking.
	columns := map[string]*ColumnDef{
		"id":   {Type: ColumnTypeString},
		"name": {Type: ColumnTypeString},
	}
	columnsOrder := []string{"ghost", "id", "name"}

	node := orderedColumnsNode(columns, columnsOrder)
	if node == nil {
		t.Fatal("expected non-nil node")
		return
	}
	// The node should contain exactly 2 keys (id, name), not 3.
	// Each key-value pair occupies 2 Content entries.
	wantEntries := 4 // 2 columns * 2 (key node + value node)
	if len(node.Content) != wantEntries {
		t.Errorf("expected %d Content entries, got %d", wantEntries, len(node.Content))
	}
	// "id" should appear before "name" (columnsOrder dictates this).
	firstKey := node.Content[0].Value
	if firstKey != "id" {
		t.Errorf("expected first key to be %q, got %q", "id", firstKey)
	}
}

func TestOrderedColumnsNode_EmptyColumns(t *testing.T) {
	t.Parallel()

	// When columns is empty, orderedColumnsNode must return nil.
	node := orderedColumnsNode(map[string]*ColumnDef{}, nil)
	if node != nil {
		t.Errorf("expected nil node for empty columns, got %v", node)
	}
}

func TestCollectionDef_MarshalYAML_WithDataDir(t *testing.T) {
	t.Parallel()

	// DataDir is non-empty: exercises the addScalar append path that was
	// previously skipped because all other tests left DataDir empty.
	def := &CollectionDef{
		DataDir: "data",
		Columns: map[string]*ColumnDef{
			"id": {Type: ColumnTypeString},
		},
	}
	out, err := yaml.Marshal(def)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(out)
	if !strings.Contains(got, "data_dir") {
		t.Errorf("expected 'data_dir' key in output, got:\n%s", got)
	}
	if !strings.Contains(got, "data") {
		t.Errorf("expected 'data' value in output, got:\n%s", got)
	}
}

func TestCollectionDef_MarshalYAML_NoColumns(t *testing.T) {
	t.Parallel()

	// Columns is nil: orderedColumnsNode returns nil, so addNode is called
	// with a nil node and must take its early-return path.
	def := &CollectionDef{
		DataDir: "data",
	}
	result, err := def.MarshalYAML()
	if err != nil {
		t.Fatalf("MarshalYAML: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	// The output must not contain a "columns" key.
	out, err := yaml.Marshal(result)
	if err != nil {
		t.Fatalf("yaml.Marshal of result: %v", err)
	}
	got := string(out)
	if strings.Contains(got, "columns:") {
		t.Errorf("expected no 'columns' key for nil Columns, got:\n%s", got)
	}
}

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
	for i := range 50 {
		next, err := yaml.Marshal(def)
		if err != nil {
			t.Fatalf("marshal iter %d: %v", i, err)
		}
		if string(next) != string(first) {
			t.Fatalf("non-deterministic output at iter %d", i)
		}
	}
}

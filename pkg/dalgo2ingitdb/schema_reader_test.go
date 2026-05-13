package dalgo2ingitdb

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dal-go/dalgo/dal"
	"github.com/dal-go/dalgo/dbschema"
)

// writeCollectionDef creates <root>/<name>/.collection/definition.yaml
// containing yamlContent. The intermediate directories are created.
func writeCollectionDef(t *testing.T, root, name, yamlContent string) {
	t.Helper()
	colDir := filepath.Join(root, name, ".collection")
	if err := os.MkdirAll(colDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", colDir, err)
	}
	defPath := filepath.Join(colDir, "definition.yaml")
	if err := os.WriteFile(defPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("write %s: %v", defPath, err)
	}
}

const countriesDef = `record_file:
  name: "{key}.yaml"
  format: yaml
  type: "map[string]any"
columns_order: [name, population]
columns:
  name:
    type: string
    required: true
  population:
    type: int
`

func TestListCollections_FindsCollectionDirs(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeCollectionDef(t, root, "countries", countriesDef)
	writeCollectionDef(t, root, "tags", countriesDef)
	writeCollectionDef(t, root, "todo/tasks", countriesDef)
	// A directory without .collection/definition.yaml — must NOT appear.
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}

	db, err := NewDatabase(root, newReader())
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	reader := db.(dbschema.SchemaReader)
	refs, err := reader.ListCollections(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListCollections: %v", err)
	}
	names := make([]string, len(refs))
	for i, r := range refs {
		names[i] = r.Name()
	}
	want := []string{"countries", "tags", "todo/tasks"}
	if !equalStrings(names, want) {
		t.Errorf("ListCollections names: got %v, want %v", names, want)
	}
}

func TestListCollections_ParentArgumentIgnored(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeCollectionDef(t, root, "tags", countriesDef)
	db, err := NewDatabase(root, newReader())
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	reader := db.(dbschema.SchemaReader)

	withNil, _ := reader.ListCollections(context.Background(), nil)
	withKey, _ := reader.ListCollections(context.Background(), dal.NewKeyWithID("ignored", "x"))
	if len(withNil) != len(withKey) {
		t.Errorf("ListCollections should ignore parent: nil=%d, key=%d", len(withNil), len(withKey))
	}
}

func TestDescribeCollection_MapsColumns(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeCollectionDef(t, root, "countries", countriesDef)
	db, err := NewDatabase(root, newReader())
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	reader := db.(dbschema.SchemaReader)

	ref := dal.NewRootCollectionRef("countries", "")
	got, err := reader.DescribeCollection(context.Background(), &ref)
	if err != nil {
		t.Fatalf("DescribeCollection: %v", err)
	}
	if got.Name != "countries" {
		t.Errorf("Name: got %q, want %q", got.Name, "countries")
	}
	if len(got.PrimaryKey) != 1 || got.PrimaryKey[0] != pkFieldName {
		t.Errorf("PrimaryKey: got %v, want [%q]", got.PrimaryKey, pkFieldName)
	}
	if len(got.Fields) != 2 {
		t.Fatalf("Fields: got %d, want 2", len(got.Fields))
	}
	if got.Fields[0].Name != "name" || got.Fields[0].Type != dbschema.String || got.Fields[0].Nullable {
		t.Errorf("Field[0]: got %+v, want {name, String, !Nullable}", got.Fields[0])
	}
	if got.Fields[1].Name != "population" || got.Fields[1].Type != dbschema.Int || !got.Fields[1].Nullable {
		t.Errorf("Field[1]: got %+v, want {population, Int, Nullable}", got.Fields[1])
	}
	if got.Indexes == nil || len(got.Indexes) != 0 {
		t.Errorf("Indexes: got %v, want empty non-nil slice", got.Indexes)
	}
}

func TestDescribeCollection_NotFound(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	db, err := NewDatabase(root, newReader())
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	reader := db.(dbschema.SchemaReader)
	ref := dal.NewRootCollectionRef("nonexistent", "")
	_, err = reader.DescribeCollection(context.Background(), &ref)
	if err == nil {
		t.Fatal("DescribeCollection(missing): want error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "not found") || !strings.Contains(msg, "nonexistent") {
		t.Errorf("error message should contain %q and %q, got: %s", "not found", "nonexistent", msg)
	}
}

func TestDescribeCollection_OmitsSyntheticPKFromFields(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	defWithKey := `record_file:
  name: "{key}.yaml"
  format: yaml
  type: "map[string]any"
columns_order: ["$key", name]
columns:
  $key:
    type: string
  name:
    type: string
`
	writeCollectionDef(t, root, "items", defWithKey)
	db, _ := NewDatabase(root, newReader())
	reader := db.(dbschema.SchemaReader)
	ref := dal.NewRootCollectionRef("items", "")
	got, err := reader.DescribeCollection(context.Background(), &ref)
	if err != nil {
		t.Fatalf("DescribeCollection: %v", err)
	}
	for _, f := range got.Fields {
		if f.Name == pkFieldName {
			t.Errorf("Fields should not contain synthesized PK %q: got %v", pkFieldName, got.Fields)
		}
	}
}

func TestListIndexes_ReturnsEmpty(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeCollectionDef(t, root, "tags", countriesDef)
	db, _ := NewDatabase(root, newReader())
	reader := db.(dbschema.SchemaReader)
	ref := dal.NewRootCollectionRef("tags", "")
	got, err := reader.ListIndexes(context.Background(), &ref)
	if err != nil {
		t.Errorf("ListIndexes err: %v", err)
	}
	if got == nil {
		t.Error("ListIndexes: want non-nil slice")
	}
	if len(got) != 0 {
		t.Errorf("ListIndexes: want empty, got %v", got)
	}
}

func TestListConstraints_ReturnsPK(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeCollectionDef(t, root, "tags", countriesDef)
	db, _ := NewDatabase(root, newReader())
	reader := db.(dbschema.SchemaReader)
	ref := dal.NewRootCollectionRef("tags", "")
	got, err := reader.ListConstraints(context.Background(), &ref)
	if err != nil {
		t.Errorf("ListConstraints err: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("ListConstraints: want 1 element, got %d", len(got))
	}
	if got[0].Type != "primary-key" {
		t.Errorf("ConstraintDef.Type: got %q, want %q", got[0].Type, "primary-key")
	}
}

func TestListReferrers_NotSupported(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeCollectionDef(t, root, "tags", countriesDef)
	db, _ := NewDatabase(root, newReader())
	reader := db.(dbschema.SchemaReader)
	ref := dal.NewRootCollectionRef("tags", "")
	got, err := reader.ListReferrers(context.Background(), &ref)
	if got != nil {
		t.Errorf("ListReferrers: want nil slice, got %v", got)
	}
	var nse *dbschema.NotSupportedError
	if !errors.As(err, &nse) {
		t.Fatalf("ListReferrers err: want *dbschema.NotSupportedError, got %T: %v", err, err)
	}
	if nse.Op != "ListReferrers" {
		t.Errorf("NotSupportedError.Op: got %q, want %q", nse.Op, "ListReferrers")
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

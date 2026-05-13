package dalgo2ingitdb

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dal-go/dalgo/dal"
	"github.com/dal-go/dalgo/dbschema"
	"github.com/dal-go/dalgo/ddl"
	"gopkg.in/yaml.v3"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func tagsCollectionDef() dbschema.CollectionDef {
	return dbschema.CollectionDef{
		Name: "tags",
		Fields: []dbschema.FieldDef{
			{Name: "label", Type: dbschema.String, Nullable: false},
			{Name: "color", Type: dbschema.String, Nullable: true},
		},
	}
}

func TestCreateCollection_WritesDefinitionYAML(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	db, _ := NewDatabase(root, newReader())
	modifier := db.(ddl.SchemaModifier)

	if err := modifier.CreateCollection(context.Background(), tagsCollectionDef()); err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	defPath := filepath.Join(root, "tags", ".collection", "definition.yaml")
	content, err := os.ReadFile(defPath)
	if err != nil {
		t.Fatalf("read %s: %v", defPath, err)
	}
	var loaded ingitdb.CollectionDef
	if err := yaml.Unmarshal(content, &loaded); err != nil {
		t.Fatalf("parse %s: %v", defPath, err)
	}
	if len(loaded.Columns) != 2 {
		t.Errorf("Columns: got %d, want 2", len(loaded.Columns))
	}
	if loaded.Columns["label"].Required != true {
		t.Errorf("Columns[label].Required: got %v, want true", loaded.Columns["label"].Required)
	}
	if loaded.Columns["color"].Required != false {
		t.Errorf("Columns[color].Required: got %v, want false", loaded.Columns["color"].Required)
	}
	wantOrder := []string{"label", "color"}
	if !equalStrings(loaded.ColumnsOrder, wantOrder) {
		t.Errorf("ColumnsOrder: got %v, want %v", loaded.ColumnsOrder, wantOrder)
	}
}

func TestCreateCollection_IfNotExists(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	db, _ := NewDatabase(root, newReader())
	modifier := db.(ddl.SchemaModifier)
	c := tagsCollectionDef()
	if err := modifier.CreateCollection(context.Background(), c); err != nil {
		t.Fatalf("first CreateCollection: %v", err)
	}
	defPath := filepath.Join(root, "tags", ".collection", "definition.yaml")
	before, _ := os.ReadFile(defPath)

	if err := modifier.CreateCollection(context.Background(), c, ddl.IfNotExists()); err != nil {
		t.Fatalf("CreateCollection IfNotExists: %v", err)
	}
	after, _ := os.ReadFile(defPath)
	if string(before) != string(after) {
		t.Error("CreateCollection IfNotExists must not modify existing definition.yaml")
	}

	// Without IfNotExists, second call must error.
	if err := modifier.CreateCollection(context.Background(), c); err == nil {
		t.Error("second CreateCollection without IfNotExists: want error")
	}
}

func TestCreateCollection_RejectsNullType(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	db, _ := NewDatabase(root, newReader())
	modifier := db.(ddl.SchemaModifier)
	bad := dbschema.CollectionDef{
		Name: "bad",
		Fields: []dbschema.FieldDef{
			{Name: "x", Type: dbschema.Null},
		},
	}
	if err := modifier.CreateCollection(context.Background(), bad); err == nil {
		t.Fatal("CreateCollection with Null field: want error")
	}
	// No directory should have been created.
	if _, err := os.Stat(filepath.Join(root, "bad")); !os.IsNotExist(err) {
		t.Error("CreateCollection must not write to disk when validation fails")
	}
}

func TestCreateCollection_RejectsBadName(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	db, _ := NewDatabase(root, newReader())
	modifier := db.(ddl.SchemaModifier)
	cases := []string{"", "..", "../escape", " name", "/abs"}
	for _, n := range cases {
		c := dbschema.CollectionDef{Name: n, Fields: []dbschema.FieldDef{{Name: "x", Type: dbschema.String}}}
		if err := modifier.CreateCollection(context.Background(), c); err == nil {
			t.Errorf("CreateCollection(%q): want error", n)
		}
	}
}

func TestDropCollection_RemovesDir(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	db, _ := NewDatabase(root, newReader())
	modifier := db.(ddl.SchemaModifier)
	if err := modifier.CreateCollection(context.Background(), tagsCollectionDef()); err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	if err := modifier.DropCollection(context.Background(), "tags"); err != nil {
		t.Fatalf("DropCollection: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "tags")); !os.IsNotExist(err) {
		t.Error("DropCollection should remove the collection dir")
	}
}

func TestDropCollection_IfExists(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	db, _ := NewDatabase(root, newReader())
	modifier := db.(ddl.SchemaModifier)
	// Missing + IfExists: nil error.
	if err := modifier.DropCollection(context.Background(), "missing", ddl.IfExists()); err != nil {
		t.Errorf("DropCollection IfExists on missing: got %v, want nil", err)
	}
	// Missing without IfExists: error.
	if err := modifier.DropCollection(context.Background(), "missing"); err == nil {
		t.Error("DropCollection on missing without IfExists: want error")
	}
}

func TestDropCollection_RefusesNonCollectionDir(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	docs := filepath.Join(root, "docs")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	db, _ := NewDatabase(root, newReader())
	modifier := db.(ddl.SchemaModifier)
	if err := modifier.DropCollection(context.Background(), "docs"); err == nil {
		t.Error("DropCollection on non-collection dir: want error")
	}
	if _, err := os.Stat(docs); err != nil {
		t.Error("DropCollection must not remove non-collection dir")
	}
}

func TestAlterCollection_AddField(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	db, _ := NewDatabase(root, newReader())
	modifier := db.(ddl.SchemaModifier)
	reader := db.(dbschema.SchemaReader)

	tags := dbschema.CollectionDef{
		Name:   "tags",
		Fields: []dbschema.FieldDef{{Name: "label", Type: dbschema.String, Nullable: false}},
	}
	if err := modifier.CreateCollection(context.Background(), tags); err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	op := ddl.AddField(dbschema.FieldDef{Name: "color", Type: dbschema.String, Nullable: true})
	if err := modifier.AlterCollection(context.Background(), "tags", op); err != nil {
		t.Fatalf("AlterCollection AddField: %v", err)
	}
	ref := dal.NewRootCollectionRef("tags", "")
	got, err := reader.DescribeCollection(context.Background(), &ref)
	if err != nil {
		t.Fatalf("DescribeCollection: %v", err)
	}
	if len(got.Fields) != 2 || got.Fields[1].Name != "color" {
		t.Errorf("after AddField: got fields %v, want [label, color]", got.Fields)
	}
}

func TestAlterCollection_DropField(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	db, _ := NewDatabase(root, newReader())
	modifier := db.(ddl.SchemaModifier)
	reader := db.(dbschema.SchemaReader)

	if err := modifier.CreateCollection(context.Background(), tagsCollectionDef()); err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	// Seed a record file containing both fields.
	recPath := filepath.Join(root, "tags", "$records")
	if err := os.MkdirAll(recPath, 0o755); err != nil {
		t.Fatalf("mkdir $records: %v", err)
	}
	rec := filepath.Join(recPath, "rust.yaml")
	if err := os.WriteFile(rec, []byte("label: rust\ncolor: orange\n"), 0o644); err != nil {
		t.Fatalf("seed record: %v", err)
	}

	if err := modifier.AlterCollection(context.Background(), "tags", ddl.DropField("color")); err != nil {
		t.Fatalf("AlterCollection DropField: %v", err)
	}
	ref := dal.NewRootCollectionRef("tags", "")
	got, _ := reader.DescribeCollection(context.Background(), &ref)
	for _, f := range got.Fields {
		if f.Name == "color" {
			t.Error("DropField: color should be absent in DescribeCollection")
		}
	}
	content, _ := os.ReadFile(rec)
	var parsed map[string]any
	_ = yaml.Unmarshal(content, &parsed)
	if _, ok := parsed["color"]; ok {
		t.Errorf("DropField: record still has color: %v", parsed)
	}
}

func TestAlterCollection_RenameField(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	db, _ := NewDatabase(root, newReader())
	modifier := db.(ddl.SchemaModifier)
	reader := db.(dbschema.SchemaReader)

	tags := dbschema.CollectionDef{
		Name:   "tags",
		Fields: []dbschema.FieldDef{{Name: "label", Type: dbschema.String}},
	}
	if err := modifier.CreateCollection(context.Background(), tags); err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	recPath := filepath.Join(root, "tags", "$records")
	if err := os.MkdirAll(recPath, 0o755); err != nil {
		t.Fatalf("mkdir $records: %v", err)
	}
	rec := filepath.Join(recPath, "rust.yaml")
	if err := os.WriteFile(rec, []byte("label: rust\n"), 0o644); err != nil {
		t.Fatalf("seed record: %v", err)
	}

	if err := modifier.AlterCollection(context.Background(), "tags", ddl.RenameField("label", "name")); err != nil {
		t.Fatalf("AlterCollection RenameField: %v", err)
	}
	ref := dal.NewRootCollectionRef("tags", "")
	got, _ := reader.DescribeCollection(context.Background(), &ref)
	if len(got.Fields) != 1 || got.Fields[0].Name != "name" {
		t.Errorf("after RenameField: got fields %v, want [name]", got.Fields)
	}
	content, _ := os.ReadFile(rec)
	var parsed map[string]any
	_ = yaml.Unmarshal(content, &parsed)
	if _, ok := parsed["label"]; ok {
		t.Errorf("RenameField: record still has label: %v", parsed)
	}
	if _, ok := parsed["name"]; !ok {
		t.Errorf("RenameField: record missing name: %v", parsed)
	}
}

func TestAlterCollection_PartialSuccessError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	db, _ := NewDatabase(root, newReader())
	modifier := db.(ddl.SchemaModifier)
	reader := db.(dbschema.SchemaReader)

	tags := dbschema.CollectionDef{
		Name:   "tags",
		Fields: []dbschema.FieldDef{{Name: "label", Type: dbschema.String}},
	}
	if err := modifier.CreateCollection(context.Background(), tags); err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}

	err := modifier.AlterCollection(context.Background(), "tags",
		ddl.AddField(dbschema.FieldDef{Name: "color", Type: dbschema.String}),
		ddl.DropField("nonexistent"),
	)
	if err == nil {
		t.Fatal("two-op alter should fail on second op")
	}
	var pse *ddl.PartialSuccessError
	if !errors.As(err, &pse) {
		t.Fatalf("want *ddl.PartialSuccessError, got %T: %v", err, err)
	}
	if len(pse.Applied) != 1 {
		t.Errorf("PartialSuccessError.Applied: got %d, want 1", len(pse.Applied))
	}
	// First op should have been applied.
	ref := dal.NewRootCollectionRef("tags", "")
	got, _ := reader.DescribeCollection(context.Background(), &ref)
	if len(got.Fields) != 2 {
		t.Errorf("after partial-success: want 2 fields, got %d", len(got.Fields))
	}
}

func TestAlterCollection_AddIndexLogsWarning(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	db, _ := NewDatabase(root, newReader())
	modifier := db.(ddl.SchemaModifier)
	if err := modifier.CreateCollection(context.Background(), tagsCollectionDef()); err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	idx := dbschema.IndexDef{Name: "label_idx", Fields: []dal.FieldName{"label"}}
	if err := modifier.AlterCollection(context.Background(), "tags", ddl.AddIndex(idx)); err != nil {
		t.Errorf("AlterCollection AddIndex should return nil, got %v", err)
	}
}

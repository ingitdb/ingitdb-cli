package dalgo2ingitdb

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/dal-go/dalgo/dal"
	"github.com/dal-go/dalgo/dbschema"
	"github.com/dal-go/dalgo/ddl"
)

// AC:create-describe-pk-roundtrip-single — a single-column source PK
// flows from dbschema.CollectionDef → definition.yaml → back to
// dbschema.CollectionDef losslessly. NOT synthesized as "$key".
func TestCreateDescribe_PKRoundTrip_Single(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	db, _ := NewDatabase(root, newReader())
	modifier := db.(ddl.SchemaModifier)
	reader := db.(dbschema.SchemaReader)

	c := dbschema.CollectionDef{
		Name: "Album",
		Fields: []dbschema.FieldDef{
			{Name: "AlbumId", Type: dbschema.Int, Nullable: false},
			{Name: "Title", Type: dbschema.String, Nullable: false},
		},
		PrimaryKey: []dal.FieldName{"AlbumId"},
	}
	if err := modifier.CreateCollection(context.Background(), c); err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}

	// On-disk: primary_key list must be present in YAML.
	defBytes, err := os.ReadFile(filepath.Join(root, "Album", ".collection", "definition.yaml"))
	if err != nil {
		t.Fatalf("read definition.yaml: %v", err)
	}
	if !slices.ContainsFunc([]string{"primary_key:", "AlbumId"}, func(s string) bool {
		return bytesContains(defBytes, s)
	}) {
		t.Errorf("definition.yaml missing primary_key serialization. Content:\n%s", defBytes)
	}

	// Round-trip through DescribeCollection.
	ref := dal.NewRootCollectionRef("Album", "")
	got, err := reader.DescribeCollection(context.Background(), &ref)
	if err != nil {
		t.Fatalf("DescribeCollection: %v", err)
	}
	if len(got.PrimaryKey) != 1 || got.PrimaryKey[0] != "AlbumId" {
		t.Errorf("PrimaryKey: got %v, want [AlbumId]", got.PrimaryKey)
	}
	if got.PrimaryKey[0] == pkFieldName {
		t.Errorf("PrimaryKey should NOT be the synthesized %q when source carries a real PK", pkFieldName)
	}
}

// AC:create-describe-pk-roundtrip-composite — composite source PK flows
// through preserving order.
func TestCreateDescribe_PKRoundTrip_Composite(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	db, _ := NewDatabase(root, newReader())
	modifier := db.(ddl.SchemaModifier)
	reader := db.(dbschema.SchemaReader)

	c := dbschema.CollectionDef{
		Name: "PlaylistTrack",
		Fields: []dbschema.FieldDef{
			{Name: "PlaylistId", Type: dbschema.Int, Nullable: false},
			{Name: "TrackId", Type: dbschema.Int, Nullable: false},
		},
		PrimaryKey: []dal.FieldName{"PlaylistId", "TrackId"},
	}
	if err := modifier.CreateCollection(context.Background(), c); err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}

	ref := dal.NewRootCollectionRef("PlaylistTrack", "")
	got, err := reader.DescribeCollection(context.Background(), &ref)
	if err != nil {
		t.Fatalf("DescribeCollection: %v", err)
	}
	want := []dal.FieldName{"PlaylistId", "TrackId"}
	if !slices.Equal(got.PrimaryKey, want) {
		t.Errorf("PrimaryKey: got %v, want %v", got.PrimaryKey, want)
	}
	// Both PK columns must still appear in Fields (they are real columns,
	// not the synthesized "$key" placeholder).
	var sawPlaylistId, sawTrackId bool
	for _, f := range got.Fields {
		if f.Name == "PlaylistId" {
			sawPlaylistId = true
		}
		if f.Name == "TrackId" {
			sawTrackId = true
		}
	}
	if !sawPlaylistId || !sawTrackId {
		t.Errorf("composite-PK columns missing from Fields: PlaylistId=%v TrackId=%v", sawPlaylistId, sawTrackId)
	}
}

// AC:describe-collection-legacy-no-pk-field — definition.yaml hand-written
// without a primary_key field (older projects) still works; DescribeCollection
// synthesizes ["$key"] for backward compatibility.
func TestDescribe_LegacyNoPKField(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// Write a definition.yaml by hand, omitting the primary_key field
	// entirely — simulates a project created before PK persistence shipped.
	colDir := filepath.Join(root, "legacy", ".collection")
	if err := os.MkdirAll(colDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", colDir, err)
	}
	legacyYAML := `record_file:
  name: '{key}.yaml'
  format: yaml
  type: map[string]any
columns:
  name:
    type: string
    required: true
columns_order:
  - name
`
	if err := os.WriteFile(filepath.Join(colDir, "definition.yaml"), []byte(legacyYAML), 0o644); err != nil {
		t.Fatalf("write definition.yaml: %v", err)
	}

	db, _ := NewDatabase(root, newReader())
	reader := db.(dbschema.SchemaReader)
	ref := dal.NewRootCollectionRef("legacy", "")
	got, err := reader.DescribeCollection(context.Background(), &ref)
	if err != nil {
		t.Fatalf("DescribeCollection: %v", err)
	}
	if len(got.PrimaryKey) != 1 || got.PrimaryKey[0] != pkFieldName {
		t.Errorf("PrimaryKey for legacy project: got %v, want [%s]", got.PrimaryKey, pkFieldName)
	}
}

// bytesContains is a tiny string-in-bytes helper to avoid pulling strings
// dependency just for one test substring check.
func bytesContains(b []byte, s string) bool {
	if len(s) == 0 {
		return true
	}
	if len(b) < len(s) {
		return false
	}
	for i := 0; i+len(s) <= len(b); i++ {
		if string(b[i:i+len(s)]) == s {
			return true
		}
	}
	return false
}

package dalgo2ingitdb

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dal-go/dalgo/dbschema"
	"github.com/dal-go/dalgo/ddl"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/config"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/validator"
)

// labelsCollectionDef returns a minimal dbschema.CollectionDef for a
// "labels" collection used by registry tests that need a second name
// alongside "tags".
func labelsCollectionDef() dbschema.CollectionDef {
	return dbschema.CollectionDef{
		Name: "labels",
		Fields: []dbschema.FieldDef{
			{Name: "name", Type: dbschema.String, Nullable: false},
		},
	}
}

// readRegistry returns the parsed contents of
// <root>/.ingitdb/root-collections.yaml or an empty map if missing.
func readRegistry(t *testing.T, root string) map[string]string {
	t.Helper()
	m, err := config.ReadRootCollectionsFromFile(root, ingitdb.NewReadOptions())
	if err != nil {
		t.Fatalf("ReadRootCollectionsFromFile: %v", err)
	}
	if m == nil {
		return map[string]string{}
	}
	return m
}

// AC:create-collection-registers-in-root-collections
func TestCreateCollection_RegistersInRootCollections_FreshProject(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	db, _ := NewDatabase(root, newReader())
	modifier := db.(ddl.SchemaModifier)

	if err := modifier.CreateCollection(context.Background(), tagsCollectionDef()); err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}

	regPath := filepath.Join(root, ".ingitdb", "root-collections.yaml")
	if _, err := os.Stat(regPath); err != nil {
		t.Fatalf("expected %s to exist: %v", regPath, err)
	}
	got := readRegistry(t, root)
	if len(got) != 1 || got["tags"] != "tags" {
		t.Errorf("registry: got %v, want {tags: tags}", got)
	}

	// validator-backed read must now see the collection.
	def, err := validator.NewCollectionsReader().ReadDefinition(root)
	if err != nil {
		t.Fatalf("ReadDefinition: %v", err)
	}
	if _, ok := def.Collections["tags"]; !ok {
		t.Errorf("ReadDefinition.Collections: missing %q (got keys %v)", "tags", mapKeys(def.Collections))
	}
}

// AC:create-collection-appends-to-existing-root-collections
func TestCreateCollection_AppendsToExistingRootCollections(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	db, _ := NewDatabase(root, newReader())
	modifier := db.(ddl.SchemaModifier)

	if err := modifier.CreateCollection(context.Background(), tagsCollectionDef()); err != nil {
		t.Fatalf("CreateCollection tags: %v", err)
	}
	if err := modifier.CreateCollection(context.Background(), labelsCollectionDef()); err != nil {
		t.Fatalf("CreateCollection labels: %v", err)
	}

	got := readRegistry(t, root)
	if len(got) != 2 || got["tags"] != "tags" || got["labels"] != "labels" {
		t.Errorf("registry: got %v, want {labels: labels, tags: tags}", got)
	}
}

// AC:create-collection-idempotent-registry-with-if-not-exists
func TestCreateCollection_IdempotentRegistryWithIfNotExists(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	db, _ := NewDatabase(root, newReader())
	modifier := db.(ddl.SchemaModifier)

	if err := modifier.CreateCollection(context.Background(), tagsCollectionDef()); err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	regPath := filepath.Join(root, ".ingitdb", "root-collections.yaml")
	contentBefore, err := os.ReadFile(regPath)
	if err != nil {
		t.Fatalf("read %s: %v", regPath, err)
	}

	if err := modifier.CreateCollection(context.Background(), tagsCollectionDef(), ddl.IfNotExists()); err != nil {
		t.Fatalf("re-CreateCollection with IfNotExists: %v", err)
	}
	contentAfter, err := os.ReadFile(regPath)
	if err != nil {
		t.Fatalf("read %s: %v", regPath, err)
	}
	if string(contentBefore) != string(contentAfter) {
		t.Errorf("registry file changed under idempotent IfNotExists:\nbefore: %q\nafter:  %q", contentBefore, contentAfter)
	}
}

// AC:create-collection-rejects-registry-conflict
func TestCreateCollection_RejectsRegistryConflict(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	// Pre-populate registry with a user-authored entry that maps `tags` to
	// a non-default path.
	if err := config.WriteRootCollectionsToFile(root, map[string]string{"tags": "legacy/tags-path"}); err != nil {
		t.Fatalf("seed registry: %v", err)
	}

	db, _ := NewDatabase(root, newReader())
	modifier := db.(ddl.SchemaModifier)

	err := modifier.CreateCollection(context.Background(), tagsCollectionDef())
	if err == nil {
		t.Fatal("CreateCollection should have errored on registry conflict")
	}
	if !errors.Is(err, ErrCollectionPathConflict) {
		t.Errorf("CreateCollection error: got %v, want wraps ErrCollectionPathConflict", err)
	}

	// Registry is unchanged.
	got := readRegistry(t, root)
	if got["tags"] != "legacy/tags-path" {
		t.Errorf("registry entry mutated: got %v, want {tags: legacy/tags-path}", got)
	}
}

// AC:drop-collection-deregisters-from-root-collections
func TestDropCollection_DeregistersFromRootCollections(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	db, _ := NewDatabase(root, newReader())
	modifier := db.(ddl.SchemaModifier)

	if err := modifier.CreateCollection(context.Background(), tagsCollectionDef()); err != nil {
		t.Fatalf("CreateCollection tags: %v", err)
	}
	if err := modifier.CreateCollection(context.Background(), labelsCollectionDef()); err != nil {
		t.Fatalf("CreateCollection labels: %v", err)
	}

	if err := modifier.DropCollection(context.Background(), "tags"); err != nil {
		t.Fatalf("DropCollection tags: %v", err)
	}

	got := readRegistry(t, root)
	if len(got) != 1 || got["labels"] != "labels" {
		t.Errorf("registry after drop: got %v, want {labels: labels}", got)
	}

	// validator-backed read must NOT see the dropped collection.
	def, err := validator.NewCollectionsReader().ReadDefinition(root)
	if err != nil {
		t.Fatalf("ReadDefinition: %v", err)
	}
	if _, ok := def.Collections["tags"]; ok {
		t.Errorf("ReadDefinition.Collections still contains dropped 'tags'")
	}
	if _, ok := def.Collections["labels"]; !ok {
		t.Errorf("ReadDefinition.Collections missing surviving 'labels': %v", mapKeys(def.Collections))
	}
}

// AC:drop-collection-tolerates-missing-registry — older projects created
// before auto-registration shipped have collection directories but no
// .ingitdb/root-collections.yaml. DropCollection must succeed without
// requiring the file to exist.
func TestDropCollection_TolerantOfMissingRegistry(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	db, _ := NewDatabase(root, newReader())
	modifier := db.(ddl.SchemaModifier)

	if err := modifier.CreateCollection(context.Background(), tagsCollectionDef()); err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}

	// Simulate an "old project": delete the registry file but leave the
	// collection directory in place.
	regPath := filepath.Join(root, ".ingitdb", "root-collections.yaml")
	if err := os.Remove(regPath); err != nil {
		t.Fatalf("remove %s: %v", regPath, err)
	}

	if err := modifier.DropCollection(context.Background(), "tags"); err != nil {
		t.Errorf("DropCollection should tolerate missing registry, got: %v", err)
	}
}

// mapKeys returns the keys of m as a slice for assertion error messages.
func mapKeys[K comparable, V any](m map[K]V) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

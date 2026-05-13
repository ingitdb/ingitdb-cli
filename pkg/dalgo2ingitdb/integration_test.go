package dalgo2ingitdb_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/dal-go/dalgo/dal"
	"github.com/dal-go/dalgo/dbschema"
	"github.com/dal-go/dalgo/ddl"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/validator"
)

// TestIntegration_FullLifecycle exercises NewDatabase →
// CreateCollection → ListCollections → DescribeCollection →
// AlterCollection(add) → AlterCollection(drop) → DropCollection
// → ListCollections against a real temp directory.
func TestIntegration_FullLifecycle(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	db, err := dalgo2ingitdb.NewDatabase(root, validator.NewCollectionsReader())
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	reader := db.(dbschema.SchemaReader)
	modifier := db.(ddl.SchemaModifier)

	events := dbschema.CollectionDef{
		Name: "events",
		Fields: []dbschema.FieldDef{
			{Name: "title", Type: dbschema.String, Nullable: false},
			{Name: "starts_at", Type: dbschema.Time, Nullable: true},
			{Name: "attendees", Type: dbschema.Int, Nullable: true},
			{Name: "is_public", Type: dbschema.Bool, Nullable: false},
		},
	}
	ctx := context.Background()

	if err := modifier.CreateCollection(ctx, events); err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}

	refs, err := reader.ListCollections(ctx, nil)
	if err != nil {
		t.Fatalf("ListCollections: %v", err)
	}
	if len(refs) != 1 || refs[0].Name() != "events" {
		t.Fatalf("ListCollections after create: got %v, want [events]", refs)
	}

	ref := dal.NewRootCollectionRef("events", "")
	got, err := reader.DescribeCollection(ctx, &ref)
	if err != nil {
		t.Fatalf("DescribeCollection: %v", err)
	}
	if got.Name != "events" {
		t.Errorf("Name: got %q, want events", got.Name)
	}
	if len(got.PrimaryKey) != 1 || got.PrimaryKey[0] != "$key" {
		t.Errorf("PrimaryKey: got %v, want [$key]", got.PrimaryKey)
	}
	wantTypes := []dbschema.Type{dbschema.String, dbschema.Time, dbschema.Int, dbschema.Bool}
	if len(got.Fields) != len(wantTypes) {
		t.Fatalf("Fields: got %d, want %d", len(got.Fields), len(wantTypes))
	}
	for i, want := range wantTypes {
		if got.Fields[i].Type != want {
			t.Errorf("Fields[%d].Type: got %s, want %s", i, got.Fields[i].Type, want)
		}
	}

	// Add a field, verify it appears.
	addColor := ddl.AddField(dbschema.FieldDef{Name: "color", Type: dbschema.String, Nullable: true})
	if err := modifier.AlterCollection(ctx, "events", addColor); err != nil {
		t.Fatalf("AlterCollection AddField: %v", err)
	}
	got, _ = reader.DescribeCollection(ctx, &ref)
	if len(got.Fields) != 5 {
		t.Errorf("after AddField: want 5 fields, got %d", len(got.Fields))
	}

	// Drop the new field.
	if err := modifier.AlterCollection(ctx, "events", ddl.DropField("color")); err != nil {
		t.Fatalf("AlterCollection DropField: %v", err)
	}
	got, _ = reader.DescribeCollection(ctx, &ref)
	if len(got.Fields) != 4 {
		t.Errorf("after DropField: want 4 fields, got %d", len(got.Fields))
	}

	// Drop the collection, verify it disappears from ListCollections.
	if err := modifier.DropCollection(ctx, "events"); err != nil {
		t.Fatalf("DropCollection: %v", err)
	}
	refs, _ = reader.ListCollections(ctx, nil)
	if len(refs) != 0 {
		t.Errorf("ListCollections after DropCollection: want empty, got %v", refs)
	}
}

// TestIntegration_ConcurrentReads verifies that multiple goroutines can
// call DescribeCollection on the same collection simultaneously and all
// return correct results. The shared lock taken by DescribeCollection
// must not serialize readers.
func TestIntegration_ConcurrentReads(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	db, err := dalgo2ingitdb.NewDatabase(root, validator.NewCollectionsReader())
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	if !db.SupportsConcurrentConnections() {
		t.Fatal("SupportsConcurrentConnections: want true")
	}
	reader := db.(dbschema.SchemaReader)
	modifier := db.(ddl.SchemaModifier)

	c := dbschema.CollectionDef{
		Name: "events",
		Fields: []dbschema.FieldDef{
			{Name: "title", Type: dbschema.String, Nullable: false},
		},
	}
	if err := modifier.CreateCollection(context.Background(), c); err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}

	const N = 10
	var ok int32
	var wg sync.WaitGroup
	for range N {
		wg.Go(func() {
			ref := dal.NewRootCollectionRef("events", "")
			got, err := reader.DescribeCollection(context.Background(), &ref)
			if err != nil {
				t.Errorf("concurrent DescribeCollection: %v", err)
				return
			}
			if got.Name != "events" || len(got.Fields) != 1 {
				t.Errorf("concurrent DescribeCollection: bad result %+v", got)
				return
			}
			atomic.AddInt32(&ok, 1)
		})
	}
	wg.Wait()
	if atomic.LoadInt32(&ok) != N {
		t.Errorf("concurrent reads: got %d/%d successes", atomic.LoadInt32(&ok), N)
	}
}

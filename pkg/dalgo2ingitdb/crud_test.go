package dalgo2ingitdb_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dal-go/dalgo/dal"
	"github.com/dal-go/dalgo/dbschema"
	"github.com/dal-go/dalgo/ddl"
	"github.com/dal-go/dalgo/update"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/validator"
)

// setupSingleRecordDB creates a fresh temp project with a single
// SingleRecord/YAML collection named "countries" and returns the
// dal.DB plus the project root. The collection is registered in
// .ingitdb/root-collections.yaml so the validator-backed CollectionsReader
// will find it.
func setupSingleRecordDB(t *testing.T) (dal.DB, string) {
	t.Helper()
	root := t.TempDir()
	db, err := dalgo2ingitdb.NewDatabase(root, validator.NewCollectionsReader())
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	modifier := db.(ddl.SchemaModifier)
	col := dbschema.CollectionDef{
		Name: "countries",
		Fields: []dbschema.FieldDef{
			{Name: "name", Type: dbschema.String, Nullable: false},
			{Name: "population", Type: dbschema.Int, Nullable: true},
		},
	}
	if err := modifier.CreateCollection(context.Background(), col); err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	registerRootCollection(t, root, "countries", "countries")
	return db, root
}

// registerRootCollection writes a minimal .ingitdb/root-collections.yaml
// mapping `id` to `path` so validator.ReadDefinition discovers the
// collection. CreateCollection only writes the schema file; root
// registration is a separate concern.
func registerRootCollection(t *testing.T, root, id, path string) {
	t.Helper()
	dir := filepath.Join(root, ".ingitdb")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	content := id + ": " + path + "\n"
	file := filepath.Join(dir, "root-collections.yaml")
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", file, err)
	}
}

// writeYAMLRecord writes a YAML record directly to disk for tests that
// want pre-existing records before exercising the driver.
func writeYAMLRecord(t *testing.T, root, collection, key, content string) {
	t.Helper()
	dir := filepath.Join(root, collection, "$records")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	path := filepath.Join(dir, key+".yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestDB_Get_RoundTripsWrittenFile(t *testing.T) {
	t.Parallel()
	db, root := setupSingleRecordDB(t)
	writeYAMLRecord(t, root, "countries", "france", "name: France\npopulation: 67000000\n")

	rec := dal.NewRecordWithData(dal.NewKeyWithID("countries", "france"), map[string]any{})
	if err := db.Get(context.Background(), rec); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if err := rec.Error(); err != nil {
		t.Fatalf("rec.Error: %v", err)
	}
	data := rec.Data().(map[string]any)
	if data["name"] != "France" {
		t.Errorf("name: got %v, want France", data["name"])
	}
	if data["population"] != 67000000 {
		t.Errorf("population: got %v (%T), want 67000000", data["population"], data["population"])
	}
}

func TestDB_Get_MissingRecordMarksNotFound(t *testing.T) {
	t.Parallel()
	db, _ := setupSingleRecordDB(t)
	rec := dal.NewRecordWithData(dal.NewKeyWithID("countries", "atlantis"), map[string]any{})
	if err := db.Get(context.Background(), rec); err != nil {
		t.Fatalf("Get: %v", err)
	}
	// dalgo's record.Exists() returns false for a missing record after
	// the driver has set ErrRecordNotFound via SetError. record.Error()
	// deliberately swallows that sentinel.
	if rec.Exists() {
		t.Errorf("rec.Exists: got true, want false for missing record")
	}
}

func TestDB_Exists_HitAndMiss(t *testing.T) {
	t.Parallel()
	db, root := setupSingleRecordDB(t)
	writeYAMLRecord(t, root, "countries", "france", "name: France\n")
	ctx := context.Background()

	got, err := db.Exists(ctx, dal.NewKeyWithID("countries", "france"))
	if err != nil {
		t.Fatalf("Exists hit: %v", err)
	}
	if !got {
		t.Errorf("Exists(france): got false, want true")
	}

	got, err = db.Exists(ctx, dal.NewKeyWithID("countries", "atlantis"))
	if err != nil {
		t.Fatalf("Exists miss: %v", err)
	}
	if got {
		t.Errorf("Exists(atlantis): got true, want false")
	}
}

func TestDB_GetMulti_LoadsEach(t *testing.T) {
	t.Parallel()
	db, root := setupSingleRecordDB(t)
	writeYAMLRecord(t, root, "countries", "france", "name: France\n")
	writeYAMLRecord(t, root, "countries", "japan", "name: Japan\n")

	r1 := dal.NewRecordWithData(dal.NewKeyWithID("countries", "france"), map[string]any{})
	r2 := dal.NewRecordWithData(dal.NewKeyWithID("countries", "japan"), map[string]any{})
	r3 := dal.NewRecordWithData(dal.NewKeyWithID("countries", "atlantis"), map[string]any{})
	if err := db.GetMulti(context.Background(), []dal.Record{r1, r2, r3}); err != nil {
		t.Fatalf("GetMulti: %v", err)
	}
	if r1.Data().(map[string]any)["name"] != "France" {
		t.Errorf("r1 name: got %v, want France", r1.Data())
	}
	if r2.Data().(map[string]any)["name"] != "Japan" {
		t.Errorf("r2 name: got %v, want Japan", r2.Data())
	}
	if r3.Exists() {
		t.Errorf("r3 should not exist (missing record)")
	}
}

func TestDB_RunReadwriteTransaction_InsertSetDelete(t *testing.T) {
	t.Parallel()
	db, root := setupSingleRecordDB(t)
	ctx := context.Background()

	insertErr := db.RunReadwriteTransaction(ctx, func(_ context.Context, tx dal.ReadwriteTransaction) error {
		rec := dal.NewRecordWithData(
			dal.NewKeyWithID("countries", "france"),
			map[string]any{"name": "France", "population": 67000000},
		)
		return tx.Insert(ctx, rec)
	})
	if insertErr != nil {
		t.Fatalf("Insert: %v", insertErr)
	}

	// File should exist on disk now.
	if _, err := os.Stat(filepath.Join(root, "countries", "$records", "france.yaml")); err != nil {
		t.Fatalf("after Insert: stat: %v", err)
	}

	// Duplicate insert must fail.
	dupErr := db.RunReadwriteTransaction(ctx, func(_ context.Context, tx dal.ReadwriteTransaction) error {
		rec := dal.NewRecordWithData(
			dal.NewKeyWithID("countries", "france"),
			map[string]any{"name": "France"},
		)
		return tx.Insert(ctx, rec)
	})
	if dupErr == nil {
		t.Fatal("duplicate Insert: want error, got nil")
	}

	// Set overwrites existing record.
	setErr := db.RunReadwriteTransaction(ctx, func(_ context.Context, tx dal.ReadwriteTransaction) error {
		rec := dal.NewRecordWithData(
			dal.NewKeyWithID("countries", "france"),
			map[string]any{"name": "Republic of France", "population": 68000000},
		)
		return tx.Set(ctx, rec)
	})
	if setErr != nil {
		t.Fatalf("Set: %v", setErr)
	}

	got := dal.NewRecordWithData(dal.NewKeyWithID("countries", "france"), map[string]any{})
	if err := db.Get(ctx, got); err != nil {
		t.Fatalf("Get after Set: %v", err)
	}
	if got.Data().(map[string]any)["name"] != "Republic of France" {
		t.Errorf("Set did not overwrite: %v", got.Data())
	}

	// Delete removes the record.
	delErr := db.RunReadwriteTransaction(ctx, func(_ context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Delete(ctx, dal.NewKeyWithID("countries", "france"))
	})
	if delErr != nil {
		t.Fatalf("Delete: %v", delErr)
	}
	if _, err := os.Stat(filepath.Join(root, "countries", "$records", "france.yaml")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("after Delete: file should be gone, stat err = %v", err)
	}

	// Delete again must return ErrRecordNotFound.
	missErr := db.RunReadwriteTransaction(ctx, func(_ context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Delete(ctx, dal.NewKeyWithID("countries", "france"))
	})
	if !errors.Is(missErr, dal.ErrRecordNotFound) {
		t.Errorf("second Delete: got %v, want ErrRecordNotFound", missErr)
	}
}

func TestDB_RunReadonlyTransaction_GetExists(t *testing.T) {
	t.Parallel()
	db, root := setupSingleRecordDB(t)
	writeYAMLRecord(t, root, "countries", "france", "name: France\n")

	err := db.RunReadonlyTransaction(context.Background(), func(ctx context.Context, tx dal.ReadTransaction) error {
		rec := dal.NewRecordWithData(dal.NewKeyWithID("countries", "france"), map[string]any{})
		if err := tx.Get(ctx, rec); err != nil {
			return err
		}
		if rec.Data().(map[string]any)["name"] != "France" {
			t.Errorf("name: got %v, want France", rec.Data())
		}
		exists, err := tx.Exists(ctx, dal.NewKeyWithID("countries", "france"))
		if err != nil {
			return err
		}
		if !exists {
			t.Error("Exists: got false, want true")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("RunReadonlyTransaction: %v", err)
	}
}

func TestDB_RunReadwriteTransaction_UpdateAndUpdateRecord(t *testing.T) {
	t.Parallel()
	db, root := setupSingleRecordDB(t)
	writeYAMLRecord(t, root, "countries", "japan", "name: Japan\npopulation: 125000000\n")
	ctx := context.Background()

	err := db.RunReadwriteTransaction(ctx, func(_ context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Update(ctx, dal.NewKeyWithID("countries", "japan"),
			[]update.Update{update.ByFieldName("population", 124000000)})
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	got := dal.NewRecordWithData(dal.NewKeyWithID("countries", "japan"), map[string]any{})
	if err := db.Get(ctx, got); err != nil {
		t.Fatalf("Get after Update: %v", err)
	}
	if got.Data().(map[string]any)["population"] != 124000000 {
		t.Errorf("after Update: population = %v, want 124000000", got.Data())
	}
}

func TestDB_ExecuteQueryToRecordsReader_FiltersAndOrders(t *testing.T) {
	t.Parallel()
	db, root := setupSingleRecordDB(t)
	writeYAMLRecord(t, root, "countries", "france", "name: France\npopulation: 67000000\n")
	writeYAMLRecord(t, root, "countries", "japan", "name: Japan\npopulation: 125000000\n")
	writeYAMLRecord(t, root, "countries", "monaco", "name: Monaco\npopulation: 38000\n")

	q := dal.From(dal.NewRootCollectionRef("countries", "")).NewQuery().
		WhereField("population", dal.GreaterThen, 1000000).
		OrderBy(dal.AscendingField("name")).
		SelectIntoRecord(func() dal.Record {
			return dal.NewRecordWithData(dal.NewKeyWithID("countries", ""), map[string]any{})
		})

	reader, err := db.ExecuteQueryToRecordsReader(context.Background(), q)
	if err != nil {
		t.Fatalf("ExecuteQueryToRecordsReader: %v", err)
	}
	defer func() { _ = reader.Close() }()

	var names []string
	for {
		rec, nextErr := reader.Next()
		if errors.Is(nextErr, dal.ErrNoMoreRecords) {
			break
		}
		if nextErr != nil {
			t.Fatalf("reader.Next: %v", nextErr)
		}
		names = append(names, rec.Data().(map[string]any)["name"].(string))
	}
	want := []string{"France", "Japan"}
	if len(names) != len(want) {
		t.Fatalf("names: got %v, want %v", names, want)
	}
	for i, n := range want {
		if names[i] != n {
			t.Errorf("names[%d]: got %q, want %q", i, names[i], n)
		}
	}
}

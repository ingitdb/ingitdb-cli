package dalgo2fsingitdb

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dal-go/dalgo/dal"
	"gopkg.in/yaml.v3"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// makeTestDef builds a minimal Definition with one SingleRecord YAML collection
// rooted at dirPath.
func makeTestDef(t *testing.T, dirPath string) *ingitdb.Definition {
	t.Helper()
	colDef := &ingitdb.CollectionDef{
		ID:      "test.items",
		DirPath: dirPath,
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "{key}.yaml",
			Format:     "yaml",
			RecordType: ingitdb.SingleRecord,
		},
		Columns: map[string]*ingitdb.ColumnDef{
			"name": {Type: ingitdb.ColumnTypeString},
		},
	}
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"test.items": colDef,
		},
	}
	return def
}

func openTestDB(t *testing.T, dirPath string, def *ingitdb.Definition) dal.DB {
	t.Helper()
	db, err := NewLocalDBWithDef(dirPath, def)
	if err != nil {
		t.Fatalf("NewLocalDBWithDef: %v", err)
	}
	return db
}

func writeYAMLFixture(t *testing.T, path string, data map[string]any) {
	t.Helper()
	content, err := yaml.Marshal(data)
	if err != nil {
		t.Fatalf("yaml.Marshal: %v", err)
	}
	if err = os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("os.WriteFile: %v", err)
	}
}

func TestGet_SingleRecord_Found(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := makeTestDef(t, dir)
	recordsDir := filepath.Join(dir, "$records")
	if err := os.MkdirAll(recordsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	writeYAMLFixture(t, filepath.Join(recordsDir, "abc.yaml"), map[string]any{"name": "Alpha"})

	db := openTestDB(t, dir, def)
	ctx := context.Background()
	key := dal.NewKeyWithID("test.items", "abc")
	data := map[string]any{}
	record := dal.NewRecordWithData(key, data)

	err := db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		return tx.Get(ctx, record)
	})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !record.Exists() {
		t.Fatal("expected record to exist")
	}
	if data["name"] != "Alpha" {
		t.Fatalf("expected name=Alpha, got %v", data["name"])
	}
}

func TestGet_SingleRecord_NotFound(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := makeTestDef(t, dir)
	db := openTestDB(t, dir, def)
	ctx := context.Background()
	key := dal.NewKeyWithID("test.items", "missing")
	data := map[string]any{}
	record := dal.NewRecordWithData(key, data)

	err := db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		return tx.Get(ctx, record)
	})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if record.Exists() {
		t.Fatal("expected record to not exist")
	}
}

func TestInsert_SingleRecord(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := makeTestDef(t, dir)
	db := openTestDB(t, dir, def)
	ctx := context.Background()
	key := dal.NewKeyWithID("test.items", "new")
	data := map[string]any{"name": "New Item"}
	record := dal.NewRecordWithData(key, data)

	err := db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Insert(ctx, record)
	})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	content, readErr := os.ReadFile(filepath.Join(dir, "$records", "new.yaml"))
	if readErr != nil {
		t.Fatalf("ReadFile: %v", readErr)
	}
	var got map[string]any
	if err = yaml.Unmarshal(content, &got); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}
	if got["name"] != "New Item" {
		t.Fatalf("expected name=New Item, got %v", got["name"])
	}
}

func TestInsert_SingleRecord_KeySubdirPattern(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	colDef := &ingitdb.CollectionDef{
		ID:      "test.items",
		DirPath: dir,
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "{key}/{key}.yaml",
			Format:     "yaml",
			RecordType: ingitdb.SingleRecord,
		},
		Columns: map[string]*ingitdb.ColumnDef{
			"name": {Type: ingitdb.ColumnTypeString},
		},
	}
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{"test.items": colDef},
	}
	db := openTestDB(t, dir, def)
	ctx := context.Background()
	key := dal.NewKeyWithID("test.items", "de")
	data := map[string]any{"name": "Germany"}
	record := dal.NewRecordWithData(key, data)

	err := db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Insert(ctx, record)
	})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	content, readErr := os.ReadFile(filepath.Join(dir, "$records", "de", "de.yaml"))
	if readErr != nil {
		t.Fatalf("ReadFile: %v", readErr)
	}
	var got map[string]any
	if err = yaml.Unmarshal(content, &got); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}
	if got["name"] != "Germany" {
		t.Fatalf("expected name=Germany, got %v", got["name"])
	}
}

func TestInsert_SingleRecord_AlreadyExists(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := makeTestDef(t, dir)
	recordsDir := filepath.Join(dir, "$records")
	if err := os.MkdirAll(recordsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	writeYAMLFixture(t, filepath.Join(recordsDir, "dup.yaml"), map[string]any{"name": "Existing"})

	db := openTestDB(t, dir, def)
	ctx := context.Background()
	key := dal.NewKeyWithID("test.items", "dup")
	data := map[string]any{"name": "Duplicate"}
	record := dal.NewRecordWithData(key, data)

	err := db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Insert(ctx, record)
	})
	if err == nil {
		t.Fatal("expected error for duplicate insert, got nil")
	}
}

func TestSet_SingleRecord(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := makeTestDef(t, dir)
	db := openTestDB(t, dir, def)
	ctx := context.Background()
	key := dal.NewKeyWithID("test.items", "upsert")
	data := map[string]any{"name": "Upserted"}
	record := dal.NewRecordWithData(key, data)

	err := db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Set(ctx, record)
	})
	if err != nil {
		t.Fatalf("Set: %v", err)
	}

	content, readErr := os.ReadFile(filepath.Join(dir, "$records", "upsert.yaml"))
	if readErr != nil {
		t.Fatalf("ReadFile: %v", readErr)
	}
	var got map[string]any
	if err = yaml.Unmarshal(content, &got); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}
	if got["name"] != "Upserted" {
		t.Fatalf("expected name=Upserted, got %v", got["name"])
	}
}

func TestDelete_SingleRecord(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := makeTestDef(t, dir)
	recordsDir := filepath.Join(dir, "$records")
	if err := os.MkdirAll(recordsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	path := filepath.Join(recordsDir, "del.yaml")
	writeYAMLFixture(t, path, map[string]any{"name": "ToDelete"})

	db := openTestDB(t, dir, def)
	ctx := context.Background()
	key := dal.NewKeyWithID("test.items", "del")

	err := db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Delete(ctx, key)
	})
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatal("expected file to be deleted")
	}
}

func TestDelete_SingleRecord_NotFound(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := makeTestDef(t, dir)
	db := openTestDB(t, dir, def)
	ctx := context.Background()
	key := dal.NewKeyWithID("test.items", "ghost")

	err := db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Delete(ctx, key)
	})
	if !errors.Is(err, dal.ErrRecordNotFound) {
		t.Fatalf("expected ErrRecordNotFound, got %v", err)
	}
}

// makeMapOfIDDef builds a Definition with one MapOfRecords JSON collection.
func makeMapOfIDDef(t *testing.T, dirPath string) *ingitdb.Definition {
	t.Helper()
	colDef := &ingitdb.CollectionDef{
		ID:      "test.tags",
		DirPath: dirPath,
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "tags.json",
			Format:     "json",
			RecordType: ingitdb.MapOfRecords,
		},
		Columns: map[string]*ingitdb.ColumnDef{
			"title":  {Type: ingitdb.ColumnTypeString, Locale: "en"},
			"titles": {Type: ingitdb.ColumnTypeL10N, Required: true},
		},
	}
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"test.tags": colDef,
		},
	}
	return def
}

// writeJSONMapOfIDFixture writes a map[string]map[string]any as a JSON file.
func writeJSONMapOfIDFixture(t *testing.T, path string, data map[string]map[string]any) {
	t.Helper()
	content, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent: %v", err)
	}
	if err = os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("os.WriteFile: %v", err)
	}
}

func TestGet_MapOfRecords_Found(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := makeMapOfIDDef(t, dir)
	writeJSONMapOfIDFixture(t, filepath.Join(dir, "tags.json"), map[string]map[string]any{
		"work": {"titles": map[string]any{"en": "Work", "ru": "Работа"}},
		"home": {"titles": map[string]any{"en": "Home", "ru": "Дом"}},
	})

	db := openTestDB(t, dir, def)
	ctx := context.Background()
	key := dal.NewKeyWithID("test.tags", "work")
	data := map[string]any{}
	record := dal.NewRecordWithData(key, data)

	err := db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		return tx.Get(ctx, record)
	})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !record.Exists() {
		t.Fatal("expected record to exist")
	}
	if data["title"] != "Work" {
		t.Fatalf("expected title=Work (from locale shortcut), got %v", data["title"])
	}
	titles, ok := data["titles"].(map[string]any)
	if !ok {
		t.Fatalf("expected titles to be a map, got %T", data["titles"])
	}
	if _, hasEN := titles["en"]; hasEN {
		t.Fatal("expected 'en' to be removed from titles after locale read transform")
	}
	if titles["ru"] != "Работа" {
		t.Fatalf("expected titles.ru=Работа, got %v", titles["ru"])
	}
}

func TestGet_MapOfRecords_KeyNotFound(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := makeMapOfIDDef(t, dir)
	writeJSONMapOfIDFixture(t, filepath.Join(dir, "tags.json"), map[string]map[string]any{
		"work": {"titles": map[string]any{"en": "Work"}},
	})

	db := openTestDB(t, dir, def)
	ctx := context.Background()
	key := dal.NewKeyWithID("test.tags", "missing")
	data := map[string]any{}
	record := dal.NewRecordWithData(key, data)

	err := db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		return tx.Get(ctx, record)
	})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if record.Exists() {
		t.Fatal("expected record to not exist")
	}
}

func TestGet_MapOfRecords_FileNotFound(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := makeMapOfIDDef(t, dir)
	db := openTestDB(t, dir, def)
	ctx := context.Background()
	key := dal.NewKeyWithID("test.tags", "work")
	data := map[string]any{}
	record := dal.NewRecordWithData(key, data)

	err := db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		return tx.Get(ctx, record)
	})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if record.Exists() {
		t.Fatal("expected record to not exist when file is missing")
	}
}

func TestInsert_MapOfRecords(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := makeMapOfIDDef(t, dir)
	db := openTestDB(t, dir, def)
	ctx := context.Background()
	key := dal.NewKeyWithID("test.tags", "work")
	data := map[string]any{
		"title":  "Work",
		"titles": map[string]any{"ru": "Работа"},
	}
	record := dal.NewRecordWithData(key, data)

	err := db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Insert(ctx, record)
	})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	raw, readErr := os.ReadFile(filepath.Join(dir, "tags.json"))
	if readErr != nil {
		t.Fatalf("ReadFile: %v", readErr)
	}
	var got map[string]map[string]any
	if err = json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	work, ok := got["work"]
	if !ok {
		t.Fatal("expected 'work' key in tags.json")
	}
	if work["title"] != "Work" {
		t.Fatalf("expected title=Work stored directly in file, got %v", work["title"])
	}
	titles, ok := work["titles"].(map[string]any)
	if !ok {
		t.Fatalf("expected 'titles' to be a map, got %T", work["titles"])
	}
	if _, hasEN := titles["en"]; hasEN {
		t.Fatal("expected 'en' NOT stored in titles (it belongs in the shortcut column)")
	}
	if titles["ru"] != "Работа" {
		t.Fatalf("expected titles.ru=Работа preserved, got %v", titles["ru"])
	}
}

func TestInsert_MapOfRecords_TitleOnly(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := makeMapOfIDDef(t, dir)
	db := openTestDB(t, dir, def)
	ctx := context.Background()
	key := dal.NewKeyWithID("test.tags", "solo")
	data := map[string]any{"title": "Solo"}
	record := dal.NewRecordWithData(key, data)

	err := db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Insert(ctx, record)
	})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	raw, readErr := os.ReadFile(filepath.Join(dir, "tags.json"))
	if readErr != nil {
		t.Fatalf("ReadFile: %v", readErr)
	}
	var got map[string]map[string]any
	if err = json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	solo, ok := got["solo"]
	if !ok {
		t.Fatal("expected 'solo' key in tags.json")
	}
	if solo["title"] != "Solo" {
		t.Fatalf("expected title=Solo stored directly, got %v", solo["title"])
	}
	if _, hasTitles := solo["titles"]; hasTitles {
		t.Fatal("expected no 'titles' key when no other locales are provided")
	}
}

func TestInsert_MapOfRecords_PrimaryLocaleInTitlesNormalized(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := makeMapOfIDDef(t, dir)
	db := openTestDB(t, dir, def)
	ctx := context.Background()
	key := dal.NewKeyWithID("test.tags", "norm")
	// User supplies primary locale inside titles map — should be promoted to title.
	data := map[string]any{
		"titles": map[string]any{"en": "Norm", "ru": "Норм"},
	}
	record := dal.NewRecordWithData(key, data)

	err := db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Insert(ctx, record)
	})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	raw, readErr := os.ReadFile(filepath.Join(dir, "tags.json"))
	if readErr != nil {
		t.Fatalf("ReadFile: %v", readErr)
	}
	var got map[string]map[string]any
	if err = json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	norm, ok := got["norm"]
	if !ok {
		t.Fatal("expected 'norm' key in tags.json")
	}
	if norm["title"] != "Norm" {
		t.Fatalf("expected title=Norm promoted from titles.en, got %v", norm["title"])
	}
	titles, ok := norm["titles"].(map[string]any)
	if !ok {
		t.Fatalf("expected 'titles' to be a map, got %T", norm["titles"])
	}
	if _, hasEN := titles["en"]; hasEN {
		t.Fatal("expected 'en' removed from titles after promotion")
	}
	if titles["ru"] != "Норм" {
		t.Fatalf("expected titles.ru=Норм, got %v", titles["ru"])
	}
}

func TestInsert_MapOfRecords_AlreadyExists(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := makeMapOfIDDef(t, dir)
	writeJSONMapOfIDFixture(t, filepath.Join(dir, "tags.json"), map[string]map[string]any{
		"work": {"titles": map[string]any{"en": "Work"}},
	})

	db := openTestDB(t, dir, def)
	ctx := context.Background()
	key := dal.NewKeyWithID("test.tags", "work")
	data := map[string]any{"title": "Work Again"}
	record := dal.NewRecordWithData(key, data)

	err := db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Insert(ctx, record)
	})
	if err == nil {
		t.Fatal("expected error for duplicate key, got nil")
	}
}

func TestSet_MapOfRecords(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := makeMapOfIDDef(t, dir)
	writeJSONMapOfIDFixture(t, filepath.Join(dir, "tags.json"), map[string]map[string]any{
		"work": {"titles": map[string]any{"en": "Work", "ru": "Работа"}},
	})

	db := openTestDB(t, dir, def)
	ctx := context.Background()
	key := dal.NewKeyWithID("test.tags", "home")
	data := map[string]any{
		"title":  "Home",
		"titles": map[string]any{"ru": "Дом"},
	}
	record := dal.NewRecordWithData(key, data)

	err := db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Set(ctx, record)
	})
	if err != nil {
		t.Fatalf("Set: %v", err)
	}

	raw, readErr := os.ReadFile(filepath.Join(dir, "tags.json"))
	if readErr != nil {
		t.Fatalf("ReadFile: %v", readErr)
	}
	var got map[string]map[string]any
	if err = json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if _, ok := got["work"]; !ok {
		t.Fatal("expected existing 'work' key to be preserved")
	}
	home, ok := got["home"]
	if !ok {
		t.Fatal("expected 'home' key to be added")
	}
	if home["title"] != "Home" {
		t.Fatalf("expected title=Home stored directly, got %v", home["title"])
	}
	titles, ok := home["titles"].(map[string]any)
	if !ok {
		t.Fatalf("expected 'titles' to be a map, got %T", home["titles"])
	}
	if _, hasEN := titles["en"]; hasEN {
		t.Fatal("expected 'en' NOT in titles (value belongs in shortcut column)")
	}
	if titles["ru"] != "Дом" {
		t.Fatalf("expected titles.ru=Дом, got %v", titles["ru"])
	}
}

func TestDelete_MapOfRecords(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := makeMapOfIDDef(t, dir)
	writeJSONMapOfIDFixture(t, filepath.Join(dir, "tags.json"), map[string]map[string]any{
		"work": {"titles": map[string]any{"en": "Work"}},
		"home": {"titles": map[string]any{"en": "Home"}},
	})

	db := openTestDB(t, dir, def)
	ctx := context.Background()
	key := dal.NewKeyWithID("test.tags", "work")

	err := db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Delete(ctx, key)
	})
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	raw, readErr := os.ReadFile(filepath.Join(dir, "tags.json"))
	if readErr != nil {
		t.Fatalf("ReadFile: %v", readErr)
	}
	var got map[string]map[string]any
	if err = json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if _, ok := got["work"]; ok {
		t.Fatal("expected 'work' key to be deleted")
	}
	if _, ok := got["home"]; !ok {
		t.Fatal("expected 'home' key to be preserved")
	}
}

func TestDelete_MapOfRecords_KeyNotFound(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := makeMapOfIDDef(t, dir)
	writeJSONMapOfIDFixture(t, filepath.Join(dir, "tags.json"), map[string]map[string]any{
		"work": {"titles": map[string]any{"en": "Work"}},
	})

	db := openTestDB(t, dir, def)
	ctx := context.Background()
	key := dal.NewKeyWithID("test.tags", "ghost")

	err := db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Delete(ctx, key)
	})
	if !errors.Is(err, dal.ErrRecordNotFound) {
		t.Fatalf("expected ErrRecordNotFound, got %v", err)
	}
}

func TestDelete_MapOfRecords_FileNotFound(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := makeMapOfIDDef(t, dir)
	db := openTestDB(t, dir, def)
	ctx := context.Background()
	key := dal.NewKeyWithID("test.tags", "work")

	err := db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Delete(ctx, key)
	})
	if !errors.Is(err, dal.ErrRecordNotFound) {
		t.Fatalf("expected ErrRecordNotFound, got %v", err)
	}
}

// ApplyLocaleToRead and ApplyLocaleToWrite tests have been moved to
// pkg/dalgo2ingitdb/locale_test.go in the shared package.

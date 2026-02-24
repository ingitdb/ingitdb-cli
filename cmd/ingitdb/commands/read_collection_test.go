package commands

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dal-go/dalgo/dal"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2fsingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func TestReadCollection_Success(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ingitdb.SchemaDir), 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	defContent := []byte("titles:\n  en: Test Items\nrecord_file:\n  name: \"{key}.yaml\"\n  type: \"map[string]any\"\n  format: yaml\ncolumns:\n  name:\n    type: string\n")
	collectionDefPath := filepath.Join(dir, ingitdb.SchemaDir, "test.items.yaml")
	if err := os.WriteFile(collectionDefPath, defContent, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	def := testDef(dir)
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil }
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
	}
	logf := func(...any) {}

	cmd := Read(homeDir, getWd, readDef, newDB, logf)
	if err := runCLICommand(cmd, "collection", "--path="+dir, "--collection=test.items"); err != nil {
		t.Fatalf("ReadCollection: %v", err)
	}
}

func TestReadCollection_CollectionNotFound(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := testDef(dir)

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil }
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
	}
	logf := func(...any) {}

	cmd := Read(homeDir, getWd, readDef, newDB, logf)
	err := runCLICommand(cmd, "collection", "--path="+dir, "--collection=no.such.collection")
	if err == nil {
		t.Fatal("expected error for unknown collection")
	}
}

func TestReadCollection_SlashNormalizedCollectionIDRejected(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := testDef(dir)

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil }
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
	}
	logf := func(...any) {}

	cmd := Read(homeDir, getWd, readDef, newDB, logf)
	err := runCLICommand(cmd, "collection", "--path="+dir, "--collection=test/items")
	if err == nil {
		t.Fatal("expected error for slash-normalized collection ID")
	}
}

func TestReadCollection_DefinitionError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return nil, errors.New("boom")
	}
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
	}
	logf := func(...any) {}

	cmd := Read(homeDir, getWd, readDef, newDB, logf)
	err := runCLICommand(cmd, "collection", "--path="+dir, "--collection=test.items")
	if err == nil {
		t.Fatal("expected error when readDefinition fails")
	}
}

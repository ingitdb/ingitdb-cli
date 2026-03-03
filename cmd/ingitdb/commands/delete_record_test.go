package commands

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dal-go/dalgo/dal"
	"gopkg.in/yaml.v3"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2fsingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func TestDeleteRecord_Success(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := testDef(dir)
	content, err := yaml.Marshal(map[string]any{"name": "Bye"})
	if err != nil {
		t.Fatalf("yaml.Marshal: %v", err)
	}
	recordsDir := filepath.Join(dir, "$records")
	if err = os.MkdirAll(recordsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	path := filepath.Join(recordsDir, "bye.yaml")
	if err = os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil }
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
	}
	logf := func(...any) {}

	cmd := Delete(homeDir, getWd, readDef, newDB, logf)
	if err = runCLICommand(cmd, "record", "--path="+dir, "--id=test.items/bye"); err != nil {
		t.Fatalf("delete record: %v", err)
	}
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatal("expected file to be deleted")
	}
}

func TestDeleteRecord_NotFound(t *testing.T) {
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

	cmd := Delete(homeDir, getWd, readDef, newDB, logf)
	err := runCLICommand(cmd, "record", "--path="+dir, "--id=test.items/ghost")
	if err == nil {
		t.Fatal("expected error for not-found record")
	}
}

func TestDeleteRecord_ReadDefinitionError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return nil, errors.New("read def error")
	}
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
	}
	logf := func(...any) {}

	cmd := Delete(homeDir, getWd, readDef, newDB, logf)
	err := runCLICommand(cmd, "record", "--path="+dir, "--id=test.items/item")
	if err == nil {
		t.Fatal("expected error when read definition fails")
	}
}

func TestDeleteRecord_InvalidID(t *testing.T) {
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

	cmd := Delete(homeDir, getWd, readDef, newDB, logf)
	err := runCLICommand(cmd, "record", "--path="+dir, "--id=invalid")
	if err == nil {
		t.Fatal("expected error for invalid ID format")
	}
}

func TestDeleteRecord_DBOpenError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := testDef(dir)

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil }
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return nil, errors.New("db open error")
	}
	logf := func(...any) {}

	cmd := Delete(homeDir, getWd, readDef, newDB, logf)
	err := runCLICommand(cmd, "record", "--path="+dir, "--id=test.items/item")
	if err == nil {
		t.Fatal("expected error when DB open fails")
	}
}

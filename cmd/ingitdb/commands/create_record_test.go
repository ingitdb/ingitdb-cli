package commands

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dal-go/dalgo/dal"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2fsingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func TestCreate_Success(t *testing.T) {
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

	cmd := Create(homeDir, getWd, readDef, newDB, logf, nil, nil, nil)
	err := runCobraCommand(cmd, "record", "--path="+dir, "--id=test.items/hello", "--data={name: Hello}")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if _, statErr := os.Stat(filepath.Join(dir, "$records", "hello.yaml")); statErr != nil {
		t.Fatalf("expected file hello.yaml to be created: %v", statErr)
	}
}

func TestCreate_MissingID(t *testing.T) {
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

	cmd := Create(homeDir, getWd, readDef, newDB, logf, nil, nil, nil)
	err := runCobraCommand(cmd, "record", "--path="+dir, "--data={name: Hello}")
	if err == nil {
		t.Fatal("expected error for missing --id flag")
	}
}

func TestCreate_InvalidYAML(t *testing.T) {
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

	cmd := Create(homeDir, getWd, readDef, newDB, logf, nil, nil, nil)
	err := runCobraCommand(cmd, "record", "--path="+dir, "--id=test.items/x", "--data=: invalid: yaml: :")
	if err == nil {
		t.Fatal("expected error for invalid YAML in --data")
	}
}

func TestCreate_CollectionNotFound(t *testing.T) {
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

	cmd := Create(homeDir, getWd, readDef, newDB, logf, nil, nil, nil)
	err := runCobraCommand(cmd, "record", "--path="+dir, "--id=no/such/thing", "--data={name: X}")
	if err == nil {
		t.Fatal("expected error for unknown collection")
	}
}

func TestCreate_ReadDefinitionError(t *testing.T) {
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

	cmd := Create(homeDir, getWd, readDef, newDB, logf, nil, nil, nil)
	err := runCobraCommand(cmd, "record", "--path="+dir, "--id=test.items/x", "--data={name: X}")
	if err == nil {
		t.Fatal("expected error when readDefinition fails")
	}
}

func TestCreate_TTYError(t *testing.T) {
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

	cmd := Create(homeDir, getWd, readDef, newDB, logf,
		strings.NewReader(""),
		func() bool { return true },
		nil,
	)
	err := runCobraCommand(cmd, "record", "--path="+dir, "--id=test.items/x")
	if err == nil {
		t.Fatal("expected error when stdin is a TTY and no --data or --edit")
	}
	if !strings.Contains(err.Error(), "stdin") && !strings.Contains(err.Error(), "--edit") {
		t.Fatalf("error should mention stdin or --edit, got: %v", err)
	}
}

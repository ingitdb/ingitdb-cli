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

func TestUpdate_Success(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := testDef(dir)
	initial, err := yaml.Marshal(map[string]any{"name": "Old"})
	if err != nil {
		t.Fatalf("yaml.Marshal: %v", err)
	}
	recordsDir := filepath.Join(dir, "$records")
	if err = os.MkdirAll(recordsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err = os.WriteFile(filepath.Join(recordsDir, "item.yaml"), initial, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil }
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
	}
	logf := func(...any) {}

	cmd := UpdateLegacy(homeDir, getWd, readDef, newDB, logf)
	if err = runCobraCommand(cmd, "record", "--path="+dir, "--id=test.items/item", "--set={name: New}"); err != nil {
		t.Fatalf("Update: %v", err)
	}

	content, readErr := os.ReadFile(filepath.Join(recordsDir, "item.yaml"))
	if readErr != nil {
		t.Fatalf("ReadFile: %v", readErr)
	}
	var got map[string]any
	if err = yaml.Unmarshal(content, &got); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}
	if got["name"] != "New" {
		t.Fatalf("expected name=New, got %v", got["name"])
	}
}

func TestUpdate_NotFound(t *testing.T) {
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

	cmd := UpdateLegacy(homeDir, getWd, readDef, newDB, logf)
	err := runCobraCommand(cmd, "record", "--path="+dir, "--id=test.items/ghost", "--set={name: X}")
	if err == nil {
		t.Fatal("expected error for not-found record")
	}
}

func TestUpdate_InvalidSetYAML(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := testDef(dir)
	initial, err := yaml.Marshal(map[string]any{"name": "Old"})
	if err != nil {
		t.Fatalf("yaml.Marshal: %v", err)
	}
	recordsDir2 := filepath.Join(dir, "$records")
	if err = os.MkdirAll(recordsDir2, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err = os.WriteFile(filepath.Join(recordsDir2, "item.yaml"), initial, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil }
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
	}
	logf := func(...any) {}

	cmd := UpdateLegacy(homeDir, getWd, readDef, newDB, logf)
	err = runCobraCommand(cmd, "record", "--path="+dir, "--id=test.items/item", "--set=: invalid yaml :")
	if err == nil {
		t.Fatal("expected error for invalid YAML in --set")
	}
}

func TestUpdate_ReadDefinitionError(t *testing.T) {
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

	cmd := UpdateLegacy(homeDir, getWd, readDef, newDB, logf)
	err := runCobraCommand(cmd, "record", "--path="+dir, "--id=test.items/item", "--set={name: X}")
	if err == nil {
		t.Fatal("expected error when read definition fails")
	}
}

func TestUpdate_InvalidID(t *testing.T) {
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

	cmd := UpdateLegacy(homeDir, getWd, readDef, newDB, logf)
	err := runCobraCommand(cmd, "record", "--path="+dir, "--id=invalid", "--set={name: X}")
	if err == nil {
		t.Fatal("expected error for invalid ID format")
	}
}

func TestUpdate_DBOpenError(t *testing.T) {
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

	cmd := UpdateLegacy(homeDir, getWd, readDef, newDB, logf)
	err := runCobraCommand(cmd, "record", "--path="+dir, "--id=test.items/item", "--set={name: X}")
	if err == nil {
		t.Fatal("expected error when DB open fails")
	}
}

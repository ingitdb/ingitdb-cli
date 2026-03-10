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

func TestReadRecord_Success(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := testDef(dir)
	content, err := yaml.Marshal(map[string]any{"name": "Test"})
	if err != nil {
		t.Fatalf("yaml.Marshal: %v", err)
	}
	recordsDir := filepath.Join(dir, "$records")
	if err = os.MkdirAll(recordsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err = os.WriteFile(filepath.Join(recordsDir, "r1.yaml"), content, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil }
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
	}
	logf := func(...any) {}

	cmd := Read(homeDir, getWd, readDef, newDB, logf)
	if err = runCobraCommand(cmd, "record", "--path="+dir, "--id=test.items/r1"); err != nil {
		t.Fatalf("ReadRecord: %v", err)
	}
}

func TestReadRecord_NotFound(t *testing.T) {
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
	err := runCobraCommand(cmd, "record", "--path="+dir, "--id=test.items/ghost")
	if err == nil {
		t.Fatal("expected error for not-found record")
	}
}

func TestReadRecord_JSONFormat(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := testDef(dir)
	content, err := yaml.Marshal(map[string]any{"name": "Test"})
	if err != nil {
		t.Fatalf("yaml.Marshal: %v", err)
	}
	recordsDir := filepath.Join(dir, "$records")
	if err = os.MkdirAll(recordsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err = os.WriteFile(filepath.Join(recordsDir, "r1.yaml"), content, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil }
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
	}
	logf := func(...any) {}

	cmd := Read(homeDir, getWd, readDef, newDB, logf)
	if err = runCobraCommand(cmd, "record", "--path="+dir, "--id=test.items/r1", "--format=json"); err != nil {
		t.Fatalf("ReadRecord with JSON format: %v", err)
	}
}

func TestReadRecord_InvalidFormat(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := testDef(dir)
	content, err := yaml.Marshal(map[string]any{"name": "Test"})
	if err != nil {
		t.Fatalf("yaml.Marshal: %v", err)
	}
	recordsDir := filepath.Join(dir, "$records")
	if err = os.MkdirAll(recordsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err = os.WriteFile(filepath.Join(recordsDir, "r1.yaml"), content, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil }
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
	}
	logf := func(...any) {}

	cmd := Read(homeDir, getWd, readDef, newDB, logf)
	err = runCobraCommand(cmd, "record", "--path="+dir, "--id=test.items/r1", "--format=xml")
	if err == nil {
		t.Fatal("expected error for invalid format")
	}
}

func TestReadRecord_ReadDefinitionError(t *testing.T) {
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

	cmd := Read(homeDir, getWd, readDef, newDB, logf)
	err := runCobraCommand(cmd, "record", "--path="+dir, "--id=test.items/r1")
	if err == nil {
		t.Fatal("expected error when read definition fails")
	}
}

func TestReadRecord_InvalidID(t *testing.T) {
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
	err := runCobraCommand(cmd, "record", "--path="+dir, "--id=invalid")
	if err == nil {
		t.Fatal("expected error for invalid ID format")
	}
}

func TestReadRecord_DBOpenError(t *testing.T) {
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

	cmd := Read(homeDir, getWd, readDef, newDB, logf)
	err := runCobraCommand(cmd, "record", "--path="+dir, "--id=test.items/r1")
	if err == nil {
		t.Fatal("expected error when DB open fails")
	}
}

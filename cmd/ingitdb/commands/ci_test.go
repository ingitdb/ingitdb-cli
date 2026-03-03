package commands

import (
	"fmt"
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func TestCI_ReturnsCommand(t *testing.T) {
	t.Parallel()

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "/tmp/db", nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return &ingitdb.Definition{}, nil
	}
	logf := func(...any) {}

	cmd := CI(homeDir, getWd, readDef, nil, logf)
	if cmd == nil {
		t.Fatal("CI() returned nil")
	}
	if cmd.Name != "ci" {
		t.Errorf("expected name 'ci', got %q", cmd.Name)
	}
	if cmd.Action == nil {
		t.Fatal("expected Action to be set")
	}
}

func TestCI_NotYetImplemented(t *testing.T) {
	t.Parallel()

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "/tmp/db", nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return &ingitdb.Definition{}, nil
	}
	logf := func(...any) {}

	cmd := CI(homeDir, getWd, readDef, nil, logf)
	err := runCLICommand(cmd)
	if err == nil {
		t.Fatal("expected error when viewBuilder is nil")
	}
}

func TestCI_Success(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"test.items": {
				ID:      "test.items",
				DirPath: dir,
			},
		},
	}

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return def, nil
	}
	viewBuilder := &mockViewBuilder{
		result: &ingitdb.MaterializeResult{
			FilesCreated:   1,
			FilesUpdated:   1,
			FilesUnchanged: 1,
		},
	}
	logf := func(...any) {}

	cmd := CI(homeDir, getWd, readDef, viewBuilder, logf)
	err := runCLICommand(cmd, "--path="+dir)
	if err != nil {
		t.Fatalf("CI: %v", err)
	}
}

func TestCI_BuildViewsError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"test.items": {
				ID:      "test.items",
				DirPath: dir,
			},
		},
	}

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return def, nil
	}
	viewBuilder := &mockViewBuilder{
		err: fmt.Errorf("build error"),
	}
	logf := func(...any) {}

	cmd := CI(homeDir, getWd, readDef, viewBuilder, logf)
	err := runCLICommand(cmd, "--path="+dir)
	if err == nil {
		t.Fatal("expected error when BuildViews fails")
	}
}

func TestCI_GetWdError(t *testing.T) {
	t.Parallel()

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "", fmt.Errorf("no wd") }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return &ingitdb.Definition{}, nil
	}
	viewBuilder := &mockViewBuilder{}
	logf := func(...any) {}

	cmd := CI(homeDir, getWd, readDef, viewBuilder, logf)
	err := runCLICommand(cmd)
	if err == nil {
		t.Fatal("expected error when getWd fails")
	}
}

package commands

import (
	"fmt"
	"testing"

	"github.com/dal-go/dalgo/dal"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func TestServe_ReturnsCommand(t *testing.T) {
	t.Parallel()

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "/tmp/db", nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return &ingitdb.Definition{}, nil
	}
	newDB := func(_ string, _ *ingitdb.Definition) (dal.DB, error) {
		return nil, nil
	}
	logf := func(...any) {}

	cmd := Serve(homeDir, getWd, readDef, newDB, logf)
	if cmd == nil {
		t.Fatal("Serve() returned nil")
	}
	if cmd.Use != "serve" {
		t.Errorf("expected name 'serve', got %q", cmd.Name())
	}
	if cmd.RunE == nil {
		t.Fatal("expected Action to be set")
	}
}

func TestServe_NoModeSpecified(t *testing.T) {
	t.Parallel()

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "/tmp/db", nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return &ingitdb.Definition{}, nil
	}
	newDB := func(_ string, _ *ingitdb.Definition) (dal.DB, error) {
		return nil, nil
	}
	logf := func(...any) {}

	cmd := Serve(homeDir, getWd, readDef, newDB, logf)
	err := runCobraCommand(cmd)
	if err == nil {
		t.Fatal("expected error when no server mode is specified")
	}
}

func TestServe_ResolvePathError(t *testing.T) {
	t.Parallel()

	homeDir := func() (string, error) { return "", fmt.Errorf("no home") }
	getWd := func() (string, error) { return "", fmt.Errorf("no wd") }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return &ingitdb.Definition{}, nil
	}
	newDB := func(_ string, _ *ingitdb.Definition) (dal.DB, error) {
		return nil, nil
	}
	logf := func(...any) {}

	cmd := Serve(homeDir, getWd, readDef, newDB, logf)
	err := runCobraCommand(cmd, "--mcp")
	if err == nil {
		t.Fatal("expected error when getWd fails")
	}
}

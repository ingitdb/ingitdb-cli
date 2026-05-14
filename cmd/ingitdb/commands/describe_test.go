package commands

import (
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func TestDescribe_ReturnsCommand(t *testing.T) {
	t.Parallel()
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "/tmp/db", nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return &ingitdb.Definition{}, nil
	}
	cmd := Describe(homeDir, getWd, readDef)
	if cmd == nil {
		t.Fatal("Describe() returned nil")
	}
	if cmd.Use != "describe <kind> <name>" {
		t.Errorf("unexpected Use: %q", cmd.Use)
	}
	gotAliases := cmd.Aliases
	if len(gotAliases) != 1 || gotAliases[0] != "desc" {
		t.Errorf("expected desc alias; got %v", gotAliases)
	}
	if len(cmd.Commands()) != 2 {
		t.Fatalf("expected 2 subcommands (collection, view); got %d", len(cmd.Commands()))
	}
}

func TestDescribe_RootRequiresKind(t *testing.T) {
	t.Parallel()
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "/tmp/db", nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return &ingitdb.Definition{}, nil
	}
	cmd := Describe(homeDir, getWd, readDef)
	err := runCobraCommand(cmd)
	if err == nil {
		t.Fatal("expected error when invoked without a kind")
	}
}

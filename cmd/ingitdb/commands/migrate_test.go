package commands

import (
	"testing"
)

func TestMigrate_ReturnsCommand(t *testing.T) {
	t.Parallel()

	cmd := Migrate()
	if cmd == nil {
		t.Fatal("Migrate() returned nil")
		return
	}
	if cmd.Name != "migrate" {
		t.Errorf("expected name 'migrate', got %q", cmd.Name)
	}
	if cmd.Action == nil {
		t.Fatal("expected Action to be set")
	}
}

func TestMigrate_NotYetImplemented(t *testing.T) {
	t.Parallel()

	cmd := Migrate()
	err := runCLICommand(cmd, "--from=v1", "--to=v2", "--target=collection")
	if err == nil {
		t.Fatal("expected error for not-yet-implemented command")
	}
}

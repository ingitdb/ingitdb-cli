package commands

import (
	"testing"
)

func TestQuery_ReturnsCommand(t *testing.T) {
	t.Parallel()

	cmd := Query()
	if cmd == nil {
		t.Fatal("Query() returned nil")
		return
	}
	if cmd.Name != "query" {
		t.Errorf("expected name 'query', got %q", cmd.Name)
	}
	if cmd.Action == nil {
		t.Fatal("expected Action to be set")
	}
}

func TestQuery_NotYetImplemented(t *testing.T) {
	t.Parallel()

	cmd := Query()
	err := runCLICommand(cmd, "--collection=test.items")
	if err == nil {
		t.Fatal("expected error for not-yet-implemented command")
	}
}

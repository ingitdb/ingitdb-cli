package commands

import (
	"testing"
)

func TestDeleteRecords_ReturnsCommand(t *testing.T) {
	t.Parallel()

	cmd := deleteRecords()
	if cmd == nil {
		t.Fatal("deleteRecords() returned nil")
		return
	}
	if cmd.Name != "records" {
		t.Errorf("expected name 'records', got %q", cmd.Name)
	}
	if cmd.Action == nil {
		t.Fatal("expected Action to be set")
	}
}

func TestDeleteRecords_NotYetImplemented(t *testing.T) {
	t.Parallel()

	cmd := deleteRecords()
	err := runCLICommand(cmd, "--collection=test.items")
	if err == nil {
		t.Fatal("expected error for not-yet-implemented command")
	}
}

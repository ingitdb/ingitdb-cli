package commands

import (
	"testing"
)

func TestDeleteView_ReturnsCommand(t *testing.T) {
	t.Parallel()

	cmd := deleteView()
	if cmd == nil {
		t.Fatal("deleteView() returned nil")
		return
	}
	if cmd.Name != "view" {
		t.Errorf("expected name 'view', got %q", cmd.Name)
	}
	if cmd.Action == nil {
		t.Fatal("expected Action to be set")
	}
}

func TestDeleteView_NotYetImplemented(t *testing.T) {
	t.Parallel()

	cmd := deleteView()
	err := runCLICommand(cmd, "--view=test.view")
	if err == nil {
		t.Fatal("expected error for not-yet-implemented command")
	}
}

package commands

import (
	"testing"
)

func TestPull_ReturnsCommand(t *testing.T) {
	t.Parallel()

	cmd := Pull()
	if cmd == nil {
		t.Fatal("Pull() returned nil")
		return
	}
	if cmd.Use != "pull" {
		t.Errorf("expected name 'pull', got %q", cmd.Name())
	}
	if cmd.RunE == nil {
		t.Fatal("expected Action to be set")
	}
}

func TestPull_NotYetImplemented(t *testing.T) {
	t.Parallel()

	cmd := Pull()
	err := runCobraCommand(cmd)
	if err == nil {
		t.Fatal("expected error for not-yet-implemented command")
	}
}

package commands

import (
	"testing"
)

func TestFind_ReturnsCommand(t *testing.T) {
	t.Parallel()

	cmd := Find()
	if cmd == nil {
		t.Fatal("Find() returned nil")
		return
	}
	if cmd.Use != "find" {
		t.Errorf("expected name 'find', got %q", cmd.Name())
	}
	if cmd.RunE == nil {
		t.Fatal("expected Action to be set")
	}
}

func TestFind_NotYetImplemented(t *testing.T) {
	t.Parallel()

	cmd := Find()
	err := runCobraCommand(cmd)
	if err == nil {
		t.Fatal("expected error for not-yet-implemented command")
	}
}

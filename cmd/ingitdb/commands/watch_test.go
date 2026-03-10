package commands

import (
	"testing"
)

func TestWatch_ReturnsCommand(t *testing.T) {
	t.Parallel()

	cmd := Watch()
	if cmd == nil {
		t.Fatal("Watch() returned nil")
	}
	if cmd.Use != "watch" {
		t.Errorf("expected name 'watch', got %q", cmd.Name())
	}
	if cmd.RunE == nil {
		t.Fatal("expected Action to be set")
	}
}

func TestWatch_NotYetImplemented(t *testing.T) {
	t.Parallel()

	cmd := Watch()
	err := runCobraCommand(cmd)
	if err == nil {
		t.Fatal("expected error for not-yet-implemented command")
	}
}

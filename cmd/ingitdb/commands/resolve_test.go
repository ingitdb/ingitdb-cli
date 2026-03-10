package commands

import (
	"testing"
)

func TestResolve_ReturnsCommand(t *testing.T) {
	t.Parallel()

	cmd := Resolve()
	if cmd == nil {
		t.Fatal("Resolve() returned nil")
		return
	}
	if cmd.Use != "resolve" {
		t.Errorf("expected name 'resolve', got %q", cmd.Name())
	}
	if cmd.RunE == nil {
		t.Fatal("expected Action to be set")
	}
}

func TestResolve_NotYetImplemented(t *testing.T) {
	t.Parallel()

	cmd := Resolve()
	err := runCobraCommand(cmd)
	if err == nil {
		t.Fatal("expected error for not-yet-implemented command")
	}
}

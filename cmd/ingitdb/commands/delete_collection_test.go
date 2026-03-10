package commands

import (
	"testing"
)

func TestDeleteCollection_ReturnsCommand(t *testing.T) {
	t.Parallel()

	cmd := deleteCollection()
	if cmd == nil {
		t.Fatal("deleteCollection() returned nil")
		return
	}
	if cmd.Use != "collection" {
		t.Errorf("expected name 'collection', got %q", cmd.Name())
	}
	if cmd.RunE == nil {
		t.Fatal("expected Action to be set")
	}
}

func TestDeleteCollection_NotYetImplemented(t *testing.T) {
	t.Parallel()

	cmd := deleteCollection()
	err := runCobraCommand(cmd, "--collection=test.items")
	if err == nil {
		t.Fatal("expected error for not-yet-implemented command")
	}
}

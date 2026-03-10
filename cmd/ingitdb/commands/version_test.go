package commands

import (
	"testing"
)

func TestVersion_ReturnsCommand(t *testing.T) {
	t.Parallel()

	cmd := Version("1.0.0", "abc123", "2024-01-01")
	if cmd == nil {
		t.Fatal("Version() returned nil")
	}
	if cmd.Use != "version" {
		t.Errorf("expected name 'version', got %q", cmd.Name())
	}
	if cmd.RunE == nil {
		t.Fatal("expected Action to be set")
	}
}

func TestVersion_PrintsVersionInfo(t *testing.T) {
	t.Parallel()

	cmd := Version("1.0.0", "abc123", "2024-01-01")
	err := runCobraCommand(cmd)
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
}

func TestVersion_EmptyValues(t *testing.T) {
	t.Parallel()

	cmd := Version("", "", "")
	err := runCobraCommand(cmd)
	if err != nil {
		t.Fatalf("Version with empty values: %v", err)
	}
}

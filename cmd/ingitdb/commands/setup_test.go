package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetup_WritesEmptySettingsYAML(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := runSetup(dir, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, ".ingitdb", "settings.yaml"))
	if err != nil {
		t.Fatalf("expected settings.yaml to exist: %v", err)
	}
	// AC-2 allows either omission of the field or empty string. Our impl
	// emits an empty file when no flags are set.
	if strings.Contains(string(got), "default_record_format:") &&
		!strings.Contains(string(got), "default_record_format: \"\"") &&
		!strings.Contains(string(got), "default_record_format: ''") {
		// Field present and non-empty — only allowed if flag was passed.
		t.Errorf("expected default_record_format to be absent or empty; got: %q", string(got))
	}
}

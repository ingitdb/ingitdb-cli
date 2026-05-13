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

func TestSetup_DefaultFormatFlag_WritesIngrToSettings(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := runSetup(dir, "ingr"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, ".ingitdb", "settings.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "default_record_format: ingr") {
		t.Errorf("expected settings.yaml to contain 'default_record_format: ingr'; got: %q", string(got))
	}
}

func TestSetup_DefaultFormatFlag_RejectsUnsupportedValue(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	err := runSetup(dir, "xml")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	msg := err.Error()
	for _, name := range []string{"xml", "yaml", "yml", "json", "markdown", "toml", "ingr", "csv"} {
		if !strings.Contains(msg, name) {
			t.Errorf("expected error to mention %q; got: %s", name, msg)
		}
	}
	// AC: no .ingitdb/ directory should have been created on rejection.
	if _, statErr := os.Stat(filepath.Join(dir, ".ingitdb")); !os.IsNotExist(statErr) {
		t.Errorf("expected .ingitdb directory to NOT exist after rejection; stat err: %v", statErr)
	}
}

func TestSetup_DefaultFormatFlag_AcceptsAllSevenFormats(t *testing.T) {
	t.Parallel()
	for _, f := range []string{"yaml", "yml", "json", "markdown", "toml", "ingr", "csv"} {
		f := f
		t.Run(f, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			if err := runSetup(dir, f); err != nil {
				t.Errorf("expected nil error for %q, got: %v", f, err)
			}
		})
	}
}

func TestSetup_DefaultFormatFlag_LoadsBackCleanly(t *testing.T) {
	t.Parallel()
	// Round-trip placeholder: setup with csv succeeds. The full
	// ReadSettingsFromFile + ResolveRecordFormat round-trip lives in Task 17.
	dir := t.TempDir()
	if err := runSetup(dir, "csv"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

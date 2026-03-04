package dalgo2fsingitdb

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// errYAMLMarshalerValue implements yaml.Marshaler and always returns an error.
// This is used to reach the `yaml.Marshal` error-return branch in writeRecordToFile
// without triggering the yaml library's panic path for unsupported Go types.
type errYAMLMarshalerValue struct{}

func (errYAMLMarshalerValue) MarshalYAML() (interface{}, error) {
	return nil, fmt.Errorf("intentional yaml marshal error")
}

func TestReadRecordFromFile_UnsupportedFormat(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	err := os.WriteFile(path, []byte("content"), 0o644)
	if err != nil {
		t.Fatalf("setup: write test file: %v", err)
	}

	_, _, err = readRecordFromFile(path, "xml")
	if err == nil {
		t.Fatal("expected error for unsupported format, got nil")
	}
	expected := `unsupported record format "xml"`
	if err.Error() != expected {
		t.Errorf("error message = %q, want %q", err.Error(), expected)
	}
}

func TestReadRecordFromFile_YAMLParseError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "invalid.yaml")
	// Write malformed YAML
	err := os.WriteFile(path, []byte("key: [unclosed"), 0o644)
	if err != nil {
		t.Fatalf("setup: write test file: %v", err)
	}

	_, _, err = readRecordFromFile(path, "yaml")
	if err == nil {
		t.Fatal("expected YAML parse error, got nil")
	}
	// Should contain "failed to parse YAML file"
	if err.Error()[:25] != "failed to parse YAML file" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestReadRecordFromFile_JSONParseError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "invalid.json")
	// Write malformed JSON
	err := os.WriteFile(path, []byte(`{"key": unclosed`), 0o644)
	if err != nil {
		t.Fatalf("setup: write test file: %v", err)
	}

	_, _, err = readRecordFromFile(path, "json")
	if err == nil {
		t.Fatal("expected JSON parse error, got nil")
	}
	// Should contain "failed to parse JSON file"
	if err.Error()[:25] != "failed to parse JSON file" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestReadRecordFromFile_ReadPermissionError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "noperm.yaml")
	err := os.WriteFile(path, []byte("key: value"), 0o644)
	if err != nil {
		t.Fatalf("setup: write test file: %v", err)
	}

	// Remove read permissions
	err = os.Chmod(path, 0o000)
	if err != nil {
		t.Fatalf("setup: chmod: %v", err)
	}
	defer func() { _ = os.Chmod(path, 0o644) }() // Cleanup

	_, _, err = readRecordFromFile(path, "yaml")
	if err == nil {
		t.Fatal("expected permission error, got nil")
	}
	// Should contain "failed to read file"
	if err.Error()[:18] != "failed to read file" && err.Error()[:18] != "failed to read fil" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestWriteRecordToFile_UnsupportedFormat(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	data := map[string]any{"key": "value"}

	err := writeRecordToFile(path, "xml", data)
	if err == nil {
		t.Fatal("expected error for unsupported format, got nil")
	}
	expected := `unsupported record format "xml"`
	if err.Error() != expected {
		t.Errorf("error message = %q, want %q", err.Error(), expected)
	}
}

func TestWriteRecordToFile_YAMLMarshalError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")
	// Create data that cannot be marshaled (function)
	data := map[string]any{
		"key": func() {},
	}

	// YAML library panics on unmarshalable types, not returns error
	// This test documents that behavior
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for unmarshalable type")
		}
	}()

	_ = writeRecordToFile(path, "yaml", data)
	t.Fatal("should have panicked before this line")
}

func TestWriteRecordToFile_JSONMarshalError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")
	// Create data that cannot be marshaled (channel)
	data := map[string]any{
		"key": make(chan int),
	}

	err := writeRecordToFile(path, "json", data)
	if err == nil {
		t.Fatal("expected marshal error, got nil")
	}
	// Error message contains additional details
	if err.Error()[:31] != "failed to marshal data as JSON:" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestWriteRecordToFile_MkdirError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Create a file where we need a directory
	blockingFile := filepath.Join(dir, "blocking")
	err := os.WriteFile(blockingFile, []byte("content"), 0o644)
	if err != nil {
		t.Fatalf("setup: write blocking file: %v", err)
	}

	// Try to create a file in a path where a file exists
	path := filepath.Join(blockingFile, "subdir", "test.yaml")
	data := map[string]any{"key": "value"}

	err = writeRecordToFile(path, "yaml", data)
	if err == nil {
		t.Fatal("expected mkdir error, got nil")
	}
	// Should contain "failed to create directory"
	if err.Error()[:27] != "failed to create directory " {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestWriteRecordToFile_WriteError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Create a directory where we want to write a file
	path := filepath.Join(dir, "test.yaml")
	err := os.Mkdir(path, 0o755)
	if err != nil {
		t.Fatalf("setup: mkdir: %v", err)
	}

	// Try to write to a directory (should fail)
	data := map[string]any{"key": "value"}
	err = writeRecordToFile(path, "yaml", data)
	if err == nil {
		t.Fatal("expected write error, got nil")
	}
	// Should contain "failed to write file"
	if err.Error()[:20] != "failed to write file" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestReadMapOfRecordsFile_InvalidRecordValue(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "invalid.yaml")
	// Write YAML where a record value is not a map
	err := os.WriteFile(path, []byte("id1: not_a_map\nid2:\n  field: value"), 0o644)
	if err != nil {
		t.Fatalf("setup: write test file: %v", err)
	}

	_, _, err = readMapOfRecordsFile(path, "yaml")
	if err == nil {
		t.Fatal("expected error for non-map record value, got nil")
	}
}

func TestReadRecordFromFile_YMLFormat(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "test.yml")
	err := os.WriteFile(path, []byte("name: Test\nvalue: 123"), 0o644)
	if err != nil {
		t.Fatalf("setup: write test file: %v", err)
	}

	data, found, err := readRecordFromFile(path, "yml")
	if err != nil {
		t.Fatalf("readRecordFromFile: %v", err)
	}
	if !found {
		t.Fatal("expected record to be found")
	}
	if data["name"] != "Test" {
		t.Errorf("name = %v, want Test", data["name"])
	}
	if data["value"] != 123 {
		t.Errorf("value = %v, want 123", data["value"])
	}
}

func TestWriteRecordToFile_YMLFormat(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "test.yml")
	data := map[string]any{"name": "Test", "value": 123}

	err := writeRecordToFile(path, "yml", data)
	if err != nil {
		t.Fatalf("writeRecordToFile: %v", err)
	}

	content, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("ReadFile: %v", readErr)
	}

	// Verify content contains expected data
	strContent := string(content)
	if strContent == "" {
		t.Fatal("expected non-empty file content")
	}
}

func TestWriteRecordToFile_JSONFormat(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")
	data := map[string]any{"name": "Test", "value": 123}

	err := writeRecordToFile(path, "json", data)
	if err != nil {
		t.Fatalf("writeRecordToFile: %v", err)
	}

	content, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("ReadFile: %v", readErr)
	}

	// Verify JSON is formatted with newline at end
	if content[len(content)-1] != '\n' {
		t.Error("expected JSON file to end with newline")
	}
}

// TestWriteRecordToFile_YAMLMarshalerReturnsError covers the
// `return fmt.Errorf("failed to marshal data as YAML: %w", err)` branch.
// The errYAMLMarshalerValue type implements yaml.Marshaler and returns an
// error, which causes yaml.Marshal to propagate the error instead of
// panicking (unlike passing an unsupported Go type such as a function).
func TestWriteRecordToFile_YAMLMarshalerReturnsError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")
	data := map[string]any{
		"key": errYAMLMarshalerValue{},
	}

	err := writeRecordToFile(path, "yaml", data)
	if err == nil {
		t.Fatal("expected YAML marshal error, got nil")
	}
	const prefix = "failed to marshal data as YAML:"
	if len(err.Error()) < len(prefix) || err.Error()[:len(prefix)] != prefix {
		t.Errorf("error = %q, want prefix %q", err.Error(), prefix)
	}
}

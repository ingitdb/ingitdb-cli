package docsbuilder

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func TestResolveCollections(t *testing.T) {
	collections := map[string]*ingitdb.CollectionDef{
		"root1": {
			ID: "root1",
			SubCollections: map[string]*ingitdb.CollectionDef{
				"sub1": {
					ID: "sub1",
					SubCollections: map[string]*ingitdb.CollectionDef{
						"subsub1": {ID: "subsub1"},
					},
				},
			},
		},
		"root2": {ID: "root2"},
	}

	tests := []struct {
		name     string
		pattern  string
		expected []string
	}{
		{
			name:     "exact match root",
			pattern:  "root1",
			expected: []string{"root1"},
		},
		{
			name:     "exact match sub",
			pattern:  "root1.sub1",
			expected: []string{"sub1"},
		},
		{
			name:     "direct subcollections",
			pattern:  "root1/*",
			expected: []string{"root1", "sub1"},
		},
		{
			name:     "recursive subcollections",
			pattern:  "root1/**",
			expected: []string{"root1", "sub1", "subsub1"},
		},
		{
			name:     "all collections",
			pattern:  "**",
			expected: []string{"root1", "sub1", "subsub1", "root2"},
		},
		{
			name:     "empty pattern",
			pattern:  "",
			expected: nil,
		},
		{
			name:     "trailing dot empty part",
			pattern:  "root1.",
			expected: nil,
		},
		{
			name:     "double dots empty parts",
			pattern:  "root1..sub1",
			expected: nil,
		},
		{
			name:     "not found",
			pattern:  "unknown",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := ResolveCollections(collections, tt.pattern)
			var got []string
			for _, res := range results {
				got = append(got, res.ID)
			}

			// Map order is not guaranteed, so sort both before comparing
			// We can just verify lengths and contains
			if len(got) != len(tt.expected) {
				t.Fatalf("expected %d collections, got %d: %v", len(tt.expected), len(got), got)
			}

			// Simple check, works if no duplicates
			for _, exp := range tt.expected {
				found := false
				for _, g := range got {
					if g == exp {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected to find %s in %v", exp, got)
				}
			}
		})
	}
}

func TestFindCollectionByDir(t *testing.T) {
	collections := map[string]*ingitdb.CollectionDef{
		"root1": {
			ID:      "root1",
			DirPath: "/a/b",
			SubCollections: map[string]*ingitdb.CollectionDef{
				"sub1": {
					ID:      "sub1",
					DirPath: "/a/b/c",
				},
			},
		},
	}

	tests := []struct {
		name     string
		dir      string
		expected string
	}{
		{"root", "/a/b", "root1"},
		{"sub", "/a/b/c", "sub1"},
		{"not found", "/x/y", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FindCollectionByDir(collections, tt.dir)
			if tt.expected == "" {
				if got != nil {
					t.Fatalf("expected nil, got %s", got.ID)
				}
			} else {
				if got == nil || got.ID != tt.expected {
					gotID := "<nil>"
					if got != nil {
						gotID = got.ID
					}
					t.Fatalf("expected %s, got %s", tt.expected, gotID)
				}
			}
		})
	}
}

func TestFindCollectionsForConflictingFiles(t *testing.T) {
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"root": {
				ID:      "root",
				DirPath: "/repo/docs/root",
			},
			"sub": {
				ID:      "sub",
				DirPath: "/repo/docs/sub",
			},
		},
	}

	wd := "/repo"
	resolveItems := map[string]bool{"readme": true}

	tests := []struct {
		name            string
		conflicted      []string
		expectedCols    []string
		expectedReadmes []string
		expectedUnres   []string
	}{
		{
			name:            "basic readme conflict",
			conflicted:      []string{"docs/root/README.md"},
			expectedCols:    []string{"root"},
			expectedReadmes: []string{"docs/root/README.md"},
			expectedUnres:   nil,
		},
		{
			name:            "unresolved file",
			conflicted:      []string{"docs/root/README.md", "src/main.go"},
			expectedCols:    []string{"root"},
			expectedReadmes: []string{"docs/root/README.md"},
			expectedUnres:   []string{"src/main.go"},
		},
		{
			name:            "readme outside collections",
			conflicted:      []string{"docs/unknown/README.md"},
			expectedCols:    nil, // Path doesn't match a collection dir
			expectedReadmes: []string{"docs/unknown/README.md"},
			expectedUnres:   nil,
		},
		{
			name:            "empty conflicted",
			conflicted:      []string{""},
			expectedCols:    nil,
			expectedReadmes: nil,
			expectedUnres:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cols, readmes, unres := FindCollectionsForConflictingFiles(def, wd, tt.conflicted, resolveItems)

			if len(cols) != len(tt.expectedCols) {
				t.Fatalf("expected %d cols, got %d", len(tt.expectedCols), len(cols))
			}
			for i, e := range tt.expectedCols {
				if cols[i].ID != e {
					t.Errorf("expected col %s, got %s", e, cols[i].ID)
				}
			}

			if len(readmes) != len(tt.expectedReadmes) {
				t.Fatalf("expected %d readmes, got %d", len(tt.expectedReadmes), len(readmes))
			}
			for i, e := range tt.expectedReadmes {
				if readmes[i] != e {
					t.Errorf("expected readme %s, got %s", e, readmes[i])
				}
			}

			if len(unres) != len(tt.expectedUnres) {
				t.Fatalf("expected %d unres, got %d", len(tt.expectedUnres), len(unres))
			}
			for i, e := range tt.expectedUnres {
				if unres[i] != e {
					t.Errorf("expected unres %s, got %s", e, unres[i])
				}
			}
		})
	}
}

// MockRecordsReader is a simple struct to satisfy RecordsReader interface and return no records
type MockRecordsReader struct {
	YieldError error
}

func (m MockRecordsReader) ReadRecords(ctx context.Context, dirPath string, col *ingitdb.CollectionDef, yield func(ingitdb.RecordEntry) error) error {
	if m.YieldError != nil {
		return m.YieldError
	}
	return nil
}

func TestProcessCollection(t *testing.T) {
	dir := t.TempDir()
	col := &ingitdb.CollectionDef{
		ID:      "testcol",
		DirPath: dir,
		Columns: map[string]*ingitdb.ColumnDef{
			"id": {Type: ingitdb.ColumnTypeString},
		},
	}
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"testcol": col,
		},
	}

	reader := MockRecordsReader{}

	// First execution should write the file
	changed, err := ProcessCollection(context.Background(), def, col, dir, reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Errorf("expected collection to be changed on first run")
	}

	// Second execution should return false (unchanged)
	changed, err = ProcessCollection(context.Background(), def, col, dir, reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if changed {
		t.Errorf("expected collection to be unchanged on second run")
	}

	// Test Error handling rendering - DataPreview failure
	colWithError := &ingitdb.CollectionDef{
		ID:      "bad",
		DirPath: dir,
		Readme: &ingitdb.CollectionReadmeDef{
			DataPreview: &ingitdb.ViewDef{Template: "missing.html"},
		},
	}
	_, err = ProcessCollection(context.Background(), def, colWithError, dir, reader)
	if err == nil {
		t.Errorf("expected error during process collection with bad view")
	}

	// Test Error handling - WriteFile failure (mock unwritable directory)
	unwritableDir := filepath.Join(dir, "unwritable")
	_ = os.MkdirAll(unwritableDir, 0o555) // read/execute only
	colUnwritable := &ingitdb.CollectionDef{
		ID:      "unwritable-col",
		DirPath: unwritableDir,
	}
	// Try creating a directory that acts as the file to cause a write failure, or use unwritable dir.
	// A simpler way to fail WriteFile is to create a directory where the file should be.
	readmePath := filepath.Join(unwritableDir, "README.md")
	_ = os.Mkdir(readmePath, 0o755)

	_, err = ProcessCollection(context.Background(), def, colUnwritable, unwritableDir, reader)
	if err == nil {
		t.Errorf("expected error during write file")
	}

	// Test Error handling - BuildView RecordsReader failure
	readerWithError := MockRecordsReader{YieldError: fmt.Errorf("mock reader error")}
	colReaderFail := &ingitdb.CollectionDef{
		ID:      "reader-fail",
		DirPath: dir,
		Readme: &ingitdb.CollectionReadmeDef{
			DataPreview: &ingitdb.ViewDef{Template: "dummy"}, // triggers renderer
		},
	}
	_, err = ProcessCollection(context.Background(), def, colReaderFail, dir, readerWithError)
	if err == nil || !strings.Contains(err.Error(), "mock reader error") {
		t.Errorf("expected reader error, got %v", err)
	}

	// Test Success rendering DataPreview inline
	validTemplatePath := filepath.Join(dir, "valid.md")
	_ = os.WriteFile(validTemplatePath, []byte("data preview {{ len .records }}"), 0o644)
	colSuccessRender := &ingitdb.CollectionDef{
		ID:      "success-render",
		DirPath: dir,
		Readme: &ingitdb.CollectionReadmeDef{
			DataPreview: &ingitdb.ViewDef{Template: "valid.md"},
		},
	}

	changed, err = ProcessCollection(context.Background(), def, colSuccessRender, dir, reader)
	if err != nil {
		t.Errorf("expected successful data preview, got %v", err)
	}
	if !changed {
		t.Errorf("expected collection to change based on success render")
	}
}

func TestUpdateDocs(t *testing.T) {
	dir := t.TempDir()

	col1 := &ingitdb.CollectionDef{
		ID:      "c1",
		DirPath: filepath.Join(dir, "c1"),
	}
	col2 := &ingitdb.CollectionDef{
		ID:      "c2",
		DirPath: filepath.Join(dir, "c2"),
	}
	_ = os.MkdirAll(col1.DirPath, 0o755)
	_ = os.MkdirAll(col2.DirPath, 0o755)

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"c1": col1,
			"c2": col2,
		},
	}

	reader := MockRecordsReader{}

	// Test writing both
	res, err := UpdateDocs(context.Background(), def, "**", dir, reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.FilesWritten != 2 {
		t.Errorf("expected 2 files written, got %d", res.FilesWritten)
	}

	// Test unchanged
	res, err = UpdateDocs(context.Background(), def, "**", dir, reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.FilesUnchanged != 2 {
		t.Errorf("expected 2 files unchanged, got %d", res.FilesUnchanged)
	}

	// Test glob that matches nothing
	res, err = UpdateDocs(context.Background(), def, "nomatch", dir, reader)
	if err != nil {
		t.Fatalf("unexpected error for empty glob: %v", err)
	}
	if res.FilesWritten != 0 && res.FilesUnchanged != 0 {
		t.Errorf("expected no files processed, got written: %d, unchanged: %d", res.FilesWritten, res.FilesUnchanged)
	}

	// Test failure logic execution loop continuation
	colFail := &ingitdb.CollectionDef{
		ID:      "cf",
		DirPath: filepath.Join(dir, "cf"),
		Readme: &ingitdb.CollectionReadmeDef{
			DataPreview: &ingitdb.ViewDef{Template: "missing.html"},
		},
	}
	_ = os.MkdirAll(colFail.DirPath, 0o755)

	defFail := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"c1": col1,
			"cf": colFail,
		},
	}

	res, err = UpdateDocs(context.Background(), defFail, "**", dir, reader)
	if err != nil {
		t.Fatalf("expected error list collected, not immediate error: %v", err)
	}
	// "cf" fails, "c1" is unchanged
	if len(res.Errors) != 1 {
		t.Errorf("expected 1 error inside res.Errors array, got %d", len(res.Errors))
	}
	if res.FilesUnchanged != 1 {
		t.Errorf("expected 'c1' to be unchanged = 1, got %d", res.FilesUnchanged)
	}
}

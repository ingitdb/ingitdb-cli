package docsbuilder

import (
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

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

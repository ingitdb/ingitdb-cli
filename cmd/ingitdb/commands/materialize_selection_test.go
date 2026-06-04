package commands

import (
	"reflect"
	"testing"
)

func TestFlagSelection(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		changed   bool
		raw       string
		wantKind  selectionKind
		wantGlobs []string
	}{
		{name: "absent", changed: false, raw: "", wantKind: selectionNone},
		{name: "bare sentinel", changed: true, raw: materializeAllSentinel, wantKind: selectionAll},
		{name: "single glob", changed: true, raw: "cities", wantKind: selectionList, wantGlobs: []string{"cities"}},
		{name: "comma list", changed: true, raw: "cities,teams", wantKind: selectionList, wantGlobs: []string{"cities", "teams"}},
		{name: "semicolon list", changed: true, raw: "cities;teams", wantKind: selectionList, wantGlobs: []string{"cities", "teams"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sel := flagSelection(tc.changed, tc.raw)
			if sel.kind != tc.wantKind {
				t.Fatalf("kind: got %v, want %v", sel.kind, tc.wantKind)
			}
			if tc.wantKind == selectionList && !reflect.DeepEqual(sel.globs, tc.wantGlobs) {
				t.Errorf("globs: got %v, want %v", sel.globs, tc.wantGlobs)
			}
		})
	}
}

func TestSplitPatterns(t *testing.T) {
	t.Parallel()

	cases := []struct {
		raw  string
		want []string
	}{
		{raw: "a, b;c", want: []string{"a", "b", "c"}},
		{raw: "cities", want: []string{"cities"}},
		{raw: " cities ; teams , agile.teams/** ", want: []string{"cities", "teams", "agile.teams/**"}},
		{raw: "a,,b", want: []string{"a", "b"}},
		{raw: "", want: nil},
	}

	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			t.Parallel()
			got := splitPatterns(tc.raw)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("splitPatterns(%q): got %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}

func TestMatchViewNames(t *testing.T) {
	t.Parallel()

	names := []string{"active_cities", "large_cities", "README"}

	cases := []struct {
		name     string
		patterns []string
		want     []string
	}{
		{name: "exact", patterns: []string{"active_cities"}, want: []string{"active_cities"}},
		{name: "glob suffix", patterns: []string{"*_cities"}, want: []string{"active_cities", "large_cities"}},
		{name: "multi pattern", patterns: []string{"active_cities", "README"}, want: []string{"active_cities", "README"}},
		{name: "no match", patterns: []string{"missing"}, want: nil},
		{name: "dedup overlap", patterns: []string{"active_cities", "*_cities"}, want: []string{"active_cities", "large_cities"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := matchViewNames(names, tc.patterns)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("matchViewNames(%v, %v): got %v, want %v", names, tc.patterns, got, tc.want)
			}
		})
	}
}

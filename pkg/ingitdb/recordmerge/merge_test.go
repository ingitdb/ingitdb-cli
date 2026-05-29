package recordmerge

import (
	"testing"
)

func rec(key string, fields map[string]any) Record {
	return Record{Key: key, Fields: fields}
}

func f(kv ...any) map[string]any {
	m := make(map[string]any)
	for i := 0; i < len(kv); i += 2 {
		m[kv[i].(string)] = kv[i+1]
	}
	return m
}

// keys returns merged record keys in order for order-sensitive assertions.
func keys(records []Record) []string {
	out := make([]string, len(records))
	for i, r := range records {
		out[i] = r.Key
	}
	return out
}

func find(records []Record, key string) (map[string]any, bool) {
	for _, r := range records {
		if r.Key == key {
			return r.Fields, true
		}
	}
	return nil, false
}

func TestMerge_RecordSetCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		base, ours, their []Record
		opts              Options
		wantEscalate      bool
		wantKeys          []string // expected merged keys in order (when not escalating)
	}{
		{
			name:     "DM-1 disjoint additions unioned (ours-then-theirs)",
			base:     nil,
			ours:     []Record{rec("a", f("v", 1))},
			their:    []Record{rec("b", f("v", 2))},
			wantKeys: []string{"a", "b"},
		},
		{
			name:     "DM-1 ours-only addition",
			base:     nil,
			ours:     []Record{rec("a", f("v", 1))},
			their:    nil,
			wantKeys: []string{"a"},
		},
		{
			name:     "DM-1 theirs-only addition",
			base:     nil,
			ours:     nil,
			their:    []Record{rec("b", f("v", 2))},
			wantKeys: []string{"b"},
		},
		{
			name:     "DM-2 identical addition deduplicated",
			base:     nil,
			ours:     []Record{rec("a", f("v", 1))},
			their:    []Record{rec("a", f("v", 1))},
			wantKeys: []string{"a"},
		},
		{
			name:         "DM-12 primary-key collision escalates",
			base:         nil,
			ours:         []Record{rec("a", f("v", 1))},
			their:        []Record{rec("a", f("v", 2))},
			wantEscalate: true,
		},
		{
			name:     "DM-4 deleted on both sides",
			base:     []Record{rec("a", f("v", 1))},
			ours:     nil,
			their:    nil,
			wantKeys: []string{},
		},
		{
			name:     "DM-3 disjoint delete (ours deletes, theirs unchanged)",
			base:     []Record{rec("a", f("v", 1)), rec("b", f("v", 2))},
			ours:     []Record{rec("b", f("v", 2))},
			their:    []Record{rec("a", f("v", 1)), rec("b", f("v", 2))},
			wantKeys: []string{"b"},
		},
		{
			name:     "disjoint delete (theirs deletes, ours unchanged)",
			base:     []Record{rec("a", f("v", 1)), rec("b", f("v", 2))},
			ours:     []Record{rec("a", f("v", 1)), rec("b", f("v", 2))},
			their:    []Record{rec("a", f("v", 1))},
			wantKeys: []string{"a"},
		},
		{
			name:         "DM-15 delete/modify escalates (ours deletes, theirs modifies)",
			base:         []Record{rec("a", f("v", 1))},
			ours:         nil,
			their:        []Record{rec("a", f("v", 9))},
			wantEscalate: true,
		},
		{
			name:         "DM-15 delete/modify escalates (theirs deletes, ours modifies)",
			base:         []Record{rec("a", f("v", 1))},
			ours:         []Record{rec("a", f("v", 9))},
			their:        nil,
			wantEscalate: true,
		},
		{
			name:     "unchanged on both sides keeps base",
			base:     []Record{rec("a", f("v", 1))},
			ours:     []Record{rec("a", f("v", 1))},
			their:    []Record{rec("a", f("v", 1))},
			wantKeys: []string{"a"},
		},
		{
			name:     "one-sided edit (ours)",
			base:     []Record{rec("a", f("v", 1))},
			ours:     []Record{rec("a", f("v", 2))},
			their:    []Record{rec("a", f("v", 1))},
			wantKeys: []string{"a"},
		},
		{
			name:     "one-sided edit (theirs)",
			base:     []Record{rec("a", f("v", 1))},
			ours:     []Record{rec("a", f("v", 1))},
			their:    []Record{rec("a", f("v", 3))},
			wantKeys: []string{"a"},
		},
		{
			name:     "converging whole-record edit",
			base:     []Record{rec("a", f("v", 1))},
			ours:     []Record{rec("a", f("v", 5))},
			their:    []Record{rec("a", f("v", 5))},
			wantKeys: []string{"a"},
		},
		{
			name:         "DM-9 same-record different fields escalates when disabled",
			base:         []Record{rec("a", f("name", "x", "email", "e"))},
			ours:         []Record{rec("a", f("name", "y", "email", "e"))},
			their:        []Record{rec("a", f("name", "x", "email", "z"))},
			opts:         Options{SameRecord: false},
			wantEscalate: true,
		},
		{
			name:     "DM-9 same-record different fields merged when enabled",
			base:     []Record{rec("a", f("name", "x", "email", "e"))},
			ours:     []Record{rec("a", f("name", "y", "email", "e"))},
			their:    []Record{rec("a", f("name", "x", "email", "z"))},
			opts:     Options{SameRecord: true},
			wantKeys: []string{"a"},
		},
		{
			name:         "DM-13 contested field escalates even when enabled",
			base:         []Record{rec("a", f("name", "x"))},
			ours:         []Record{rec("a", f("name", "y"))},
			their:        []Record{rec("a", f("name", "z"))},
			opts:         Options{SameRecord: true},
			wantEscalate: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := Merge(tt.base, tt.ours, tt.their, tt.opts)
			if got.Escalate != tt.wantEscalate {
				t.Fatalf("Escalate = %v (reason %q), want %v", got.Escalate, got.Reason, tt.wantEscalate)
			}
			if tt.wantEscalate {
				if got.Reason == "" {
					t.Fatalf("escalated outcome must carry a reason")
				}
				if got.Merged != nil {
					t.Fatalf("escalated outcome must not carry merged records")
				}
				return
			}
			if gotKeys := keys(got.Merged); !equalStrings(gotKeys, tt.wantKeys) {
				t.Fatalf("merged keys = %v, want %v", gotKeys, tt.wantKeys)
			}
		})
	}
}

func TestMerge_DM9_MergesBothFieldChanges(t *testing.T) {
	t.Parallel()
	base := []Record{rec("a", f("name", "x", "email", "e"))}
	ours := []Record{rec("a", f("name", "y", "email", "e"))}
	their := []Record{rec("a", f("name", "x", "email", "z"))}

	got := Merge(base, ours, their, Options{SameRecord: true})
	if got.Escalate {
		t.Fatalf("unexpected escalate: %s", got.Reason)
	}
	fields, ok := find(got.Merged, "a")
	if !ok {
		t.Fatalf("record a missing")
	}
	if fields["name"] != "y" {
		t.Errorf("name = %v, want y", fields["name"])
	}
	if fields["email"] != "z" {
		t.Errorf("email = %v, want z", fields["email"])
	}
}

func TestMerge_OrderingOursBeforeTheirs(t *testing.T) {
	t.Parallel()
	base := []Record{rec("base1", f("v", 0))}
	ours := []Record{rec("base1", f("v", 0)), rec("o1", f("v", 1)), rec("o2", f("v", 2))}
	their := []Record{rec("base1", f("v", 0)), rec("t1", f("v", 3))}

	got := Merge(base, ours, their, Options{})
	want := []string{"base1", "o1", "o2", "t1"}
	if gotKeys := keys(got.Merged); !equalStrings(gotKeys, want) {
		t.Fatalf("order = %v, want %v", gotKeys, want)
	}
}

func TestMergeFields_FieldLevelCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		base, ours, their map[string]any
		wantOK            bool
		want              map[string]any
	}{
		{
			name:   "DM-10 disjoint new fields unioned",
			base:   f("id", 1),
			ours:   f("id", 1, "a", "ours"),
			their:  f("id", 1, "b", "theirs"),
			wantOK: true,
			want:   f("id", 1, "a", "ours", "b", "theirs"),
		},
		{
			name:   "DM-11 ours edits a field, theirs adds another",
			base:   f("a", 1),
			ours:   f("a", 2),
			their:  f("a", 1, "b", 9),
			wantOK: true,
			want:   f("a", 2, "b", 9),
		},
		{
			name:   "converging same new field same value",
			base:   f("a", 1),
			ours:   f("a", 1, "b", 5),
			their:  f("a", 1, "b", 5),
			wantOK: true,
			want:   f("a", 1, "b", 5),
		},
		{
			name:   "DM-14 same new field different value contested",
			base:   f("a", 1),
			ours:   f("a", 1, "b", 5),
			their:  f("a", 1, "b", 6),
			wantOK: false,
		},
		{
			name:   "both delete the same field",
			base:   f("a", 1, "b", 2),
			ours:   f("a", 1),
			their:  f("a", 1),
			wantOK: true,
			want:   f("a", 1),
		},
		{
			name:   "ours deletes a field, theirs unchanged",
			base:   f("a", 1, "b", 2),
			ours:   f("a", 1),
			their:  f("a", 1, "b", 2),
			wantOK: true,
			want:   f("a", 1),
		},
		{
			name:   "theirs deletes a field, ours unchanged",
			base:   f("a", 1, "b", 2),
			ours:   f("a", 1, "b", 2),
			their:  f("a", 1),
			wantOK: true,
			want:   f("a", 1),
		},
		{
			name:   "field delete/modify contested (ours deletes, theirs modifies)",
			base:   f("a", 1, "b", 2),
			ours:   f("a", 1),
			their:  f("a", 1, "b", 7),
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := mergeFields(tt.base, tt.ours, tt.their)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !tt.wantOK {
				return
			}
			if !fieldsEqual(got, tt.want) {
				t.Fatalf("merged fields = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValuesEqual_StrictKindAware(t *testing.T) {
	t.Parallel()
	if valuesEqual(1, "1") {
		t.Error("int 1 must not equal string \"1\"")
	}
	if valuesEqual(1, 1.0) {
		t.Error("int 1 must not equal float 1.0 (strict kind-aware)")
	}
	if !valuesEqual(map[string]any{"a": 1, "b": 2}, map[string]any{"b": 2, "a": 1}) {
		t.Error("maps differing only in key order must be equal")
	}
	if !valuesEqual([]any{1, 2}, []any{1, 2}) {
		t.Error("identical slices must be equal")
	}
	if valuesEqual([]any{1, 2}, []any{2, 1}) {
		t.Error("reordered slices must not be equal (lists are ordered)")
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

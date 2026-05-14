package sqlflags

import (
	"strings"
	"testing"
)

func TestRejectUnusedFlags_SelectMode(t *testing.T) {
	t.Parallel()
	// In single-record mode, select rejects --where, --order-by, --limit,
	// --min-affected. Test the shared subset here (--limit is verb-local).
	err := RejectSetModeFlags(SetModeFlags{
		WhereSupplied:       true,
		AllSupplied:         false,
		MinAffectedSupplied: false,
	}, ModeID)
	if err == nil {
		t.Fatal("expected error when --where supplied in ModeID")
	}
	if !strings.Contains(err.Error(), "--where") {
		t.Errorf("error should name --where, got: %v", err)
	}
}

func TestRejectSetUnsetSameField(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		set      []Assignment
		unset    []string
		wantErr  bool
		errField string
	}{
		{name: "no conflict", set: []Assignment{{Field: "name"}}, unset: []string{"draft"}},
		{name: "conflict", set: []Assignment{{Field: "active"}}, unset: []string{"active"}, wantErr: true, errField: "active"},
		{name: "conflict mid-list", set: []Assignment{{Field: "a"}, {Field: "b"}}, unset: []string{"x", "b"}, wantErr: true, errField: "b"},
		{name: "empty both", set: nil, unset: nil},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := RejectSetUnsetSameField(tt.set, tt.unset)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error")
					return
				}
				if !strings.Contains(err.Error(), tt.errField) {
					t.Errorf("error should name field %q, got: %v", tt.errField, err)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestRejectAllWithWhere(t *testing.T) {
	t.Parallel()
	// --all and --where are mutually exclusive in set mode.
	err := RejectSetModeFlags(SetModeFlags{
		WhereSupplied: true,
		AllSupplied:   true,
	}, ModeFrom)
	if err == nil {
		t.Fatal("expected error when --all and --where both supplied")
	}
}

func TestRejectSetModeFlagsRequireOneOf(t *testing.T) {
	t.Parallel()
	// In set mode (ModeFrom), neither --where nor --all → rejected.
	err := RejectSetModeFlags(SetModeFlags{
		WhereSupplied: false,
		AllSupplied:   false,
	}, ModeFrom)
	if err == nil {
		t.Fatal("expected error when neither --where nor --all supplied in set mode")
	}
}

func TestRejectSetModeFlags_AllInModeID(t *testing.T) {
	t.Parallel()
	err := RejectSetModeFlags(SetModeFlags{AllSupplied: true}, ModeID)
	if err == nil {
		t.Fatal("expected error when --all supplied in ModeID")
	}
	if !strings.Contains(err.Error(), "--all") {
		t.Errorf("error should name --all, got: %v", err)
	}
}

func TestRejectSetModeFlags_MinAffectedInModeID(t *testing.T) {
	t.Parallel()
	err := RejectSetModeFlags(SetModeFlags{MinAffectedSupplied: true}, ModeID)
	if err == nil {
		t.Fatal("expected error when --min-affected supplied in ModeID")
	}
	if !strings.Contains(err.Error(), "--min-affected") {
		t.Errorf("error should name --min-affected, got: %v", err)
	}
}

func TestRejectSetModeFlags_UnknownMode(t *testing.T) {
	t.Parallel()
	err := RejectSetModeFlags(SetModeFlags{}, Mode(99))
	if err == nil {
		t.Fatal("expected error for unknown mode")
	}
}

func TestRejectSetModeFlags_ModeFrom_WhereOnly(t *testing.T) {
	t.Parallel()
	err := RejectSetModeFlags(SetModeFlags{WhereSupplied: true}, ModeFrom)
	if err != nil {
		t.Errorf("unexpected error with --where in ModeFrom: %v", err)
	}
}

func TestRejectSetModeFlags_ModeFrom_AllOnly(t *testing.T) {
	t.Parallel()
	err := RejectSetModeFlags(SetModeFlags{AllSupplied: true}, ModeFrom)
	if err != nil {
		t.Errorf("unexpected error with --all in ModeFrom: %v", err)
	}
}

package sqlflags

// specscore: feature/shared-cli-flags

import "fmt"

// SetModeFlags carries the boolean presence of set-mode-only flags for
// applicability checking. Verb-specific flags (--limit, --fields)
// remain the verb's concern; this helper covers only the shared
// shape governed by shared-cli-flags.
type SetModeFlags struct {
	WhereSupplied       bool
	AllSupplied         bool
	MinAffectedSupplied bool
}

// RejectSetModeFlags enforces the cross-flag rules that depend on the
// resolved Mode.
//
// In ModeID (single-record): --where, --all, and --min-affected MUST
// all be absent.
//
// In ModeFrom (set): exactly one of --where or --all MUST be supplied;
// neither and both are rejected. --min-affected is unconstrained at
// this layer (it has its own validation in ParseMinAffected and its
// own applicability rule against ModeID).
func RejectSetModeFlags(f SetModeFlags, mode Mode) error {
	switch mode {
	case ModeID:
		if f.WhereSupplied {
			return fmt.Errorf("--where is invalid with --id (single-record mode); use --from for set queries")
		}
		if f.AllSupplied {
			return fmt.Errorf("--all is invalid with --id (single-record mode)")
		}
		if f.MinAffectedSupplied {
			return fmt.Errorf("--min-affected is invalid with --id (single-record mode)")
		}
		return nil
	case ModeFrom:
		if !f.WhereSupplied && !f.AllSupplied {
			return fmt.Errorf("set mode requires one of --where or --all")
		}
		if f.WhereSupplied && f.AllSupplied {
			return fmt.Errorf("--where and --all are mutually exclusive")
		}
		return nil
	default:
		return fmt.Errorf("invalid mode")
	}
}

// RejectSetUnsetSameField enforces that no field name appears in both
// --set and --unset within the same invocation.
func RejectSetUnsetSameField(sets []Assignment, unsets []string) error {
	if len(sets) == 0 || len(unsets) == 0 {
		return nil
	}
	unsetIndex := make(map[string]struct{}, len(unsets))
	for _, name := range unsets {
		unsetIndex[name] = struct{}{}
	}
	for _, a := range sets {
		if _, conflict := unsetIndex[a.Field]; conflict {
			return fmt.Errorf("field %q appears in both --set and --unset; use one or the other", a.Field)
		}
	}
	return nil
}

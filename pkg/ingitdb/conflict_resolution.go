// specscore: feature/cli/resolve/auto-resolve/record-merge
package ingitdb

// RecordMergeConfig configures the record-aware auto-merge of data-row
// conflicts. Both fields are pointers so an explicit `false` at a narrower
// scope (e.g. a collection) can override an inherited `true` — a nil value
// means "inherit".
type RecordMergeConfig struct {
	// Enabled turns the default disjoint/non-divergent auto-merge on or off.
	// Defaults to true when unset at every scope.
	Enabled *bool `yaml:"enabled,omitempty"`
	// SameRecord enables opt-in merging of non-contested changes to the same
	// record. Defaults to false when unset at every scope.
	SameRecord *bool `yaml:"same_record,omitempty"`
}

// ConflictResolutionConfig groups conflict-resolution settings. It is set at
// the database level (in .ingitdb settings) and may be overridden per
// collection (in the collection's definition).
type ConflictResolutionConfig struct {
	RecordMerge *RecordMergeConfig `yaml:"record_merge,omitempty"`
}

// EffectiveRecordMerge is the resolved record-merge configuration after the
// database default and any per-collection override are applied.
type EffectiveRecordMerge struct {
	Enabled    bool
	SameRecord bool
}

// ResolveRecordMerge computes the effective record-merge configuration for a
// collection: app defaults (enabled, not same-record) overlaid by the
// database-level config, then the per-collection override. Either argument may
// be nil.
func ResolveRecordMerge(def *Definition, col *CollectionDef) EffectiveRecordMerge {
	eff := EffectiveRecordMerge{Enabled: true, SameRecord: false}

	apply := func(c *ConflictResolutionConfig) {
		if c == nil || c.RecordMerge == nil {
			return
		}
		if c.RecordMerge.Enabled != nil {
			eff.Enabled = *c.RecordMerge.Enabled
		}
		if c.RecordMerge.SameRecord != nil {
			eff.SameRecord = *c.RecordMerge.SameRecord
		}
	}

	if def != nil {
		apply(def.Settings.ConflictResolution)
	}
	if col != nil {
		apply(col.ConflictResolution)
	}
	return eff
}

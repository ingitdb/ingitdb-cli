// Package recordmerge implements the record-aware three-way merge engine for
// auto-resolving logically non-conflicting data-row conflicts.
//
// specscore: feature/cli/resolve/auto-resolve/record-merge
package recordmerge

import "reflect"

// Record is one data row: a primary key plus its parsed, typed fields.
type Record struct {
	Key    string
	Fields map[string]any
}

// Options controls which case classes the engine is allowed to auto-merge.
type Options struct {
	// SameRecord enables merging non-contested changes to the same record
	// (cases DM-9..DM-11). When false, any record changed on both sides
	// escalates.
	SameRecord bool
}

// Outcome is the result of a merge attempt. When Escalate is true the conflict
// is not auto-resolvable and Reason explains why; Merged is then nil.
type Outcome struct {
	Merged   []Record
	Escalate bool
	Reason   string
}

// valuesEqual reports strict, kind-aware equality of two parsed values. No
// cross-kind coercion is performed (e.g. 1 != "1", 1 != 1.0), so it absorbs
// representation noise (key order, whitespace, quoting) — which parses to
// identical typed values — without ever declaring differently-typed values
// equal.
func valuesEqual(a, b any) bool {
	return reflect.DeepEqual(a, b)
}

// fieldsEqual reports whether two records' field maps are strictly equal.
func fieldsEqual(a, b map[string]any) bool {
	return reflect.DeepEqual(a, b)
}

// index builds a key→fields lookup for a record slice.
func index(records []Record) map[string]map[string]any {
	m := make(map[string]map[string]any, len(records))
	for _, r := range records {
		m[r.Key] = r.Fields
	}
	return m
}

// Merge performs a three-way merge of keyed record sets. base/ours/theirs are
// the records parsed from the common ancestor and the two conflict sides.
//
// It returns Escalate=true (with a reason) for any conflict that is not
// provably safe to auto-resolve; otherwise Merged holds the union, ordered as
// surviving base records first (in base order), then records added by ours,
// then records added by theirs.
func Merge(base, ours, theirs []Record, opts Options) Outcome {
	baseIdx := index(base)
	oursIdx := index(ours)
	theirsIdx := index(theirs)

	merged := make(map[string]map[string]any)

	for key := range union(baseIdx, oursIdx, theirsIdx) {
		bf, inBase := baseIdx[key]
		of, inOurs := oursIdx[key]
		tf, inTheirs := theirsIdx[key]

		switch {
		case !inBase:
			// Added on one or both sides (no ancestor).
			switch {
			case inOurs && inTheirs:
				if fieldsEqual(of, tf) {
					merged[key] = of // DM-2: identical addition deduplicated
				} else {
					return escalate("primary-key collision on %q: both sides added different content", key)
				}
			case inOurs:
				merged[key] = of // DM-1: ours-only addition
			default:
				merged[key] = tf // DM-1: theirs-only addition
			}
		default:
			// Existed in base; reason about modify/delete on each side.
			oursDeleted := !inOurs
			theirsDeleted := !inTheirs
			oursChanged := inOurs && !fieldsEqual(of, bf)
			theirsChanged := inTheirs && !fieldsEqual(tf, bf)

			switch {
			case oursDeleted && theirsDeleted:
				// DM-4: deleted on both sides; omit.
			case oursDeleted:
				if theirsChanged {
					return escalate("delete/modify on %q: deleted by one side, modified by the other", key)
				}
				// theirs unchanged → honor deletion; omit.
			case theirsDeleted:
				if oursChanged {
					return escalate("delete/modify on %q: deleted by one side, modified by the other", key)
				}
				// ours unchanged → honor deletion; omit.
			case !oursChanged && !theirsChanged:
				merged[key] = bf // unchanged on both sides
			case oursChanged && !theirsChanged:
				merged[key] = of // one-sided edit
			case theirsChanged && !oursChanged:
				merged[key] = tf // one-sided edit
			default:
				// Both sides changed the same record.
				if fieldsEqual(of, tf) {
					merged[key] = of // converging whole-record edit
					continue
				}
				if !opts.SameRecord {
					return escalate("same record %q changed on both sides and same-record merge is disabled", key)
				}
				rec, ok := mergeFields(bf, of, tf)
				if !ok {
					return escalate("contested field in record %q: same field set to different values", key)
				}
				merged[key] = rec
			}
		}
	}

	return Outcome{Merged: orderedRecords(base, ours, theirs, merged)}
}

// mergeFields merges field-level changes for a record modified on both sides.
// It returns ok=false when any single field is contested (changed on both
// sides to different values, including divergent add/add and delete/modify).
func mergeFields(base, ours, theirs map[string]any) (map[string]any, bool) {
	result := make(map[string]any)

	for field := range unionFields(base, ours, theirs) {
		bv, inBase := base[field]
		ov, inOurs := ours[field]
		tv, inTheirs := theirs[field]

		oursChanged := inOurs != inBase || (inOurs && !valuesEqual(ov, bv))
		theirsChanged := inTheirs != inBase || (inTheirs && !valuesEqual(tv, bv))

		switch {
		case oursChanged && theirsChanged:
			// Both touched this field: only safe if they converge.
			if inOurs != inTheirs || (inOurs && !valuesEqual(ov, tv)) {
				return nil, false
			}
			if inOurs {
				result[field] = ov
			}
		case oursChanged:
			if inOurs {
				result[field] = ov
			}
		case theirsChanged:
			if inTheirs {
				result[field] = tv
			}
		default:
			// Unchanged on both sides; reachable only when the field is in
			// base (otherwise one side would register as changed).
			result[field] = bv
		}
	}

	return result, true
}

func escalate(format string, args ...any) Outcome {
	return Outcome{Escalate: true, Reason: sprintf(format, args...)}
}

// specscore: feature/cli/resolve/auto-resolve/record-merge
package recordmerge

import "fmt"

// union returns the set of keys present in any of the given indexes.
func union(indexes ...map[string]map[string]any) map[string]struct{} {
	keys := make(map[string]struct{})
	for _, idx := range indexes {
		for k := range idx {
			keys[k] = struct{}{}
		}
	}
	return keys
}

// unionFields returns the set of field names present in any of the given maps.
func unionFields(maps ...map[string]any) map[string]struct{} {
	keys := make(map[string]struct{})
	for _, m := range maps {
		for k := range m {
			keys[k] = struct{}{}
		}
	}
	return keys
}

// orderedRecords renders the surviving merged keys into a deterministic slice:
// surviving base records first (in base order), then keys added by ours, then
// keys added by theirs — each in their original slice order, never duplicated.
func orderedRecords(base, ours, theirs []Record, merged map[string]map[string]any) []Record {
	result := make([]Record, 0, len(merged))
	emitted := make(map[string]struct{}, len(merged))

	emit := func(key string) {
		if _, done := emitted[key]; done {
			return
		}
		fields, survives := merged[key]
		if !survives {
			return
		}
		emitted[key] = struct{}{}
		result = append(result, Record{Key: key, Fields: fields})
	}

	for _, r := range base {
		emit(r.Key)
	}
	for _, r := range ours {
		emit(r.Key)
	}
	for _, r := range theirs {
		emit(r.Key)
	}
	return result
}

// sprintf is a thin alias so callers read clearly at the use site.
func sprintf(format string, args ...any) string {
	return fmt.Sprintf(format, args...)
}

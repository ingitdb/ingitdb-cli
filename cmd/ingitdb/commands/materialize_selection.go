package commands

// specscore: feature/cli/materialize

import (
	"path"
	"strings"
)

// selectionKind describes the tri-state of a --collections / --views flag.
type selectionKind int

const (
	// selectionNone means the flag was absent.
	selectionNone selectionKind = iota
	// selectionAll means the flag was supplied bare (all artifacts of that type).
	selectionAll
	// selectionList means the flag was supplied with a glob list.
	selectionList
)

// selection is the resolved tri-state of a selector flag.
type selection struct {
	kind  selectionKind
	globs []string
}

// flagSelection resolves a selector flag into a selection from its Changed state
// and raw value. A raw value equal to materializeAllSentinel means "all".
func flagSelection(changed bool, raw string) selection {
	if !changed {
		return selection{kind: selectionNone}
	}
	if raw == materializeAllSentinel {
		return selection{kind: selectionAll}
	}
	return selection{kind: selectionList, globs: splitPatterns(raw)}
}

// splitPatterns splits a glob list on ',' and ';', trimming whitespace and
// dropping empty entries. Returns nil for an empty input.
func splitPatterns(raw string) []string {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';'
	})
	var out []string
	for _, f := range fields {
		trimmed := strings.TrimSpace(f)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// matchViewNames returns the subset of names matching any of the glob patterns,
// preserving the order of names and de-duplicating. Glob semantics are those of
// path.Match (e.g. '*' matches any sequence within a segment); an exact name
// also matches itself.
func matchViewNames(names []string, patterns []string) []string {
	var out []string
	for _, name := range names {
		if !viewNameMatchesAny(name, patterns) {
			continue
		}
		out = append(out, name)
	}
	return out
}

func viewNameMatchesAny(name string, patterns []string) bool {
	for _, pattern := range patterns {
		if pattern == name {
			return true
		}
		matched, err := path.Match(pattern, name)
		if err == nil && matched {
			return true
		}
	}
	return false
}

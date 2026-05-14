package sqlflags

// specscore: feature/shared-cli-flags

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Assignment is one parsed --set expression.
type Assignment struct {
	Field string
	Value any
}

// rejectedSetOperators are the operators that must not appear between
// field and value in a --set expression. Order matters: longer prefixes
// first so we detect "===" before "==".
var rejectedSetOperators = []string{"===", "!==", "==", "!=", ">=", "<=", ">", "<"}

// ParseSet parses one --set expression: `field=value`.
// Comparison operators between field and value are rejected.
func ParseSet(s string) (Assignment, error) {
	if s == "" {
		return Assignment{}, fmt.Errorf("empty --set expression")
	}
	idx := strings.Index(s, "=")
	if idx < 0 {
		return Assignment{}, fmt.Errorf("missing '=' in --set expression %q", s)
	}
	field := strings.TrimSpace(s[:idx])
	if field == "" {
		return Assignment{}, fmt.Errorf("missing field name in --set expression %q", s)
	}
	for _, op := range rejectedSetOperators {
		opIdx := strings.Index(s, op)
		if opIdx >= 0 && opIdx <= idx {
			return Assignment{}, fmt.Errorf("--set requires single '='; found %q in %q", op, s)
		}
	}
	rawVal := s[idx+1:]
	val, parseErr := parseYAMLScalar(rawVal)
	if parseErr != nil {
		return Assignment{}, fmt.Errorf("invalid --set value in %q: %w", s, parseErr)
	}
	return Assignment{Field: field, Value: val}, nil
}

// parseYAMLScalar performs YAML 1.2 scalar inference on raw, returning
// the typed Go value. An empty string is returned as "".
func parseYAMLScalar(raw string) (any, error) {
	if raw == "" {
		return "", nil
	}
	var out any
	if err := yaml.Unmarshal([]byte(raw), &out); err != nil {
		return nil, err
	}
	return out, nil
}

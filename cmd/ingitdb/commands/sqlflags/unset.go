package sqlflags

import (
	"fmt"
	"strings"
)

// ParseUnset parses a comma-separated --unset field list.
// Each field must be non-empty, contain no '=', and contain no
// whitespace inside the name.
func ParseUnset(s string) ([]string, error) {
	if s == "" {
		return nil, fmt.Errorf("empty --unset value")
	}
	parts := strings.Split(s, ",")
	fields := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			return nil, fmt.Errorf("empty field in --unset %q (check for stray commas)", s)
		}
		if strings.Contains(trimmed, "=") {
			return nil, fmt.Errorf("--unset field %q must not contain '='", trimmed)
		}
		if strings.ContainsAny(trimmed, " \t\n") {
			return nil, fmt.Errorf("--unset field %q must not contain whitespace", trimmed)
		}
		fields = append(fields, trimmed)
	}
	return fields, nil
}

package sqlflags

// specscore: feature/shared-cli-flags

import (
	"fmt"
	"strings"
)

// ParseFields parses --fields. Returns nil for "*" or empty (meaning
// "all fields"). Otherwise returns the trimmed comma-separated list,
// preserving order. Empty entries (stray commas) are rejected for
// consistency with ParseOrderBy and ParseUnset.
func ParseFields(s string) ([]string, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "*" {
		return nil, nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			return nil, fmt.Errorf("empty --fields entry in %q (check for stray commas)", s)
		}
		out = append(out, p)
	}
	return out, nil
}

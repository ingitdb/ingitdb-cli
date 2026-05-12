package sqlflags

import (
	"fmt"
	"strings"
)

// OrderTerm is one parsed --order-by entry.
type OrderTerm struct {
	Field      string
	Descending bool
}

// ParseOrderBy parses a comma-separated --order-by list. A leading '-'
// indicates descending order for that field. An empty input returns
// nil with no error.
func ParseOrderBy(s string) ([]OrderTerm, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	parts := strings.Split(s, ",")
	terms := make([]OrderTerm, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			return nil, fmt.Errorf("empty --order-by entry in %q (check for stray commas)", s)
		}
		desc := false
		if strings.HasPrefix(trimmed, "-") {
			desc = true
			trimmed = strings.TrimSpace(trimmed[1:])
			if trimmed == "" {
				return nil, fmt.Errorf("empty field after '-' in --order-by %q", s)
			}
		}
		terms = append(terms, OrderTerm{Field: trimmed, Descending: desc})
	}
	return terms, nil
}

package sqlflags

import "strings"

// ParseFields parses --fields. Returns nil for "*" or empty (meaning
// "all fields"). Otherwise returns the trimmed comma-separated list,
// preserving order.
func ParseFields(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" || s == "*" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

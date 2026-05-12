package sqlflags

import (
	"fmt"
	"strconv"
)

// ParseMinAffected parses --min-affected=N into a positive integer.
// N must be >= 1; zero and negative values are rejected.
func ParseMinAffected(s string) (int, error) {
	if s == "" {
		return 0, fmt.Errorf("--min-affected value is required")
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("--min-affected %q is not an integer: %w", s, err)
	}
	if n < 1 {
		return 0, fmt.Errorf("--min-affected must be >= 1, got %d", n)
	}
	return n, nil
}

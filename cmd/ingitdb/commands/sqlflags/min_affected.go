package sqlflags

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
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

// MinAffectedFromCmd reads --min-affected from cmd, returning the
// parsed value, a "supplied" boolean, and any error. When the flag is
// not supplied, returns (0, false, nil). When supplied with N >= 1,
// returns (N, true, nil). When supplied with N < 1, returns an error.
//
// Verb commands should prefer this helper over GetInt because it
// distinguishes "not supplied" (no threshold check) from "explicit 0"
// (which is rejected by the spec).
func MinAffectedFromCmd(cmd *cobra.Command) (int, bool, error) {
	if !cmd.Flags().Changed("min-affected") {
		return 0, false, nil
	}
	n, err := cmd.Flags().GetInt("min-affected")
	if err != nil {
		return 0, false, fmt.Errorf("--min-affected: %w", err)
	}
	if n < 1 {
		return 0, false, fmt.Errorf("--min-affected must be >= 1, got %d", n)
	}
	return n, true, nil
}

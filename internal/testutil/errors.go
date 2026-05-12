// Package testutil provides shared test helpers for ingitdb-cli tests.
//
// The package contains no production code. It is intentionally placed under
// `internal/` so it can only be imported by other packages in this module,
// and its functions accept `*testing.T` so they can only be called from
// test code.
package testutil

import (
	"strings"
	"testing"
)

// MustErrContain asserts that err is non-nil and its message contains every
// substring in substrs. It reports failure via t.Fatalf. t.Helper() makes the
// failure point at the test that called MustErrContain rather than this line.
//
// Example:
//
//	_, err := parseInput(badInput)
//	testutil.MustErrContain(t, err, "$content", "collide")
func MustErrContain(t testing.TB, err error, substrs ...string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	msg := err.Error()
	for _, s := range substrs {
		if !strings.Contains(msg, s) {
			t.Fatalf("error %q does not contain %q", msg, s)
		}
	}
}

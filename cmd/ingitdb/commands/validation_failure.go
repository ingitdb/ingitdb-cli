package commands

import (
	"errors"
	"fmt"
)

// ValidationFailedExitCode is returned when the validate command completed and
// found that the repository does not satisfy its InGitDB definition.
const ValidationFailedExitCode = 2

// ErrValidationFailed identifies repository validation findings separately
// from command configuration, I/O, startup, and other runtime failures.
var ErrValidationFailed = errors.New("repository validation failed")

// NewValidationFailedError preserves a human-readable finding while making it
// possible for the process boundary to select ValidationFailedExitCode.
func NewValidationFailedError(err error) error {
	if err == nil {
		return ErrValidationFailed
	}
	return fmt.Errorf("%w: %v", ErrValidationFailed, err)
}

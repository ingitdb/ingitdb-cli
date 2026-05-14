package sqlflags

// specscore: feature/shared-cli-flags

import "fmt"

// Mode is the verb operating mode.
type Mode int

const (
	ModeInvalid Mode = iota
	ModeID           // single-record (--id supplied)
	ModeFrom         // set (--from supplied)
)

// ResolveMode returns the operating mode for a verb based on its --id
// and --from flag values. Empty string means "not supplied".
// Supplying both or neither is rejected.
func ResolveMode(idFlag, fromFlag string) (Mode, error) {
	hasID := idFlag != ""
	hasFrom := fromFlag != ""
	switch {
	case hasID && hasFrom:
		return ModeInvalid, fmt.Errorf("--id and --from are mutually exclusive; supply exactly one")
	case hasID:
		return ModeID, nil
	case hasFrom:
		return ModeFrom, nil
	default:
		return ModeInvalid, fmt.Errorf("one of --id or --from is required")
	}
}

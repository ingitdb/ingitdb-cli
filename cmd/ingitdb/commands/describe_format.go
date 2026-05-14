package commands

// specscore: feature/cli/describe

import (
	"fmt"
	"strings"
)

const (
	// engineIngitDB is the engine identifier the CLI passes when the
	// describe command runs against an ingitdb project. resolveFormat
	// uses it to determine that "sql" is not a supported native output.
	engineIngitDB = "ingitdb"
)

// resolveFormat normalises the user-supplied --format value into one of
// the canonical output formats {yaml, json, sql}.
//
//   - empty   → "yaml" (the documented default)
//   - "yaml"  → "yaml"
//   - "json"  → "json"
//   - "native"→ the engine's canonical format ("yaml" for ingitdb;
//     "sql" for any non-ingitdb engine)
//   - "sql"/"SQL" → routes to native; for the ingitdb engine this is
//     reported as an error so the caller surfaces it to the user.
//
// Any other value produces an error listing the accepted values.
func resolveFormat(raw, engine string) (string, error) {
	switch strings.ToLower(raw) {
	case "":
		return "yaml", nil
	case "yaml":
		return "yaml", nil
	case "json":
		return "json", nil
	case "native":
		if engine == engineIngitDB {
			return "yaml", nil
		}
		return "sql", nil
	case "sql":
		if engine == engineIngitDB {
			return "", fmt.Errorf(`engine %q native format is "yaml"; use --format=yaml or --format=native`, engine)
		}
		return "sql", nil
	default:
		return "", fmt.Errorf("invalid --format value %q (valid values: yaml, json, native, sql)", raw)
	}
}

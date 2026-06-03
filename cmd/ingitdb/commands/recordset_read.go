package commands

// specscore: feature/computed-columns-via-dalgo

import (
	"github.com/dal-go/dalgo/recordset"

	"github.com/ingitdb/ingitdb-cli/cmd/ingitdb/commands/sqlflags"
)

// whereColumnNames returns the recordset columns referenced by --where
// conditions, excluding the "$id" pseudo-field and any name that is not an
// actual recordset column (an unknown field simply never matches, as before).
func whereColumnNames(rs recordset.Recordset, conds []sqlflags.Condition) []string {
	var names []string
	seen := make(map[string]bool)
	for _, c := range conds {
		if c.Field == "$id" || seen[c.Field] {
			continue
		}
		seen[c.Field] = true
		if rs.GetColumnByName(c.Field) != nil {
			names = append(names, c.Field)
		}
	}
	return names
}

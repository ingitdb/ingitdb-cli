package docsbuilder

import (
	"fmt"
	"strings"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// BuildViewHeader generates a human-readable header string based on ViewDef parameters.
func BuildViewHeader(view *ingitdb.ViewDef) string {
	var sb strings.Builder

	if view.Top > 0 {
		fmt.Fprintf(&sb, "Top %d records", view.Top)
	} else {
		sb.WriteString("All records")
	}

	if view.Where != "" {
		fmt.Fprintf(&sb, " where %s", view.Where)
	}

	if view.OrderBy != "" {
		fmt.Fprintf(&sb, " ordered by %s", view.OrderBy)
	}

	return sb.String()
}

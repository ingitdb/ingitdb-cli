package docsbuilder

import (
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func TestBuildCollectionReadme(t *testing.T) {
	col := &ingitdb.CollectionDef{
		ID:     "teams",
		Titles: map[string]string{"en": "Agile Teams"},
		Columns: map[string]*ingitdb.ColumnDef{
			"id": {
				Type:     ingitdb.ColumnTypeString,
				Required: true,
			},
			"name": {
				Type:     ingitdb.ColumnTypeString,
				Required: true,
				Locale:   "en",
			},
			"department_id": {
				Type:       ingitdb.ColumnTypeString,
				ForeignKey: "departments",
			},
		},
		ColumnsOrder: []string{"id", "name", "department_id"},
		SubCollections: map[string]*ingitdb.CollectionDef{
			"members": {ID: "members"},
		},
		Views: map[string]*ingitdb.ViewDef{
			"active_teams": {
				ID:      "active_teams",
				Columns: []string{"id", "name"},
			},
		},
	}

	def := &ingitdb.Definition{}

	content, err := BuildCollectionReadme(col, def)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedSections := []string{
		"# Agile Teams",
		"## Columns",
		"| id | string | Required |",
		"| name | string | Required, Locale(en) |",
		"| department_id | string | FK(departments) |",
		"## Subcollections",
		"| [members](members) | 0 |",
		"## Views",
		"| active_teams | 2 |",
	}

	for _, expected := range expectedSections {
		if !strings.Contains(content, expected) {
			t.Errorf("expected generated README to contain: %q\ngot:\n%s", expected, content)
		}
	}
}

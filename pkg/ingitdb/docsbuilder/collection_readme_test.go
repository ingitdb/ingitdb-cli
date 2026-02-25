package docsbuilder

import (
	"context"
	"fmt"
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

	content, err := BuildCollectionReadme(context.Background(), col, def, nil)
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

func TestBuildCollectionReadme_HideSectionsAndDataPreview(t *testing.T) {
	col := &ingitdb.CollectionDef{
		ID:     "teams",
		Titles: map[string]string{"en": "Agile Teams"},
		Columns: map[string]*ingitdb.ColumnDef{
			"id": {Type: ingitdb.ColumnTypeString},
		},
		SubCollections: map[string]*ingitdb.CollectionDef{
			"members": {ID: "members"},
		},
		Views: map[string]*ingitdb.ViewDef{
			"active_teams": {ID: "active_teams", Columns: []string{"id", "name"}},
		},
		Readme: &ingitdb.CollectionReadmeDef{
			HideColumns:        true,
			HideSubcollections: true,
			HideViews:          true,
			DataPreview: &ingitdb.ViewDef{
				Top: 5,
			},
		},
	}

	def := &ingitdb.Definition{}

	renderer := func(ctx context.Context, col *ingitdb.CollectionDef, view *ingitdb.ViewDef) (string, error) {
		return "| fake | data |\n| --- | --- |\n", nil
	}

	content, err := BuildCollectionReadme(context.Background(), col, def, renderer)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	unexpectedSections := []string{
		"## Columns",
		"## Subcollections",
		"## Views",
	}

	expectedSections := []string{
		"# Agile Teams",
		"## Data preview",
		"*Top 5 records*",
		"| fake | data |",
	}

	for _, unexpected := range unexpectedSections {
		if strings.Contains(content, unexpected) {
			t.Errorf("expected generated README NOT to contain: %q\ngot:\n%s", unexpected, content)
		}
	}

	for _, expected := range expectedSections {
		if !strings.Contains(content, expected) {
			t.Errorf("expected generated README to contain: %q\ngot:\n%s", expected, content)
		}
	}
}

func TestBuildCollectionReadme_DataPreviewError(t *testing.T) {
	col := &ingitdb.CollectionDef{
		ID: "teams",
		Readme: &ingitdb.CollectionReadmeDef{
			DataPreview: &ingitdb.ViewDef{Top: 5},
		},
	}
	def := &ingitdb.Definition{}
	renderer := func(ctx context.Context, col *ingitdb.CollectionDef, view *ingitdb.ViewDef) (string, error) {
		return "", fmt.Errorf("simulated render error")
	}

	_, err := BuildCollectionReadme(context.Background(), col, def, renderer)
	if err == nil {
		t.Fatalf("expected error from renderer to be propagated")
	}
	if !strings.Contains(err.Error(), "simulated render error") {
		t.Fatalf("expected error to contain 'simulated render error', got: %v", err)
	}
}

func TestBuildCollectionReadme_AlternativeBranches(t *testing.T) {
	col := &ingitdb.CollectionDef{
		ID:     "teams",
		Titles: map[string]string{"ru": "Команды"}, // no "en" title
		Columns: map[string]*ingitdb.ColumnDef{
			"id":   {Type: ingitdb.ColumnTypeString},
			"name": {Type: ingitdb.ColumnTypeString},
		},
		// no ColumnsOrder
		SubCollections: map[string]*ingitdb.CollectionDef{
			"members": {ID: "members", DirPath: "foo/members"},
		},
		DirPath: "foo", // provide DirPath to test relative path calculation
	}

	def := &ingitdb.Definition{}
	content, err := BuildCollectionReadme(context.Background(), col, def, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedSections := []string{
		"# Команды", // should pick the non-en title
		"| id | string | - |",
		"| name | string | - |",
		"| [members](members) | 0 |", // relative path
	}

	for _, expected := range expectedSections {
		if !strings.Contains(content, expected) {
			t.Errorf("expected generated README to contain: %q\ngot:\n%s", expected, content)
		}
	}
}

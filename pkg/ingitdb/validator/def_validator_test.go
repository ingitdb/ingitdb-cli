package validator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func TestReadDefinition(t *testing.T) {
	for _, tt := range []struct {
		name            string
		dir             string
		err             string
		wantCollections int
	}{
		{
			name:            "missing_root_config_file",
			dir:             ".",
			err:             "",
			wantCollections: 0,
		},
		{
			name:            "repo_root",
			dir:             "../../../",
			err:             "",
			wantCollections: -1, // any positive count is fine; only check no error
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			currentDir, err := os.Getwd()
			if err != nil {
				t.Fatalf("failed to get current dir: %s", err)
			}
			dbDirPath := filepath.Join(currentDir, tt.dir)
			def, err := ReadDefinition(dbDirPath, ingitdb.Validate())
			if err == nil && tt.err != "" {
				t.Fatal("got no error, expected: " + tt.err)
			}
			if tt.err == "" && err != nil {
				t.Fatal("expected no error, got: " + err.Error())
			}
			if tt.err != "" && err != nil && !strings.Contains(err.Error(), tt.err) {
				t.Fatalf("expected error to contain '%s', got '%s'", tt.err, err)
			}
			if tt.err == "" && def == nil {
				t.Fatalf("expected definition to be non-nil")
			}
			if tt.err == "" && len(def.Collections) != tt.wantCollections && tt.wantCollections >= 0 {
				t.Fatalf("expected %d collections, got %d", tt.wantCollections, len(def.Collections))
			}
		})
	}
}

func TestDefaultViewInjection(t *testing.T) {
	t.Parallel()

	// Create a temporary directory structure with default_view in the collection definition
	tmpDir := t.TempDir()

	// Create .ingitdb directory
	ingitdbDir := filepath.Join(tmpDir, ".ingitdb")
	if err := os.Mkdir(ingitdbDir, 0o755); err != nil {
		t.Fatalf("failed to create .ingitdb dir: %v", err)
	}

	// Create settings.yaml
	settingsYamlPath := filepath.Join(ingitdbDir, "settings.yaml")
	settingsYamlContent := `languages:
  - required: en
`
	if err := os.WriteFile(settingsYamlPath, []byte(settingsYamlContent), 0o644); err != nil {
		t.Fatalf("failed to write .ingitdb/settings.yaml: %v", err)
	}

	// Create root-collections.yaml
	rootCollectionsYamlPath := filepath.Join(ingitdbDir, "root-collections.yaml")
	rootCollectionsYamlContent := `articles: ./articles
`
	if err := os.WriteFile(rootCollectionsYamlPath, []byte(rootCollectionsYamlContent), 0o644); err != nil {
		t.Fatalf("failed to write .ingitdb/root-collections.yaml: %v", err)
	}

	// Create articles/.collection/definition.yaml with default_view
	articlesSchemaDir := filepath.Join(tmpDir, "articles", ".collection")
	if err := os.MkdirAll(articlesSchemaDir, 0o755); err != nil {
		t.Fatalf("failed to create articles schema dir: %v", err)
	}

	defYamlPath := filepath.Join(articlesSchemaDir, "definition.yaml")
	defYamlContent := `columns:
  id:
    type: string
  title:
    type: string
  content:
    type: string
columns_order:
  - id
  - title
  - content
record_file:
  type: json_array
  name: articles.json
data_dir: data
default_view:
  format: csv
  max_batch_size: 100
  order_by: id
`
	if err := os.WriteFile(defYamlPath, []byte(defYamlContent), 0o644); err != nil {
		t.Fatalf("failed to write definition.yaml: %v", err)
	}

	// Read the definition
	def, err := ReadDefinition(tmpDir)
	if err != nil {
		t.Fatalf("ReadDefinition failed: %v", err)
	}

	if len(def.Collections) != 1 {
		t.Fatalf("expected 1 collection, got %d", len(def.Collections))
	}

	col, ok := def.Collections["articles"]
	if !ok {
		t.Fatalf("expected 'articles' collection, got %v", def.Collections)
	}

	// Check that default_view was injected into Views
	if col.Views == nil {
		t.Fatalf("expected Views to be non-nil")
	}

	defaultView, ok := col.Views[ingitdb.DefaultViewID]
	if !ok {
		t.Fatalf("expected default_view (ID=%s) in Views, got %v", ingitdb.DefaultViewID, col.Views)
	}

	// Verify the default view properties
	if defaultView.ID != ingitdb.DefaultViewID {
		t.Errorf("expected view ID to be %s, got %s", ingitdb.DefaultViewID, defaultView.ID)
	}

	if !defaultView.IsDefault {
		t.Errorf("expected IsDefault to be true, got %v", defaultView.IsDefault)
	}

	if defaultView.Format != "csv" {
		t.Errorf("expected format to be 'csv', got %q", defaultView.Format)
	}

	if defaultView.MaxBatchSize != 100 {
		t.Errorf("expected max_batch_size to be 100, got %d", defaultView.MaxBatchSize)
	}

	if defaultView.OrderBy != "id" {
		t.Errorf("expected order_by to be 'id', got %q", defaultView.OrderBy)
	}

	// Verify that the original col.DefaultView still has the data
	if col.DefaultView == nil {
		t.Fatalf("expected col.DefaultView to be non-nil")
	}

	if col.DefaultView != defaultView {
		t.Errorf("expected col.DefaultView and Views[%s] to be the same", ingitdb.DefaultViewID)
	}
}

package validator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// TestNewCollectionsReader verifies that NewCollectionsReader returns
// a valid CollectionsReader implementation.
func TestNewCollectionsReader(t *testing.T) {
	t.Parallel()

	reader := NewCollectionsReader()
	if reader == nil {
		t.Fatal("NewCollectionsReader() returned nil")
	}

	// Verify it satisfies the interface
	var _ = reader
}

// TestDefinitionReader_ReadDefinition verifies the CollectionsReader interface
// implementation delegates correctly to the package-level ReadDefinition function.
func TestDefinitionReader_ReadDefinition(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		setupFn         func(t *testing.T) string
		opts            []ingitdb.ReadOption
		wantErr         string
		wantCollections int
	}{
		{
			name: "missing_root_config",
			setupFn: func(t *testing.T) string {
				return t.TempDir()
			},
			opts:            nil,
			wantErr:         "",
			wantCollections: 0,
		},
		{
			name: "valid_definition",
			setupFn: func(t *testing.T) string {
				root := t.TempDir()
				// Create .ingitdb/root-collections.yaml
				ingitDBDir := filepath.Join(root, ".ingitdb")
				err := os.MkdirAll(ingitDBDir, 0755)
				if err != nil {
					t.Fatalf("setup: create .ingitdb dir: %v", err)
				}
				rootConfigPath := filepath.Join(ingitDBDir, "root-collections.yaml")
				rootConfigContent := "test: test-ingitdb\n"
				err = os.WriteFile(rootConfigPath, []byte(rootConfigContent), 0644)
				if err != nil {
					t.Fatalf("setup: write root config: %v", err)
				}

				// Create collection definition
				collectionDir := filepath.Join(root, "test-ingitdb")
				schemaDir := filepath.Join(collectionDir, ingitdb.SchemaDir)
				err = os.MkdirAll(schemaDir, 0755)
				if err != nil {
					t.Fatalf("setup: create collection dir: %v", err)
				}

				collectionDefPath := filepath.Join(schemaDir, ingitdb.CollectionDefFileName)
				collectionDefContent := `record_file:
  name: "{key}.yaml"
  type: "map[string]any"
  format: yaml
columns:
  id:
    type: string
    primaryKey: true
  name:
    type: string
`
				err = os.WriteFile(collectionDefPath, []byte(collectionDefContent), 0644)
				if err != nil {
					t.Fatalf("setup: write collection def: %v", err)
				}

				return root
			},
			opts:    []ingitdb.ReadOption{ingitdb.Validate()},
			wantErr:         "",
			wantCollections: 1,
		},
		{
			name: "invalid_yaml_in_collection",
			setupFn: func(t *testing.T) string {
				root := t.TempDir()
				// Create .ingitdb/root-collections.yaml
				ingitDBDir := filepath.Join(root, ".ingitdb")
				err := os.MkdirAll(ingitDBDir, 0755)
				if err != nil {
					t.Fatalf("setup: create .ingitdb dir: %v", err)
				}
				rootConfigPath := filepath.Join(ingitDBDir, "root-collections.yaml")
				rootConfigContent := "bad: bad-collection\n"
				err = os.WriteFile(rootConfigPath, []byte(rootConfigContent), 0644)
				if err != nil {
					t.Fatalf("setup: write root config: %v", err)
				}

				// Create collection with invalid YAML
				collectionDir := filepath.Join(root, "bad-collection")
				schemaDir := filepath.Join(collectionDir, ingitdb.SchemaDir)
				err = os.MkdirAll(schemaDir, 0755)
				if err != nil {
					t.Fatalf("setup: create collection dir: %v", err)
				}

				collectionDefPath := filepath.Join(schemaDir, ingitdb.CollectionDefFileName)
				invalidYAML := "columns: [invalid yaml\n"
				err = os.WriteFile(collectionDefPath, []byte(invalidYAML), 0644)
				if err != nil {
					t.Fatalf("setup: write invalid collection def: %v", err)
				}

				return root
			},
			opts:    nil,
			wantErr: "failed to parse YAML file",
		},
		{
			name: "validation_enabled_with_invalid_schema",
			setupFn: func(t *testing.T) string {
				root := t.TempDir()
				// Create .ingitdb/root-collections.yaml
				ingitDBDir := filepath.Join(root, ".ingitdb")
				err := os.MkdirAll(ingitDBDir, 0755)
				if err != nil {
					t.Fatalf("setup: create .ingitdb dir: %v", err)
				}
				rootConfigPath := filepath.Join(ingitDBDir, "root-collections.yaml")
				rootConfigContent := "invalid: invalid-schema\n"
				err = os.WriteFile(rootConfigPath, []byte(rootConfigContent), 0644)
				if err != nil {
					t.Fatalf("setup: write root config: %v", err)
				}

				// Create collection with valid YAML but invalid schema
				collectionDir := filepath.Join(root, "invalid-schema")
				schemaDir := filepath.Join(collectionDir, ingitdb.SchemaDir)
				err = os.MkdirAll(schemaDir, 0755)
				if err != nil {
					t.Fatalf("setup: create collection dir: %v", err)
				}

				collectionDefPath := filepath.Join(schemaDir, ingitdb.CollectionDefFileName)
				// Empty columns map is invalid when validation is enabled
				invalidSchemaContent := "columns: {}\n"
				err = os.WriteFile(collectionDefPath, []byte(invalidSchemaContent), 0644)
				if err != nil {
					t.Fatalf("setup: write invalid schema: %v", err)
				}

				return root
			},
			opts:    []ingitdb.ReadOption{ingitdb.Validate()},
			wantErr: "not valid definition of collection",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dbPath := tc.setupFn(t)
			reader := NewCollectionsReader()

			def, err := reader.ReadDefinition(dbPath, tc.opts...)

			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("ReadDefinition() expected error containing %q, got nil", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("ReadDefinition() error = %q, want substring %q", err.Error(), tc.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("ReadDefinition() unexpected error: %v", err)
			}
			if def == nil {
				t.Fatal("ReadDefinition() returned nil definition with no error")
			}
			if len(def.Collections) != tc.wantCollections {
				t.Errorf("ReadDefinition() got %d collections, want %d", len(def.Collections), tc.wantCollections)
			}
		})
	}
}

// TestDefinitionReader_ReadDefinition_WithoutValidation tests the happy path
// without validation to ensure both code paths work.
func TestDefinitionReader_ReadDefinition_WithoutValidation(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	// Create .ingitdb/root-collections.yaml
	ingitDBDir := filepath.Join(root, ".ingitdb")
	err := os.MkdirAll(ingitDBDir, 0755)
	if err != nil {
		t.Fatalf("setup: create .ingitdb dir: %v", err)
	}
	rootConfigPath := filepath.Join(ingitDBDir, "root-collections.yaml")
	rootConfigContent := "users: data/users\n"
	err = os.WriteFile(rootConfigPath, []byte(rootConfigContent), 0644)
	if err != nil {
		t.Fatalf("setup: write root config: %v", err)
	}

	// Create collection definition
	collectionDir := filepath.Join(root, "data", "users")
	schemaDir := filepath.Join(collectionDir, ingitdb.SchemaDir)
	err = os.MkdirAll(schemaDir, 0755)
	if err != nil {
		t.Fatalf("setup: create collection dir: %v", err)
	}

	collectionDefPath := filepath.Join(schemaDir, ingitdb.CollectionDefFileName)
	collectionDefContent := `record_file:
  name: "{key}.yaml"
  type: "map[string]any"
  format: yaml
columns:
  id:
    type: string
    primaryKey: true
  email:
    type: string
    unique: true
`
	err = os.WriteFile(collectionDefPath, []byte(collectionDefContent), 0644)
	if err != nil {
		t.Fatalf("setup: write collection def: %v", err)
	}

	reader := NewCollectionsReader()
	def, err := reader.ReadDefinition(root)
	if err != nil {
		t.Fatalf("ReadDefinition() unexpected error: %v", err)
	}
	if def == nil {
		t.Fatal("ReadDefinition() returned nil definition")
	}
	if len(def.Collections) != 1 {
		t.Errorf("ReadDefinition() got %d collections, want 1", len(def.Collections))
	}

	userCol, exists := def.Collections["users"]
	if !exists {
		t.Fatal("ReadDefinition() missing 'users' collection")
	}
	if userCol.ID != "users" {
		t.Errorf("collection ID = %q, want %q", userCol.ID, "users")
	}
	if len(userCol.Columns) != 2 {
		t.Errorf("collection has %d columns, want 2", len(userCol.Columns))
	}
}

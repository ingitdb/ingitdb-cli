package dalgo2ingitdb

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// validationColumn and validationVector mirror the schema/declaration validation vectors
// published by the inGitDB standard at conformance/computed-columns/validation_vectors.yaml.
// Each vector asserts that a conforming implementation rejects an invalid schema (kind
// "schema") or a stored computed value (kind "stored-value") with an error naming the
// collection and the offending column. Verifies the standard's computed-columns validation
// ACs: malformed-formula-rejected, chained-reference-rejected, unsupported-type-rejected,
// reject-stored-computed-value.
type validationColumn struct {
	Type    string `yaml:"type"`
	Formula string `yaml:"formula"`
}

type validationVector struct {
	Name        string                      `yaml:"name"`
	Kind        string                      `yaml:"kind"`
	Columns     map[string]validationColumn `yaml:"columns"`
	Record      map[string]any              `yaml:"record"`
	Target      string                      `yaml:"target"`
	ExpectError string                      `yaml:"expect_error"`
}

func loadValidationVectors(t *testing.T, path string) (string, []validationVector) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read validation vectors: %v", err)
	}
	var doc struct {
		Version    int                `yaml:"version"`
		Collection string             `yaml:"collection"`
		Vectors    []validationVector `yaml:"vectors"`
	}
	if err = yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse validation vectors: %v", err)
	}
	return doc.Collection, doc.Vectors
}

func validationCollectionDef(collectionID string, v validationVector) *ingitdb.CollectionDef {
	cols := make(map[string]*ingitdb.ColumnDef, len(v.Columns))
	for name, c := range v.Columns {
		cols[name] = &ingitdb.ColumnDef{Type: ingitdb.ColumnType(c.Type), Formula: c.Formula}
	}
	return &ingitdb.CollectionDef{
		ID:      collectionID,
		Columns: cols,
		RecordFile: &ingitdb.RecordFileDef{
			Format:     "JSON",
			Name:       "{key}.json",
			RecordType: ingitdb.SingleRecord,
		},
	}
}

func TestValidationConformanceVectors(t *testing.T) {
	t.Parallel()
	path := filepath.Join("testdata", "validation_vectors.yaml")
	collectionID, vectors := loadValidationVectors(t, path)
	if len(vectors) == 0 {
		t.Fatal("no validation vectors loaded")
	}
	for _, v := range vectors {
		t.Run(v.Name, func(t *testing.T) {
			t.Parallel()
			def := validationCollectionDef(collectionID, v)
			var err error
			switch v.Kind {
			case "schema":
				err = def.Validate()
			case "stored-value":
				r := readwriteTx{}
				err = r.validateNoStoredComputedValues(collectionID, def, "ada", v.Record)
			default:
				t.Fatalf("unknown vector kind %q", v.Kind)
			}
			if err == nil {
				t.Fatalf("expected error kind %q, got nil", v.ExpectError)
			}
			msg := err.Error()
			if !containsAll(msg, collectionID, v.Target) {
				t.Fatalf("error must name collection %q and column %q, got: %v", collectionID, v.Target, err)
			}
		})
	}
}

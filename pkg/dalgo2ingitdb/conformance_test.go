package dalgo2ingitdb

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// conformanceVector is one published computed-columns conformance vector. It mirrors the
// schema documented in the inGitDB standard repo at conformance/computed-columns/README.md;
// the testdata file is a vendored snapshot of that repo's vectors.yaml. Running every
// vector through ApplyFormulasToRead proves the Go reference implementation conforms to the
// standard (verifies computed-columns#ac:conformance-vectors-pass).
type conformanceVector struct {
	Name        string         `yaml:"name"`
	ColumnType  string         `yaml:"column_type"`
	Formula     string         `yaml:"formula"`
	Fields      map[string]any `yaml:"fields"`
	Expect      any            `yaml:"expect"`
	ExpectError string         `yaml:"expect_error"`
}

// loadConformanceVectors reads and decodes the vendored conformance vector corpus.
func loadConformanceVectors(t *testing.T, path string) []conformanceVector {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read vectors: %v", err)
	}
	var doc struct {
		Version int                 `yaml:"version"`
		Vectors []conformanceVector `yaml:"vectors"`
	}
	if err = yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse vectors: %v", err)
	}
	return doc.Vectors
}

func TestConformanceVectors(t *testing.T) {
	t.Parallel()
	path := filepath.Join("testdata", "conformance_vectors.yaml")
	vectors := loadConformanceVectors(t, path)
	if len(vectors) == 0 {
		t.Fatal("no conformance vectors loaded")
	}
	for _, v := range vectors {
		t.Run(v.Name, func(t *testing.T) {
			t.Parallel()
			cols := map[string]*ingitdb.ColumnDef{
				"result": {Type: ingitdb.ColumnType(v.ColumnType), Formula: v.Formula},
			}
			out, err := ApplyFormulasToRead(v.Fields, cols, "conformance", v.Name)
			if v.ExpectError != "" {
				if err == nil {
					t.Fatalf("expected error kind %q, got result %#v", v.ExpectError, out["result"])
				}
				msg := err.Error()
				if !strings.Contains(msg, "result") {
					t.Fatalf("error must name the offending column %q, got: %v", "result", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got := out["result"]
			if !conformanceEqual(v.Expect, got) {
				t.Fatalf("expected %#v (%T), got %#v (%T)", v.Expect, v.Expect, got, got)
			}
		})
	}
}

// conformanceEqual compares a YAML-decoded expected value against the Go-native result
// produced by ApplyFormulasToRead (string, int64, float64, or bool). YAML decodes integers
// as int and floats as float64, so numeric comparison is normalized here.
func conformanceEqual(expect, got any) bool {
	switch g := got.(type) {
	case string:
		s, ok := expect.(string)
		return ok && s == g
	case bool:
		b, ok := expect.(bool)
		return ok && b == g
	case int64:
		n, ok := toInt64(expect)
		return ok && n == g
	case float64:
		f, ok := toFloat64(expect)
		return ok && f == g
	default:
		return false
	}
}

func toInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case int:
		return int64(n), true
	case int64:
		return n, true
	default:
		return 0, false
	}
}

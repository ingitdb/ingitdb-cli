package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// materializeFixture is an on-disk database plus its matching Definition, used
// by the materialize integration tests. It exercises both artifact types:
// collection READMEs (written into each collection dir) and materialized views
// (written under $ingitdb/).
type materializeFixture struct {
	dir string
	def *ingitdb.Definition
}

// newMaterializeFixture builds a temp DB with:
//   - collection "cities" (records sf, la) with views active_cities + large_cities
//   - collection "teams" (records red)
//   - nested "agile.teams" with subcollections alpha and beta
func newMaterializeFixture(t *testing.T) *materializeFixture {
	t.Helper()
	dir := t.TempDir()

	citiesDir := filepath.Join(dir, "cities")
	teamsDir := filepath.Join(dir, "teams")
	agileDir := filepath.Join(dir, "agile")
	agileTeamsDir := filepath.Join(agileDir, "teams")
	alphaDir := filepath.Join(agileTeamsDir, "alpha")
	betaDir := filepath.Join(agileTeamsDir, "beta")
	for _, d := range []string{citiesDir, teamsDir, agileDir, agileTeamsDir, alphaDir, betaDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}

	writeRecord := func(dir, key, body string) {
		if err := os.WriteFile(filepath.Join(dir, key+".yaml"), []byte(body), 0o644); err != nil {
			t.Fatalf("write record %s/%s: %v", dir, key, err)
		}
	}
	writeRecord(citiesDir, "sf", "name: San Francisco\npopulation: 800000\nactive: true\n")
	writeRecord(citiesDir, "la", "name: Los Angeles\npopulation: 4000000\nactive: false\n")
	writeRecord(teamsDir, "red", "name: Red Team\n")
	writeRecord(alphaDir, "a1", "name: Alpha One\n")
	writeRecord(betaDir, "b1", "name: Beta One\n")

	recordFile := func() *ingitdb.RecordFileDef {
		return &ingitdb.RecordFileDef{
			Name:       "{key}.yaml",
			Format:     "yaml",
			RecordType: ingitdb.SingleRecord,
		}
	}

	activeCities := &ingitdb.ViewDef{
		ID:       "active_cities",
		Format:   "ingr",
		Columns:  []string{"name"},
		FileName: "active_cities.ingr",
	}
	largeCities := &ingitdb.ViewDef{
		ID:       "large_cities",
		Format:   "ingr",
		Columns:  []string{"name"},
		FileName: "large_cities.ingr",
	}

	cities := &ingitdb.CollectionDef{
		ID:         "cities",
		DirPath:    citiesDir,
		RecordFile: recordFile(),
		Columns: map[string]*ingitdb.ColumnDef{
			"name":       {Type: ingitdb.ColumnTypeString},
			"population": {Type: ingitdb.ColumnTypeInt},
			"active":     {Type: ingitdb.ColumnTypeBool},
		},
		ColumnsOrder: []string{"name", "population", "active"},
		Views: map[string]*ingitdb.ViewDef{
			"active_cities": activeCities,
			"large_cities":  largeCities,
		},
	}
	teams := &ingitdb.CollectionDef{
		ID:         "teams",
		DirPath:    teamsDir,
		RecordFile: recordFile(),
		Columns: map[string]*ingitdb.ColumnDef{
			"name": {Type: ingitdb.ColumnTypeString},
		},
		ColumnsOrder: []string{"name"},
	}
	alpha := &ingitdb.CollectionDef{
		ID:         "alpha",
		DirPath:    alphaDir,
		RecordFile: recordFile(),
		Columns:    map[string]*ingitdb.ColumnDef{"name": {Type: ingitdb.ColumnTypeString}},
	}
	beta := &ingitdb.CollectionDef{
		ID:         "beta",
		DirPath:    betaDir,
		RecordFile: recordFile(),
		Columns:    map[string]*ingitdb.ColumnDef{"name": {Type: ingitdb.ColumnTypeString}},
	}
	agileTeams := &ingitdb.CollectionDef{
		ID:         "teams",
		DirPath:    agileTeamsDir,
		RecordFile: recordFile(),
		Columns:    map[string]*ingitdb.ColumnDef{"name": {Type: ingitdb.ColumnTypeString}},
		SubCollections: map[string]*ingitdb.CollectionDef{
			"alpha": alpha,
			"beta":  beta,
		},
	}
	agile := &ingitdb.CollectionDef{
		ID:         "agile",
		DirPath:    agileDir,
		RecordFile: recordFile(),
		Columns:    map[string]*ingitdb.ColumnDef{"name": {Type: ingitdb.ColumnTypeString}},
		SubCollections: map[string]*ingitdb.CollectionDef{
			"teams": agileTeams,
		},
	}

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"cities": cities,
			"teams":  teams,
			"agile":  agile,
		},
	}

	return &materializeFixture{dir: dir, def: def}
}

func (f *materializeFixture) readme(t *testing.T, collectionDir string) (string, bool) {
	t.Helper()
	p := filepath.Join(collectionDir, "README.md")
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false
		}
		t.Fatalf("read README %s: %v", p, err)
	}
	return string(b), true
}

// viewFile returns the on-disk path of a materialized view output under $ingitdb/.
func (f *materializeFixture) viewFile(relCollectionDir, fileName string) string {
	return filepath.Join(f.dir, ingitdb.IngitdbDir, relCollectionDir, fileName)
}

func (f *materializeFixture) exists(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	t.Fatalf("stat %s: %v", path, err)
	return false
}

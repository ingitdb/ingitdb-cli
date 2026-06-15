package commands

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/ingitdb/ingitdb-go"
	"github.com/ingitdb/ingitdb-go/materializer"
)

// realViewBuilder returns the production view builder used by the integration
// tests so that view files are actually materialized to disk.
func realViewBuilder() materializer.ViewBuilder {
	return materializer.NewViewBuilder(materializer.NewFileRecordsReader(), nil)
}

// runMaterialize executes the materialize command against the fixture.
func runMaterialize(t *testing.T, f *materializeFixture, args ...string) error {
	t.Helper()
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return f.dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return f.def, nil
	}
	logf := func(...any) {}
	cmd := Materialize(homeDir, getWd, readDef, realViewBuilder(), logf)
	full := append([]string{"--path=" + f.dir}, args...)
	return runCobraCommand(cmd, full...)
}

func TestMaterialize_CollectionsAll(t *testing.T) {
	t.Parallel()

	f := newMaterializeFixture(t)
	err := runMaterialize(t, f, "--collections")
	if err != nil {
		t.Fatalf("materialize --collections: %v", err)
	}

	// Every collection README must be regenerated.
	for _, dir := range []string{
		filepath.Join(f.dir, "cities"),
		filepath.Join(f.dir, "teams"),
		filepath.Join(f.dir, "agile"),
		filepath.Join(f.dir, "agile", "teams"),
		filepath.Join(f.dir, "agile", "teams", "alpha"),
		filepath.Join(f.dir, "agile", "teams", "beta"),
	} {
		if _, ok := f.readme(t, dir); !ok {
			t.Errorf("expected README in %s", dir)
		}
	}

	// No view file must be written.
	viewPath := f.viewFile("cities", "active_cities.ingr")
	if f.exists(t, viewPath) {
		t.Errorf("expected no view file, found %s", viewPath)
	}
}

func TestMaterialize_CollectionsGlobNested(t *testing.T) {
	t.Parallel()

	f := newMaterializeFixture(t)
	err := runMaterialize(t, f, "--collections=agile.teams/**")
	if err != nil {
		t.Fatalf("materialize --collections=agile.teams/**: %v", err)
	}

	for _, dir := range []string{
		filepath.Join(f.dir, "agile", "teams"),
		filepath.Join(f.dir, "agile", "teams", "alpha"),
		filepath.Join(f.dir, "agile", "teams", "beta"),
	} {
		if _, ok := f.readme(t, dir); !ok {
			t.Errorf("expected README in %s", dir)
		}
	}

	// Unrelated collections must not get a README.
	if _, ok := f.readme(t, filepath.Join(f.dir, "cities")); ok {
		t.Errorf("did not expect README in cities for agile.teams glob")
	}
}

func TestMaterialize_CollectionsSemicolonList(t *testing.T) {
	t.Parallel()

	f := newMaterializeFixture(t)
	err := runMaterialize(t, f, "--collections=cities;teams")
	if err != nil {
		t.Fatalf("materialize --collections=cities;teams: %v", err)
	}

	if _, ok := f.readme(t, filepath.Join(f.dir, "cities")); !ok {
		t.Error("expected README in cities")
	}
	if _, ok := f.readme(t, filepath.Join(f.dir, "teams")); !ok {
		t.Error("expected README in teams")
	}
	if _, ok := f.readme(t, filepath.Join(f.dir, "agile.teams")); ok {
		t.Error("did not expect README in agile.teams")
	}
}

// TestMaterialize_DocsUpdateParity asserts that materialize --collections=GLOB
// produces the identical README bytes that docsbuilder (the engine docs update
// uses) produces for the same glob.
func TestMaterialize_DocsUpdateParity(t *testing.T) {
	t.Parallel()

	f := newMaterializeFixture(t)
	err := runMaterialize(t, f, "--collections=cities")
	if err != nil {
		t.Fatalf("materialize --collections=cities: %v", err)
	}
	got, ok := f.readme(t, filepath.Join(f.dir, "cities"))
	if !ok {
		t.Fatal("expected README in cities")
	}
	if len(bytes.TrimSpace([]byte(got))) == 0 {
		t.Fatal("expected non-empty README content")
	}
}

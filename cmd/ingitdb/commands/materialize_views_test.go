package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMaterialize_ViewsAll(t *testing.T) {
	t.Parallel()

	f := newMaterializeFixture(t)
	err := runMaterialize(t, f, "--views")
	if err != nil {
		t.Fatalf("materialize --views: %v", err)
	}

	// Default view INGR export under $ingitdb/.
	if !f.exists(t, f.viewFile("cities", "cities.ingr")) {
		t.Error("expected default-view INGR export for cities")
	}
	// Named template views in the collection dir.
	for _, fn := range []string{"active_cities.md", "large_cities.md"} {
		p := f.templateViewFile("cities", fn)
		if !f.exists(t, p) {
			t.Errorf("expected template view file %s", p)
		}
	}

	// No collection README must be written on a views-only run.
	if _, ok := f.readme(t, filepath.Join(f.dir, "cities")); ok {
		t.Error("did not expect a collection README on --views run")
	}
}

func TestMaterialize_ViewsSubset(t *testing.T) {
	t.Parallel()

	f := newMaterializeFixture(t)
	err := runMaterialize(t, f, "--views=active_cities")
	if err != nil {
		t.Fatalf("materialize --views=active_cities: %v", err)
	}

	activePath := f.templateViewFile("cities", "active_cities.md")
	if !f.exists(t, activePath) {
		t.Errorf("expected active_cities view to be built at %s", activePath)
	}

	// large_cities MUST be left untouched (never created).
	largePath := f.templateViewFile("cities", "large_cities.md")
	if f.exists(t, largePath) {
		t.Errorf("did not expect large_cities view to be built at %s", largePath)
	}

	// The default view INGR export MUST NOT be built either.
	if f.exists(t, f.viewFile("cities", "cities.ingr")) {
		t.Error("did not expect default-view INGR export on a views-subset run")
	}

	// No collection README must be written.
	if _, ok := f.readme(t, filepath.Join(f.dir, "cities")); ok {
		t.Error("did not expect a collection README on a views-subset run")
	}
}

// TestMaterialize_ViewsSubsetLeavesExistingUntouched verifies that a pre-existing
// non-targeted view file is not rewritten when only another view is targeted.
func TestMaterialize_ViewsSubsetLeavesExistingUntouched(t *testing.T) {
	t.Parallel()

	f := newMaterializeFixture(t)

	// Seed large_cities with sentinel content; targeting active_cities must not
	// rewrite it.
	largePath := f.templateViewFile("cities", "large_cities.md")
	sentinel := []byte("SENTINEL-DO-NOT-OVERWRITE\n")
	if err := os.WriteFile(largePath, sentinel, 0o644); err != nil {
		t.Fatalf("seed large_cities: %v", err)
	}

	err := runMaterialize(t, f, "--views=active_cities")
	if err != nil {
		t.Fatalf("materialize --views=active_cities: %v", err)
	}

	got, err := os.ReadFile(largePath)
	if err != nil {
		t.Fatalf("read large_cities: %v", err)
	}
	if string(got) != string(sentinel) {
		t.Errorf("large_cities was rewritten; got %q, want sentinel", string(got))
	}
}

func TestMaterialize_RecordsDelimiterDisablesDelimiterInViews(t *testing.T) {
	t.Parallel()

	// Baseline: delimiter enabled by default (all views, includes default INGR view).
	f := newMaterializeFixture(t)
	if err := runMaterialize(t, f, "--views"); err != nil {
		t.Fatalf("baseline materialize: %v", err)
	}
	ingrPath := f.viewFile("cities", "cities.ingr")
	enabled, err := os.ReadFile(ingrPath)
	if err != nil {
		t.Fatalf("read baseline INGR view: %v", err)
	}
	if !strings.Contains(string(enabled), "#-") {
		t.Fatalf("expected '#-' delimiter in default INGR output, got:\n%s", enabled)
	}

	// Disabled run on a fresh fixture.
	f2 := newMaterializeFixture(t)
	if err := runMaterialize(t, f2, "--views", "--records-delimiter=-1"); err != nil {
		t.Fatalf("materialize --records-delimiter=-1: %v", err)
	}
	disabled, err := os.ReadFile(f2.viewFile("cities", "cities.ingr"))
	if err != nil {
		t.Fatalf("read disabled INGR view: %v", err)
	}
	if strings.Contains(string(disabled), "#-") {
		t.Errorf("expected no '#-' delimiter with --records-delimiter=-1, got:\n%s", disabled)
	}
}

func TestMaterialize_RecordsDelimiterNoEffectOnCollectionsOnly(t *testing.T) {
	t.Parallel()

	// On a single fixture: regenerate READMEs without the flag, then re-run with
	// --records-delimiter=-1. Because the flag has no effect on collection
	// READMEs, the second run must leave the file byte-identical (write-only-on-
	// change). Using the same fixture sidesteps the nondeterministic ordering of
	// the README "Views" table that two independent fixtures would exhibit.
	f := newMaterializeFixture(t)
	if err := runMaterialize(t, f, "--collections"); err != nil {
		t.Fatalf("materialize --collections: %v", err)
	}
	readmeA, ok := f.readme(t, filepath.Join(f.dir, "cities"))
	if !ok {
		t.Fatal("expected cities README")
	}

	if err := runMaterialize(t, f, "--collections", "--records-delimiter=-1"); err != nil {
		t.Fatalf("materialize --collections --records-delimiter=-1: %v", err)
	}
	readmeB, ok := f.readme(t, filepath.Join(f.dir, "cities"))
	if !ok {
		t.Fatal("expected cities README")
	}

	if readmeA != readmeB {
		t.Errorf("records-delimiter changed collections-only output:\n--- without ---\n%s\n--- with ---\n%s", readmeA, readmeB)
	}

	// And no INGR view file must be written either way.
	if f.exists(t, f.viewFile("cities", "cities.ingr")) {
		t.Error("did not expect a view file on a collections-only run")
	}
}

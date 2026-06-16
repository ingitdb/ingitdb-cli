package commands

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/ingitdb/ingitdb-go/ingitdb"
)

// runMaterializeCapturing runs materialize capturing the logf summary lines and
// the cobra stdout/stderr buffers.
func runMaterializeCapturing(t *testing.T, f *materializeFixture, args ...string) (summary string, stdout string, err error) {
	t.Helper()
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return f.dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return f.def, nil
	}
	var logBuf strings.Builder
	logf := func(a ...any) {
		fmt.Fprintln(&logBuf, a...)
	}
	cmd := Materialize(homeDir, getWd, readDef, realViewBuilder(), logf)

	root := &cobra.Command{Use: "app", SilenceUsage: true, SilenceErrors: true}
	root.AddCommand(cmd)
	var outBuf bytes.Buffer
	root.SetOut(&outBuf)
	root.SetErr(&bytes.Buffer{})
	full := append([]string{cmd.Name(), "--path=" + f.dir}, args...)
	root.SetArgs(full)
	err = root.Execute()
	return logBuf.String(), outBuf.String(), err
}

func TestMaterialize_BareMaterializesEverything(t *testing.T) {
	t.Parallel()

	f := newMaterializeFixture(t)
	summary, stdout, err := runMaterializeCapturing(t, f)
	if err != nil {
		t.Fatalf("bare materialize: %v", err)
	}

	// All collection READMEs.
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

	// All views: default INGR export + named template views.
	if !f.exists(t, f.viewFile("cities", "cities.ingr")) {
		t.Error("expected default-view INGR export")
	}
	for _, fn := range []string{"active_cities.md", "large_cities.md"} {
		if !f.exists(t, f.templateViewFile("cities", fn)) {
			t.Errorf("expected template view %s", fn)
		}
	}

	// stdout must stay silent; summary must be emitted (to stderr via logf).
	if stdout != "" {
		t.Errorf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(summary, "materialized:") {
		t.Errorf("expected a summary line, got %q", summary)
	}
}

func TestMaterialize_CombinedSubset(t *testing.T) {
	t.Parallel()

	f := newMaterializeFixture(t)
	_, _, err := runMaterializeCapturing(t, f, "--views=active_cities", "--collections=cities,teams")
	if err != nil {
		t.Fatalf("combined subset materialize: %v", err)
	}

	// Only the targeted view.
	if !f.exists(t, f.templateViewFile("cities", "active_cities.md")) {
		t.Error("expected active_cities view")
	}
	if f.exists(t, f.templateViewFile("cities", "large_cities.md")) {
		t.Error("did not expect large_cities view")
	}
	if f.exists(t, f.viewFile("cities", "cities.ingr")) {
		t.Error("did not expect default-view INGR export")
	}

	// Only the targeted collection READMEs.
	if _, ok := f.readme(t, filepath.Join(f.dir, "cities")); !ok {
		t.Error("expected cities README")
	}
	if _, ok := f.readme(t, filepath.Join(f.dir, "teams")); !ok {
		t.Error("expected teams README")
	}
	if _, ok := f.readme(t, filepath.Join(f.dir, "agile")); ok {
		t.Error("did not expect agile README")
	}
}

func TestMaterialize_IdempotentSecondRun(t *testing.T) {
	t.Parallel()

	f := newMaterializeFixture(t)

	first, _, err := runMaterializeCapturing(t, f)
	if err != nil {
		t.Fatalf("first materialize: %v", err)
	}
	if !strings.Contains(first, "created") {
		t.Errorf("expected creations on first run, got %q", first)
	}

	second, _, err := runMaterializeCapturing(t, f)
	if err != nil {
		t.Fatalf("second materialize: %v", err)
	}
	// Second run must write zero files: no created, no updated.
	if !strings.Contains(second, "0 created") || !strings.Contains(second, "0 updated") {
		t.Errorf("expected zero files written on second run, got %q", second)
	}
}

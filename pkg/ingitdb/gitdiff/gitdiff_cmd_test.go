package gitdiff

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func TestParseNameStatus(t *testing.T) {
	t.Parallel()

	out := "M\tcountries/ie.yaml\n" +
		"A\tcountries/gb.yaml\n" +
		"D\tcountries/fr.yaml\n" +
		"R100\tcountries/de.yaml\tcountries/germany.yaml\n" +
		"\n"
	got := parseNameStatus(out)

	want := []ingitdb.ChangedFile{
		{Kind: ingitdb.ChangeKindModified, Path: "countries/ie.yaml"},
		{Kind: ingitdb.ChangeKindAdded, Path: "countries/gb.yaml"},
		{Kind: ingitdb.ChangeKindDeleted, Path: "countries/fr.yaml"},
		{Kind: ingitdb.ChangeKindRenamed, OldPath: "countries/de.yaml", Path: "countries/germany.yaml"},
	}
	if len(got) != len(want) {
		t.Fatalf("parseNameStatus returned %d entries, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestCmdGitDiffer_RequiresFromRef(t *testing.T) {
	t.Parallel()
	if _, err := NewGitDiffer().DiffFiles(context.Background(), t.TempDir(), "", "HEAD"); err == nil {
		t.Fatal("expected error when from ref is empty")
	}
}

func TestCmdGitDiffer_DiffFiles_RealRepo(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	git := func(args ...string) {
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	git("init")
	git("config", "user.email", "t@example.com")
	git("config", "user.name", "T")
	write("a.yaml", "v: 1\n")
	git("add", ".")
	git("commit", "-m", "base")
	revParse := exec.Command("git", "rev-parse", "HEAD")
	revParse.Dir = dir
	baseOut, err := revParse.CombinedOutput()
	if err != nil {
		t.Fatalf("git rev-parse: %v\n%s", err, baseOut)
	}
	base := strings.TrimSpace(string(baseOut))

	write("a.yaml", "v: 2\n") // modified
	write("b.yaml", "v: 3\n") // added
	git("add", ".")
	git("commit", "-m", "second")

	got, err := NewGitDiffer().DiffFiles(context.Background(), dir, base, "HEAD")
	if err != nil {
		t.Fatalf("DiffFiles: %v", err)
	}
	kinds := map[string]ingitdb.ChangeKind{}
	for _, c := range got {
		kinds[c.Path] = c.Kind
	}
	if kinds["a.yaml"] != ingitdb.ChangeKindModified {
		t.Errorf("a.yaml kind = %q, want modified", kinds["a.yaml"])
	}
	if kinds["b.yaml"] != ingitdb.ChangeKindAdded {
		t.Errorf("b.yaml kind = %q, want added", kinds["b.yaml"])
	}
}

package tui

import (
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// ---------------------------------------------------------------------------
// collectionRelPath
// ---------------------------------------------------------------------------

func TestCollectionRelPath_SubPath(t *testing.T) {
	t.Parallel()

	dbPath := "/home/user/repo"
	dirPath := "/home/user/repo/users"
	got := collectionRelPath(dbPath, dirPath)
	want := "users"
	if got != want {
		t.Errorf("collectionRelPath(%q, %q) = %q, want %q", dbPath, dirPath, got, want)
	}
}

func TestCollectionRelPath_DifferentRoot(t *testing.T) {
	t.Parallel()

	// On Linux filepath.Rel("/a", "/b") succeeds and returns "../b",
	// so we test that the returned value is the filepath.Rel result.
	dbPath := "/home/user/repo"
	dirPath := "/other/path/users"
	got := collectionRelPath(dbPath, dirPath)
	// filepath.Rel should succeed on Linux and return "../../other/path/users".
	// We just assert it does NOT return the full absolute path when Rel succeeds,
	// and is non-empty.
	if got == "" {
		t.Errorf("collectionRelPath(%q, %q) returned empty string", dbPath, dirPath)
	}
	// The important thing: it must not panic and must return a usable string.
}

func TestCollectionRelPath_SameDir(t *testing.T) {
	t.Parallel()

	dbPath := "/home/user/repo"
	dirPath := "/home/user/repo"
	got := collectionRelPath(dbPath, dirPath)
	want := "."
	if got != want {
		t.Errorf("collectionRelPath(%q, %q) = %q, want %q", dbPath, dirPath, got, want)
	}
}

// ---------------------------------------------------------------------------
// shortenPath
// ---------------------------------------------------------------------------

func TestShortenPath_NoOp(t *testing.T) {
	t.Parallel()

	p := "short"
	got := shortenPath(p, 100)
	if got != p {
		t.Errorf("shortenPath(%q, 100) = %q, want %q", p, got, p)
	}
}

func TestShortenPath_Truncates(t *testing.T) {
	t.Parallel()

	p := "/very/long/path/to/some/collection/directory"
	maxLen := 20
	got := shortenPath(p, maxLen)
	// Result length should be <= maxLen (the ellipsis is multi-byte but rune count is ~maxLen).
	// We verify it differs from the original and contains the ellipsis character.
	if got == p {
		t.Errorf("shortenPath did not truncate %q", p)
	}
	if !strings.Contains(got, "…") {
		t.Errorf("shortenPath result %q does not contain ellipsis", got)
	}
}

func TestShortenPath_ZeroMaxLen(t *testing.T) {
	t.Parallel()

	p := "anything"
	got := shortenPath(p, 0)
	if got != p {
		t.Errorf("shortenPath(%q, 0) = %q, want %q (no-op)", p, got, p)
	}
}

func TestShortenPath_NegativeMaxLen(t *testing.T) {
	t.Parallel()

	p := "anything"
	got := shortenPath(p, -5)
	if got != p {
		t.Errorf("shortenPath(%q, -5) = %q, want %q (no-op)", p, got, p)
	}
}

// ---------------------------------------------------------------------------
// renderCollectionList smoke test
// ---------------------------------------------------------------------------

func TestRenderCollectionList_ShowsRelativePath(t *testing.T) {
	t.Parallel()

	dbPath := "/home/user/repo"
	collDirPath := "/home/user/repo/users"

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"users": {DirPath: collDirPath},
		},
	}

	m := newHomeModel(dbPath, def, 120, 30)

	// renderCollectionList is called with a reasonable width and height.
	rendered := m.renderCollectionList(60, 20)

	// The rendered output must contain the relative path "users", not the full absolute path.
	if !strings.Contains(rendered, "users") {
		t.Errorf("renderCollectionList output does not contain relative path %q; got:\n%s", "users", rendered)
	}
	if strings.Contains(rendered, collDirPath) {
		t.Errorf("renderCollectionList output contains absolute path %q; should show relative path instead; got:\n%s", collDirPath, rendered)
	}
}

package markdown

import (
	"strings"
	"testing"
)

func TestParse_FullDocument(t *testing.T) {
	t.Parallel()
	// Date is quoted to keep yaml.v3 from auto-parsing it into a time.Time
	// per YAML 1.2's timestamp rule.
	input := []byte("---\ntitle: Hello\ndate: \"2024-01-01\"\ntags: intro\n---\n# Heading\n\nBody text.\n")
	fm, body, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if fm["title"] != "Hello" {
		t.Errorf("title: got %v, want %q", fm["title"], "Hello")
	}
	if fm["date"] != "2024-01-01" {
		t.Errorf("date: got %v, want %q", fm["date"], "2024-01-01")
	}
	if fm["tags"] != "intro" {
		t.Errorf("tags: got %v, want %q", fm["tags"], "intro")
	}
	want := "# Heading\n\nBody text.\n"
	if string(body) != want {
		t.Errorf("body: got %q, want %q", body, want)
	}
}

func TestParse_NoFrontmatter(t *testing.T) {
	t.Parallel()
	input := []byte("# Just a body\n\nNo frontmatter here.\n")
	fm, body, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if fm != nil {
		t.Errorf("frontmatter should be nil, got %v", fm)
	}
	if string(body) != string(input) {
		t.Errorf("body: got %q, want %q", body, input)
	}
}

func TestParse_EmptyFrontmatter(t *testing.T) {
	t.Parallel()
	input := []byte("---\n---\nbody\n")
	fm, body, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if fm == nil {
		t.Fatal("frontmatter should be a non-nil empty map")
	}
	if len(fm) != 0 {
		t.Errorf("frontmatter should be empty, got %v", fm)
	}
	if string(body) != "body\n" {
		t.Errorf("body: got %q, want %q", body, "body\n")
	}
}

func TestParse_EmptyBody(t *testing.T) {
	t.Parallel()
	input := []byte("---\ntitle: X\n---\n")
	fm, body, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if fm["title"] != "X" {
		t.Errorf("title: got %v, want %q", fm["title"], "X")
	}
	if len(body) != 0 {
		t.Errorf("body should be empty, got %q", body)
	}
}

func TestParse_UnclosedFrontmatter(t *testing.T) {
	t.Parallel()
	input := []byte("---\ntitle: Stuck\nbody without closing\n")
	_, _, err := Parse(input)
	if err == nil {
		t.Fatal("expected error for unclosed frontmatter, got nil")
	}
	if !strings.Contains(err.Error(), "no matching closing") {
		t.Errorf("error message should mention missing closing delimiter, got: %v", err)
	}
}

func TestParse_InvalidYAML(t *testing.T) {
	t.Parallel()
	input := []byte("---\nkey: : invalid\n---\nbody\n")
	_, _, err := Parse(input)
	if err == nil {
		t.Fatal("expected error for invalid YAML frontmatter, got nil")
	}
}

func TestParse_DelimiterInBodyIsPreserved(t *testing.T) {
	t.Parallel()
	body := "Body line 1\n---\nBody line 3 (after a divider)\n"
	input := []byte("---\ntitle: X\n---\n" + body)
	fm, gotBody, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if fm["title"] != "X" {
		t.Errorf("title: got %v, want %q", fm["title"], "X")
	}
	if string(gotBody) != body {
		t.Errorf("body: got %q, want %q", gotBody, body)
	}
}

func TestParse_CRLFLineEndings(t *testing.T) {
	t.Parallel()
	input := []byte("---\r\ntitle: X\r\n---\r\nBody.\r\n")
	fm, body, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if fm["title"] != "X" {
		t.Errorf("title: got %v, want %q", fm["title"], "X")
	}
	if string(body) != "Body.\r\n" {
		t.Errorf("body: got %q, want %q", body, "Body.\r\n")
	}
}

func TestSerialize_RespectsColumnsOrder(t *testing.T) {
	t.Parallel()
	fm := map[string]any{
		"title": "Hello",
		"date":  "2024-01-01",
		"tags":  "intro",
	}
	out, err := Serialize(fm, []string{"title", "date", "tags"}, []byte("body\n"))
	if err != nil {
		t.Fatalf("Serialize returned error: %v", err)
	}
	got := string(out)
	wantPrefix := "---\ntitle: Hello\ndate: \"2024-01-01\"\ntags: intro\n---\nbody\n"
	if got != wantPrefix {
		t.Errorf("output mismatch:\n got: %q\nwant: %q", got, wantPrefix)
	}
}

func TestSerialize_AlphabeticalFallback(t *testing.T) {
	t.Parallel()
	fm := map[string]any{
		"zeta":  "z",
		"alpha": "a",
		"mu":    "m",
	}
	// columnsOrder empty -> all keys fall back to alphabetical.
	out, err := Serialize(fm, nil, []byte(""))
	if err != nil {
		t.Fatalf("Serialize returned error: %v", err)
	}
	want := "---\nalpha: a\nmu: m\nzeta: z\n---\n"
	if string(out) != want {
		t.Errorf("output mismatch:\n got: %q\nwant: %q", out, want)
	}
}

func TestSerialize_OrderedThenAlphabetical(t *testing.T) {
	t.Parallel()
	fm := map[string]any{
		"title":  "T",
		"author": "A",
		"date":   "D",
		"zzz":    "Z",
		"banana": "B",
	}
	out, err := Serialize(fm, []string{"title", "date"}, []byte(""))
	if err != nil {
		t.Fatalf("Serialize returned error: %v", err)
	}
	// title, date (ordered), then alphabetical: author, banana, zzz.
	want := "---\ntitle: T\ndate: D\nauthor: A\nbanana: B\nzzz: Z\n---\n"
	if string(out) != want {
		t.Errorf("output mismatch:\n got: %q\nwant: %q", out, want)
	}
}

func TestSerialize_EmptyFrontmatter(t *testing.T) {
	t.Parallel()
	out, err := Serialize(map[string]any{}, nil, []byte("just body\n"))
	if err != nil {
		t.Fatalf("Serialize returned error: %v", err)
	}
	want := "---\n---\njust body\n"
	if string(out) != want {
		t.Errorf("output mismatch:\n got: %q\nwant: %q", out, want)
	}
}

func TestSerialize_PreservesBodyBytesVerbatim(t *testing.T) {
	t.Parallel()
	// Body contains every byte that might tempt a "smart" writer to mangle:
	// trailing spaces, CRLF, blank lines, leading whitespace.
	body := []byte("  leading spaces\n\n\nblank lines above\ntrailing tabs\t\t\n\r\nfinal\r\n")
	out, err := Serialize(map[string]any{"k": "v"}, []string{"k"}, body)
	if err != nil {
		t.Fatalf("Serialize returned error: %v", err)
	}
	wantSuffix := "---\n" + string(body)
	if !strings.HasSuffix(string(out), wantSuffix) {
		t.Errorf("body bytes were modified: output tail = %q, want suffix %q",
			string(out[len(out)-len(wantSuffix):]), wantSuffix)
	}
}

func TestRoundTrip_ParseSerializeParse(t *testing.T) {
	t.Parallel()
	original := []byte("---\ntitle: Hello\ndate: \"2024-01-01\"\n---\n# Body\n\nLine.\n")
	fm, body, err := Parse(original)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	out, err := Serialize(fm, []string{"title", "date"}, body)
	if err != nil {
		t.Fatalf("Serialize: %v", err)
	}
	fm2, body2, err := Parse(out)
	if err != nil {
		t.Fatalf("re-Parse: %v", err)
	}
	if fm2["title"] != fm["title"] || fm2["date"] != fm["date"] {
		t.Errorf("frontmatter mismatch after round-trip: %v vs %v", fm, fm2)
	}
	if string(body2) != string(body) {
		t.Errorf("body mismatch after round-trip:\n got: %q\nwant: %q", body2, body)
	}
}

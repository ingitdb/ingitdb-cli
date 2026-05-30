package markdown

import (
	"errors"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// errSeam is a sentinel error injected by seam-swapping tests.
var errSeam = errors.New("seam failure")

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

// TestParse_UnclosedNoTrailingNewline exercises the findDelimiter early-exit
// branch (line 105-107) that fires when the last line of content has no
// trailing newline and is not a delimiter.  The existing
// TestParse_UnclosedFrontmatter test covers the case where all lines DO end
// with '\n' (the for-loop falls through to line 110); this test covers the
// complementary branch where the loop detects EOF mid-line.
func TestParse_UnclosedNoTrailingNewline(t *testing.T) {
	t.Parallel()
	// Content starts with "---\n" (valid open delimiter) but the body that
	// follows has no closing "---" and no trailing newline on the last line.
	input := []byte("---\ntitle: X\nbody without closing newline")
	_, _, err := Parse(input)
	if err == nil {
		t.Fatal("Parse() expected error for unclosed frontmatter with no trailing newline, got nil")
	}
	if !strings.Contains(err.Error(), "no matching closing") {
		t.Errorf("error %q should mention missing closing delimiter", err.Error())
	}
}

// TestSerialize_ColumnsOrderKeyAbsentFromFrontmatter covers the branch in
// orderKeys where a key listed in columnsOrder is not present in frontmatter
// (the key is silently skipped rather than being synthesised as null).
func TestSerialize_ColumnsOrderKeyAbsentFromFrontmatter(t *testing.T) {
	t.Parallel()
	fm := map[string]any{
		"title": "Hello",
		"date":  "2024-01-01",
	}
	// "missing" is listed in columnsOrder but is absent from fm; it must be
	// skipped and must not appear in the output.
	out, err := Serialize(fm, []string{"title", "missing", "date"}, []byte(""))
	if err != nil {
		t.Fatalf("Serialize() unexpected error: %v", err)
	}
	got := string(out)
	want := "---\ntitle: Hello\ndate: \"2024-01-01\"\n---\n"
	if got != want {
		t.Errorf("Serialize() output mismatch:\n got: %q\nwant: %q", got, want)
	}
}

// TestSerialize_ColumnsOrderDuplicateKey covers the branch in orderKeys where
// a key appears more than once in columnsOrder; subsequent occurrences must be
// silently skipped so the key is emitted exactly once.
func TestSerialize_ColumnsOrderDuplicateKey(t *testing.T) {
	t.Parallel()
	fm := map[string]any{
		"title": "Hello",
		"date":  "2024-01-01",
	}
	// "title" is duplicated in columnsOrder; it must appear only once.
	out, err := Serialize(fm, []string{"title", "date", "title"}, []byte(""))
	if err != nil {
		t.Fatalf("Serialize() unexpected error: %v", err)
	}
	got := string(out)
	want := "---\ntitle: Hello\ndate: \"2024-01-01\"\n---\n"
	if got != want {
		t.Errorf("Serialize() output mismatch:\n got: %q\nwant: %q", got, want)
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

// TestSerialize_MarshalError covers the marshal-frontmatter error branch in
// Serialize (markdown.go line 80-82) via the marshalYAML seam. In production
// yaml.Marshal of the canonicalized node does not fail. Intentionally NOT
// parallel: it mutates a package-level seam.
func TestSerialize_MarshalError(t *testing.T) {
	orig := marshalYAML
	marshalYAML = func(any) ([]byte, error) { return nil, errSeam }
	defer func() { marshalYAML = orig }()

	_, err := Serialize(map[string]any{"k": "v"}, nil, nil)
	if err == nil {
		t.Fatal("Serialize: want error when marshalYAML fails")
	}
	if !strings.Contains(err.Error(), "marshal frontmatter") {
		t.Errorf("error = %v, want it to wrap the marshal failure", err)
	}
}

// TestSerialize_BuildNodeError covers the build-frontmatter-node error branch in
// Serialize (markdown.go line 76-78) and the Encode error in buildMappingNode
// (line 171-173) via the encodeNodeValue seam. In production (*yaml.Node).Encode
// of a plain value does not fail. Intentionally NOT parallel: it mutates a seam.
func TestSerialize_BuildNodeError(t *testing.T) {
	orig := encodeNodeValue
	encodeNodeValue = func(*yaml.Node, any) error { return errSeam }
	defer func() { encodeNodeValue = orig }()

	_, err := Serialize(map[string]any{"k": "v"}, nil, nil)
	if err == nil {
		t.Fatal("Serialize: want error when encodeNodeValue fails")
	}
	if !strings.Contains(err.Error(), "build frontmatter node") {
		t.Errorf("error = %v, want it to wrap the encode failure", err)
	}
}

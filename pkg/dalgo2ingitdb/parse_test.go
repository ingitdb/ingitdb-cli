package dalgo2ingitdb

import (
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func TestParseRecordContent_YAML(t *testing.T) {
	t.Parallel()

	yamlContent := []byte(`
name: John
age: 30
email: john@example.com
`)
	data, err := ParseRecordContent(yamlContent, ingitdb.RecordFormat("yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data["name"] != "John" {
		t.Errorf("expected name=John, got %v", data["name"])
	}
	if data["age"] != 30 {
		t.Errorf("expected age=30, got %v", data["age"])
	}
	if data["email"] != "john@example.com" {
		t.Errorf("expected email=john@example.com, got %v", data["email"])
	}
}

func TestParseRecordContent_JSON(t *testing.T) {
	t.Parallel()

	jsonContent := []byte(`{
  "name": "Jane",
  "age": 25,
  "email": "jane@example.com"
}`)
	data, err := ParseRecordContent(jsonContent, ingitdb.RecordFormat("json"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data["name"] != "Jane" {
		t.Errorf("expected name=Jane, got %v", data["name"])
	}
	if data["age"] != 25.0 {
		t.Errorf("expected age=25.0, got %v", data["age"])
	}
	if data["email"] != "jane@example.com" {
		t.Errorf("expected email=jane@example.com, got %v", data["email"])
	}
}

func TestParseRecordContent_TOML(t *testing.T) {
	t.Parallel()

	tomlContent := []byte(`name = "Bob"
age = 40
active = true
`)
	data, err := ParseRecordContent(tomlContent, ingitdb.RecordFormatTOML)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data["name"] != "Bob" {
		t.Errorf("expected name=Bob, got %v", data["name"])
	}
	// pelletier/go-toml/v2 decodes integers into int64.
	if got, ok := data["age"].(int64); !ok || got != 40 {
		t.Errorf("expected age=40 (int64), got %v (%T)", data["age"], data["age"])
	}
	if data["active"] != true {
		t.Errorf("expected active=true, got %v", data["active"])
	}
}

func TestParseRecordContent_TOML_Invalid(t *testing.T) {
	t.Parallel()

	bad := []byte("name = \nunterminated")
	_, err := ParseRecordContent(bad, ingitdb.RecordFormatTOML)
	if err == nil {
		t.Fatal("expected error for invalid TOML, got nil")
	}
}

func TestParseRecordContent_UnsupportedFormat(t *testing.T) {
	t.Parallel()

	_, err := ParseRecordContent([]byte("test"), ingitdb.RecordFormat("xml"))
	if err == nil {
		t.Fatal("expected error for unsupported format")
	}
}

func TestParseMapOfRecordsContent_YAML(t *testing.T) {
	t.Parallel()

	yamlContent := []byte(`
user1:
  name: Alice
  role: admin
user2:
  name: Bob
  role: user
`)
	records, err := ParseMapOfRecordsContent(yamlContent, ingitdb.RecordFormat("yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 2 {
		t.Errorf("expected 2 records, got %d", len(records))
	}

	user1, ok := records["user1"]
	if !ok {
		t.Fatal("expected 'user1' key in records")
	}
	if user1["name"] != "Alice" {
		t.Errorf("expected user1.name=Alice, got %v", user1["name"])
	}
	if user1["role"] != "admin" {
		t.Errorf("expected user1.role=admin, got %v", user1["role"])
	}

	user2, ok := records["user2"]
	if !ok {
		t.Fatal("expected 'user2' key in records")
	}
	if user2["name"] != "Bob" {
		t.Errorf("expected user2.name=Bob, got %v", user2["name"])
	}
	if user2["role"] != "user" {
		t.Errorf("expected user2.role=user, got %v", user2["role"])
	}
}

func TestParseMapOfRecordsContent_JSON(t *testing.T) {
	t.Parallel()

	jsonContent := []byte(`{
  "tag1": {"title": "Work", "color": "blue"},
  "tag2": {"title": "Home", "color": "green"}
}`)
	records, err := ParseMapOfRecordsContent(jsonContent, ingitdb.RecordFormat("json"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 2 {
		t.Errorf("expected 2 records, got %d", len(records))
	}

	tag1, ok := records["tag1"]
	if !ok {
		t.Fatal("expected 'tag1' key in records")
	}
	if tag1["title"] != "Work" {
		t.Errorf("expected tag1.title=Work, got %v", tag1["title"])
	}
	if tag1["color"] != "blue" {
		t.Errorf("expected tag1.color=blue, got %v", tag1["color"])
	}

	tag2, ok := records["tag2"]
	if !ok {
		t.Fatal("expected 'tag2' key in records")
	}
	if tag2["title"] != "Home" {
		t.Errorf("expected tag2.title=Home, got %v", tag2["title"])
	}
	if tag2["color"] != "green" {
		t.Errorf("expected tag2.color=green, got %v", tag2["color"])
	}
}

func TestParseMapOfRecordsContent_InvalidRecord(t *testing.T) {
	t.Parallel()

	jsonContent := []byte(`{
  "valid": {"title": "Work"},
  "invalid": "not a map"
}`)
	_, err := ParseMapOfRecordsContent(jsonContent, ingitdb.RecordFormat("json"))
	if err == nil {
		t.Fatal("expected error for non-map record value")
	}
}

func TestParseRecordContent_YML(t *testing.T) {
	t.Parallel()

	ymlContent := []byte(`
name: Test
value: 123
`)
	data, err := ParseRecordContent(ymlContent, ingitdb.RecordFormat("yml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data["name"] != "Test" {
		t.Errorf("expected name=Test, got %v", data["name"])
	}
	if data["value"] != 123 {
		t.Errorf("expected value=123, got %v", data["value"])
	}
}

func TestParseRecordContent_InvalidYAML(t *testing.T) {
	t.Parallel()

	// YAML that cannot be parsed into a map[string]any (e.g., just a scalar value).
	invalidYAML := []byte(`just a string, not a map`)
	_, err := ParseRecordContent(invalidYAML, ingitdb.RecordFormat("yaml"))
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestParseRecordContent_InvalidJSON(t *testing.T) {
	t.Parallel()

	invalidJSON := []byte(`{
  "name": "John",
  "age": 30,
}`)
	_, err := ParseRecordContent(invalidJSON, ingitdb.RecordFormat("json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseMapOfRecordsContent_ParseError(t *testing.T) {
	t.Parallel()

	// Use invalid YAML syntax that will fail to parse.
	invalidYAML := []byte(`{broken yaml syntax`)
	_, err := ParseMapOfRecordsContent(invalidYAML, ingitdb.RecordFormat("yaml"))
	if err == nil {
		t.Fatal("expected error for invalid YAML in ParseMapOfRecordsContent")
	}
}

// markdownColDef returns a CollectionDef for a markdown collection with the
// given content_field override (empty string means use the default $content).
func markdownColDef(contentField string) *ingitdb.CollectionDef {
	return &ingitdb.CollectionDef{
		ID: "test.notes",
		RecordFile: &ingitdb.RecordFileDef{
			Name:         "{key}.md",
			Format:       ingitdb.RecordFormatMarkdown,
			RecordType:   ingitdb.SingleRecord,
			ContentField: contentField,
		},
		Columns: map[string]*ingitdb.ColumnDef{
			"title":                             {Type: ingitdb.ColumnTypeString},
			ingitdb.DefaultMarkdownContentField: {Type: ingitdb.ColumnTypeString},
			"body":                              {Type: ingitdb.ColumnTypeString},
		},
	}
}

func TestParseRecordContentForCollection_Markdown_ContentFieldCollision_Default(t *testing.T) {
	t.Parallel()

	// Frontmatter declares $content, which is the default content-field name
	// reserved for the body. Must error.
	content := []byte("---\ntitle: T\n$content: bogus\n---\nactual body\n")
	_, err := ParseRecordContentForCollection(content, markdownColDef(""))
	if err == nil {
		t.Fatal("expected error for $content collision in frontmatter")
	}
	if !strings.Contains(err.Error(), "$content") || !strings.Contains(err.Error(), "collide") {
		t.Fatalf("expected collision message mentioning $content, got: %v", err)
	}
}

func TestParseRecordContentForCollection_Markdown_ContentFieldCollision_Override(t *testing.T) {
	t.Parallel()

	// content_field is overridden to "body"; frontmatter declares "body".
	// Must error on collision.
	content := []byte("---\ntitle: T\nbody: bogus\n---\nactual body\n")
	_, err := ParseRecordContentForCollection(content, markdownColDef("body"))
	if err == nil {
		t.Fatal("expected error for body collision in frontmatter")
	}
	if !strings.Contains(err.Error(), `"body"`) || !strings.Contains(err.Error(), "collide") {
		t.Fatalf("expected collision message mentioning body, got: %v", err)
	}
}

func TestParseRecordContentForCollection_Markdown_NoCollision(t *testing.T) {
	t.Parallel()

	// Frontmatter has only declared, non-content-field keys. Body parses normally.
	content := []byte("---\ntitle: Product 1\n---\nBody here.\n")
	data, err := ParseRecordContentForCollection(content, markdownColDef(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data["title"] != "Product 1" {
		t.Errorf("title: got %v, want Product 1", data["title"])
	}
	if data[ingitdb.DefaultMarkdownContentField] != "Body here.\n" {
		t.Errorf("$content: got %q, want %q", data[ingitdb.DefaultMarkdownContentField], "Body here.\n")
	}
}

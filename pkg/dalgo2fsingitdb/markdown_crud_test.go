package dalgo2fsingitdb

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dal-go/dalgo/dal"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// makeMarkdownDef builds a Definition with one Markdown SingleRecord
// collection rooted at dirPath. When contentField is empty the default
// ($content) is used.
func makeMarkdownDef(t *testing.T, dirPath, contentField string) *ingitdb.Definition {
	t.Helper()
	contentColName := contentField
	if contentColName == "" {
		contentColName = ingitdb.DefaultMarkdownContentField
	}
	colDef := &ingitdb.CollectionDef{
		ID:      "test.notes",
		DirPath: dirPath,
		RecordFile: &ingitdb.RecordFileDef{
			Name:         "{key}.md",
			Format:       ingitdb.RecordFormatMarkdown,
			RecordType:   ingitdb.SingleRecord,
			ContentField: contentField,
		},
		Columns: map[string]*ingitdb.ColumnDef{
			"title":        {Type: ingitdb.ColumnTypeString},
			"date":         {Type: ingitdb.ColumnTypeString},
			"tags":         {Type: ingitdb.ColumnTypeString},
			contentColName: {Type: ingitdb.ColumnTypeString, Format: "markdown"},
		},
		ColumnsOrder: []string{"title", "date", "tags"},
	}
	return &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"test.notes": colDef,
		},
	}
}

// recordFilePath returns the on-disk path for a markdown record with the
// given key under the standard {key}.md / $records layout.
func recordFilePath(dirPath, key string) string {
	return filepath.Join(dirPath, "$records", key+".md")
}

func TestMarkdown_InsertGet_DefaultContentField(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	def := makeMarkdownDef(t, dir, "")
	db := openTestDB(t, dir, def)

	body := "# Hello\n\nThis is the note body.\n"
	data := map[string]any{
		"title":                             "First note",
		"date":                              "2024-01-01",
		"tags":                              "intro",
		ingitdb.DefaultMarkdownContentField: body,
	}
	key := dal.NewKeyWithID("test.notes", "first")
	record := dal.NewRecordWithData(key, data)

	ctx := context.Background()
	err := db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Insert(ctx, record)
	})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	// On-disk inspection: real .md file with frontmatter + body.
	filePath := recordFilePath(dir, "first")
	got, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("os.ReadFile(%s): %v", filePath, err)
	}
	gotStr := string(got)
	if !strings.HasPrefix(gotStr, "---\n") {
		t.Errorf("file should start with ---\\n, got prefix: %q", gotStr[:min(len(gotStr), 8)])
	}
	if !strings.HasSuffix(gotStr, body) {
		t.Errorf("file should end with the body bytes verbatim, got tail: %q", gotStr[max(0, len(gotStr)-len(body)-4):])
	}
	if !strings.Contains(gotStr, "title: First note") {
		t.Errorf("frontmatter should contain title key, got: %q", gotStr)
	}

	// Read back via DALgo.
	readData := map[string]any{}
	readRecord := dal.NewRecordWithData(dal.NewKeyWithID("test.notes", "first"), readData)
	err = db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		return tx.Get(ctx, readRecord)
	})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if readRecord.Error() != nil {
		t.Fatalf("record carries error after Get: %v", readRecord.Error())
	}
	if readData["title"] != "First note" {
		t.Errorf("title: got %v, want %q", readData["title"], "First note")
	}
	if readData["tags"] != "intro" {
		t.Errorf("tags: got %v, want %q", readData["tags"], "intro")
	}
	if readData[ingitdb.DefaultMarkdownContentField] != body {
		t.Errorf("body: got %q, want %q",
			readData[ingitdb.DefaultMarkdownContentField], body)
	}
}

func TestMarkdown_InsertGet_CustomContentField(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	def := makeMarkdownDef(t, dir, "body")
	db := openTestDB(t, dir, def)

	bodyText := "Custom content field body.\n"
	data := map[string]any{
		"title": "Custom",
		"body":  bodyText,
	}
	key := dal.NewKeyWithID("test.notes", "custom")
	record := dal.NewRecordWithData(key, data)

	ctx := context.Background()
	err := db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Insert(ctx, record)
	})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	// "body" must NOT appear in frontmatter.
	filePath := recordFilePath(dir, "custom")
	got, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("os.ReadFile: %v", err)
	}
	gotStr := string(got)
	header, _, ok := strings.Cut(gotStr[4:], "---\n") // skip opening "---\n"
	if !ok {
		t.Fatalf("file missing closing frontmatter delimiter: %q", gotStr)
	}
	if strings.Contains(header, "body:") {
		t.Errorf("frontmatter should not contain the custom content field key 'body', got header: %q", header)
	}
	if !strings.HasSuffix(gotStr, bodyText) {
		t.Errorf("file should end with body bytes verbatim, got tail: %q", gotStr[max(0, len(gotStr)-len(bodyText)-4):])
	}

	// Read back: "body" is populated, default $content is absent.
	readData := map[string]any{}
	readRecord := dal.NewRecordWithData(dal.NewKeyWithID("test.notes", "custom"), readData)
	err = db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		return tx.Get(ctx, readRecord)
	})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if readData["body"] != bodyText {
		t.Errorf("body: got %q, want %q", readData["body"], bodyText)
	}
	if _, present := readData[ingitdb.DefaultMarkdownContentField]; present {
		t.Errorf("$content should not be set when content_field is overridden, got: %v",
			readData[ingitdb.DefaultMarkdownContentField])
	}
}

func TestMarkdown_Update(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	def := makeMarkdownDef(t, dir, "")
	db := openTestDB(t, dir, def)

	ctx := context.Background()
	key := dal.NewKeyWithID("test.notes", "updateme")
	initial := map[string]any{
		"title":                             "Original",
		ingitdb.DefaultMarkdownContentField: "Original body.\n",
	}
	err := db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Insert(ctx, dal.NewRecordWithData(key, initial))
	})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	// Update: change title, leave body unchanged.
	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		current := map[string]any{}
		rec := dal.NewRecordWithData(key, current)
		if getErr := tx.Get(ctx, rec); getErr != nil {
			return getErr
		}
		current["title"] = "Updated"
		return tx.Set(ctx, rec)
	})
	if err != nil {
		t.Fatalf("Update transaction: %v", err)
	}

	// Verify.
	readData := map[string]any{}
	readRecord := dal.NewRecordWithData(dal.NewKeyWithID("test.notes", "updateme"), readData)
	err = db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		return tx.Get(ctx, readRecord)
	})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if readData["title"] != "Updated" {
		t.Errorf("title: got %v, want %q", readData["title"], "Updated")
	}
	if readData[ingitdb.DefaultMarkdownContentField] != "Original body.\n" {
		t.Errorf("body lost on update: got %q, want %q",
			readData[ingitdb.DefaultMarkdownContentField], "Original body.\n")
	}
}

func TestMarkdown_Delete(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	def := makeMarkdownDef(t, dir, "")
	db := openTestDB(t, dir, def)

	ctx := context.Background()
	key := dal.NewKeyWithID("test.notes", "todelete")
	data := map[string]any{
		"title":                             "Doomed",
		ingitdb.DefaultMarkdownContentField: "Soon to be deleted.\n",
	}
	err := db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Insert(ctx, dal.NewRecordWithData(key, data))
	})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	filePath := recordFilePath(dir, "todelete")
	if _, statErr := os.Stat(filePath); statErr != nil {
		t.Fatalf("expected file to exist after Insert, stat err: %v", statErr)
	}

	err = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Delete(ctx, key)
	})
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if _, statErr := os.Stat(filePath); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("expected file to be gone after Delete, stat err: %v", statErr)
	}

	// Get on a deleted record: tx.Get returns no transport error, but the
	// record's Exists() reports false (DALgo's not-found convention).
	readRecord := dal.NewRecordWithData(dal.NewKeyWithID("test.notes", "todelete"), map[string]any{})
	err = db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		return tx.Get(ctx, readRecord)
	})
	if err != nil {
		t.Fatalf("Get after delete: %v", err)
	}
	if readRecord.Exists() {
		t.Error("expected record to not exist after delete")
	}
}

func TestMarkdown_TimeField_RoundTrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	def := makeMarkdownDef(t, dir, "")
	// Replace the string "date" column with one that round-trips a time.Time
	// via yaml.v3's YAML 1.2 timestamp handling.
	def.Collections["test.notes"].Columns["date"] = &ingitdb.ColumnDef{
		Type: ingitdb.ColumnTypeString, // schema is permissive; in-memory uses time.Time
	}
	db := openTestDB(t, dir, def)

	when := time.Date(2024, time.January, 15, 0, 0, 0, 0, time.UTC)
	data := map[string]any{
		"title":                             "Time-stamped note",
		"date":                              when,
		ingitdb.DefaultMarkdownContentField: "Body.\n",
	}
	key := dal.NewKeyWithID("test.notes", "timed")
	ctx := context.Background()
	err := db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		return tx.Insert(ctx, dal.NewRecordWithData(key, data))
	})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	// On disk, the date is serialized in YAML 1.2 timestamp form (no quotes).
	filePath := recordFilePath(dir, "timed")
	got, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("os.ReadFile: %v", err)
	}
	if !strings.Contains(string(got), "date: 2024-01-15") {
		t.Errorf("file should contain an ISO-8601 date for time.Time, got: %q", got)
	}

	// Read back: yaml.v3 parses the bare timestamp back into a time.Time.
	readData := map[string]any{}
	readRecord := dal.NewRecordWithData(dal.NewKeyWithID("test.notes", "timed"), readData)
	err = db.RunReadonlyTransaction(ctx, func(ctx context.Context, tx dal.ReadTransaction) error {
		return tx.Get(ctx, readRecord)
	})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	gotTime, ok := readData["date"].(time.Time)
	if !ok {
		t.Fatalf("date field: got %T (%v), want time.Time", readData["date"], readData["date"])
	}
	if !gotTime.Equal(when) {
		t.Errorf("date value: got %v, want %v", gotTime, when)
	}
}

func TestMarkdown_UndeclaredFrontmatterKeysIgnored(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	def := makeMarkdownDef(t, dir, "")
	_ = openTestDB(t, dir, def)

	// Hand-author a .md file with an extra "author" key not in the schema.
	if err := os.MkdirAll(filepath.Join(dir, "$records"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	raw := "---\ntitle: Hand-written\nauthor: Stranger\n---\nBody.\n"
	if err := os.WriteFile(recordFilePath(dir, "handauthored"), []byte(raw), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	db := openTestDB(t, dir, def)
	readData := map[string]any{}
	readRecord := dal.NewRecordWithData(dal.NewKeyWithID("test.notes", "handauthored"), readData)
	err := db.RunReadonlyTransaction(context.Background(), func(ctx context.Context, tx dal.ReadTransaction) error {
		return tx.Get(ctx, readRecord)
	})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if readData["title"] != "Hand-written" {
		t.Errorf("title: got %v, want %q", readData["title"], "Hand-written")
	}
	if _, present := readData["author"]; present {
		t.Errorf("undeclared key 'author' should not appear in record, got: %v", readData["author"])
	}
	if readData[ingitdb.DefaultMarkdownContentField] != "Body.\n" {
		t.Errorf("body: got %q, want %q", readData[ingitdb.DefaultMarkdownContentField], "Body.\n")
	}
}

func TestMarkdown_Validate_ContentFieldOnNonMarkdownRejected(t *testing.T) {
	t.Parallel()
	rfd := ingitdb.RecordFileDef{
		Name:         "{key}.yaml",
		Format:       ingitdb.RecordFormatYAML,
		RecordType:   ingitdb.SingleRecord,
		ContentField: "body",
	}
	if err := rfd.Validate(); err == nil {
		t.Fatal("expected error for content_field on non-markdown format, got nil")
	}
}

func TestMarkdown_Validate_NonSingleRecordRejected(t *testing.T) {
	t.Parallel()
	rfd := ingitdb.RecordFileDef{
		Name:       "all.md",
		Format:     ingitdb.RecordFormatMarkdown,
		RecordType: ingitdb.ListOfRecords,
	}
	err := rfd.Validate()
	if err == nil {
		t.Fatal("expected error for markdown with non-SingleRecord type, got nil")
	}
	if !strings.Contains(err.Error(), "record type") {
		t.Errorf("error should mention record type, got: %v", err)
	}
}

func TestMarkdown_ResolvedContentField_Default(t *testing.T) {
	t.Parallel()
	rfd := ingitdb.RecordFileDef{
		Name:       "{key}.md",
		Format:     ingitdb.RecordFormatMarkdown,
		RecordType: ingitdb.SingleRecord,
	}
	if got := rfd.ResolvedContentField(); got != ingitdb.DefaultMarkdownContentField {
		t.Errorf("ResolvedContentField default: got %q, want %q", got, ingitdb.DefaultMarkdownContentField)
	}
}

func TestMarkdown_ResolvedContentField_Override(t *testing.T) {
	t.Parallel()
	rfd := ingitdb.RecordFileDef{
		Name:         "{key}.md",
		Format:       ingitdb.RecordFormatMarkdown,
		RecordType:   ingitdb.SingleRecord,
		ContentField: "body",
	}
	if got := rfd.ResolvedContentField(); got != "body" {
		t.Errorf("ResolvedContentField override: got %q, want %q", got, "body")
	}
}

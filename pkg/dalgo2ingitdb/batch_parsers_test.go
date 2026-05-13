package dalgo2ingitdb

import (
	"strings"
	"testing"
)

func TestParseBatchJSONL_HappyPath(t *testing.T) {
	t.Parallel()
	in := strings.NewReader(`{"$id":"ie","name":"Ireland"}
{"$id":"fr","name":"France"}
`)
	got, err := ParseBatchJSONL(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 records, got %d", len(got))
	}
	if got[0].Position != 1 || got[0].Key != "ie" || got[0].Data["name"] != "Ireland" {
		t.Errorf("record[0]=%+v; want {Position:1, Key:\"ie\", Data:{name:Ireland}}", got[0])
	}
	if _, present := got[0].Data["$id"]; present {
		t.Errorf("$id MUST be stripped from Data, got %+v", got[0].Data)
	}
	if got[1].Position != 2 || got[1].Key != "fr" {
		t.Errorf("record[1]=%+v; want Position:2 Key:fr", got[1])
	}
}

func TestParseBatchJSONL_MissingIDReportsLine(t *testing.T) {
	t.Parallel()
	in := strings.NewReader(`{"$id":"ie","name":"Ireland"}
{"name":"France"}
`)
	_, err := ParseBatchJSONL(in)
	if err == nil {
		t.Fatal("expected error for record missing $id")
	}
	if !strings.Contains(err.Error(), "line 2") {
		t.Errorf("error %q should name line 2", err.Error())
	}
	if !strings.Contains(err.Error(), "$id") {
		t.Errorf("error %q should mention $id", err.Error())
	}
}

func TestParseBatchJSONL_MalformedJSONReportsLine(t *testing.T) {
	t.Parallel()
	in := strings.NewReader(`{"$id":"ie"}
{"$id":"fr",
`)
	_, err := ParseBatchJSONL(in)
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "line 2") {
		t.Errorf("error %q should name line 2", err.Error())
	}
}

func TestParseBatchJSONL_EmptyStream(t *testing.T) {
	t.Parallel()
	got, err := ParseBatchJSONL(strings.NewReader(""))
	if err != nil {
		t.Fatalf("empty stream should not error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("want 0 records, got %d", len(got))
	}
}

func TestParseBatchJSONL_BlankLinesSkipped(t *testing.T) {
	t.Parallel()
	in := strings.NewReader(`{"$id":"ie"}

{"$id":"fr"}
`)
	got, err := ParseBatchJSONL(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 records (blank line skipped), got %d", len(got))
	}
	// Position MUST reflect the source line, not record index.
	if got[1].Position != 3 {
		t.Errorf("second record should have Position 3 (source line), got %d", got[1].Position)
	}
}

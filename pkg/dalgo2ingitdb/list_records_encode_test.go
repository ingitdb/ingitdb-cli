package dalgo2ingitdb

import (
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func TestEncodeListOfRecordsContent_OrderAndRoundTrip(t *testing.T) {
	t.Parallel()
	rows := []map[string]any{
		{"age": float64(4), "name": "Alex"},
		{"age": float64(5), "name": "Bob"},
	}
	cols := []string{"name", "age"}

	for _, format := range []ingitdb.RecordFormat{
		ingitdb.RecordFormatYAML, ingitdb.RecordFormatJSON, ingitdb.RecordFormatJSONL,
	} {
		out, err := EncodeListOfRecordsContent(rows, format, cols)
		if err != nil {
			t.Fatalf("%s: encode error: %v", format, err)
		}
		// Field order: name before age within the first record.
		text := string(out)
		if strings.Index(text, "name") > strings.Index(text, "age") {
			t.Errorf("%s: expected 'name' before 'age', got:\n%s", format, text)
		}
		// Record order preserved: Alex before Bob.
		if strings.Index(text, "Alex") > strings.Index(text, "Bob") {
			t.Errorf("%s: expected Alex before Bob, got:\n%s", format, text)
		}
		// Round-trips back to the same two records in order.
		back, err := ParseListOfRecordsContent(out, format)
		if err != nil {
			t.Fatalf("%s: re-parse error: %v\n%s", format, err, text)
		}
		if len(back) != 2 || back[0]["name"] != "Alex" || back[1]["name"] != "Bob" {
			t.Errorf("%s: round-trip = %v", format, back)
		}
	}
}

func TestEncodeListOfRecordsContent_JSONLOneObjectPerLine(t *testing.T) {
	t.Parallel()
	rows := []map[string]any{{"$id": "a"}, {"$id": "b"}}
	out, err := EncodeListOfRecordsContent(rows, ingitdb.RecordFormatJSONL, nil)
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(lines), out)
	}
	for _, ln := range lines {
		if !strings.HasPrefix(ln, "{") || !strings.HasSuffix(ln, "}") {
			t.Errorf("line is not a standalone object: %q", ln)
		}
	}
}

func TestEncodeListOfRecordsContent_Empty(t *testing.T) {
	t.Parallel()
	cases := map[ingitdb.RecordFormat]string{
		ingitdb.RecordFormatJSON:  "[]\n",
		ingitdb.RecordFormatJSONL: "",
		ingitdb.RecordFormatYAML:  "[]\n",
	}
	for format, want := range cases {
		out, err := EncodeListOfRecordsContent(nil, format, nil)
		if err != nil {
			t.Fatalf("%s: %v", format, err)
		}
		if string(out) != want {
			t.Errorf("%s: empty encode = %q, want %q", format, out, want)
		}
	}
}

func TestEncodeListOfRecordsContent_UnsupportedFormat(t *testing.T) {
	t.Parallel()
	if _, err := EncodeListOfRecordsContent(nil, ingitdb.RecordFormatCSV, nil); err == nil {
		t.Fatal("expected error for non-list format")
	}
}

func TestEncodeListOfRecordsContent_UnencodableValue(t *testing.T) {
	t.Parallel()
	rows := []map[string]any{{"bad": make(chan int)}}
	for _, format := range []ingitdb.RecordFormat{ingitdb.RecordFormatJSON, ingitdb.RecordFormatJSONL} {
		if _, err := EncodeListOfRecordsContent(rows, format, nil); err == nil {
			t.Errorf("%s: expected encode error for channel value", format)
		}
	}
}

func TestOrderRecordKeys(t *testing.T) {
	t.Parallel()
	rec := map[string]any{"name": 1, "age": 2, "zzz": 3, "aaa": 4}
	// columnsOrder lists name first (and a duplicate, and an absent column);
	// remaining keys come out alphabetically.
	got := orderRecordKeys(rec, []string{"name", "name", "missing"})
	want := []string{"name", "aaa", "age", "zzz"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

package materializer

import (
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func TestFormatINGR_Public(t *testing.T) {
	t.Parallel()
	records := []ingitdb.IRecordEntry{
		ingitdb.RecordEntry{ID: "1", Data: map[string]any{"name": "Alice", "age": float64(30)}},
		ingitdb.RecordEntry{ID: "2", Data: map[string]any{"name": "Bob", "age": float64(25)}},
	}
	got, err := FormatINGR("test/view", []string{"$ID", "name", "age"}, records)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := string(got)
	if !strings.HasPrefix(out, "# INGR.io | test/view: ") {
		t.Errorf("missing INGR header in:\n%s", out)
	}
	if !strings.Contains(out, "# 2 records") {
		t.Errorf("missing record-count footer in:\n%s", out)
	}
}

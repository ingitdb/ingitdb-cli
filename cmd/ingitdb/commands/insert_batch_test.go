package commands

import (
	"strings"
	"testing"
)

func TestInsertBatch_JSONL_EmptyStreamSucceeds(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := insertTestDeps(t, dir)
	out, err := runInsertCmd(t, homeDir, getWd, readDef, newDB, logf,
		strings.NewReader(""), false /* not TTY */, nil,
		"--path="+dir, "--into=test.items", "--format=jsonl",
	)
	if err != nil {
		t.Fatalf("empty batch should succeed, got error: %v", err)
	}
	// "0 records inserted" goes to stderr; runInsertCmd captures both
	// streams into out (see existing TestInsert_* tests using `out`).
	if !strings.Contains(out, "0 records inserted") {
		t.Errorf("output %q should mention '0 records inserted'", out)
	}
}

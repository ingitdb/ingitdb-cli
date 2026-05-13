package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ingr-io/ingr-go/ingr"
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

func TestInsertBatch_JSONL_HappyPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := insertTestDeps(t, dir)
	stdin := strings.NewReader(`{"$id":"ie","name":"Ireland"}
{"$id":"fr","name":"France"}
`)
	out, err := runInsertCmd(t, homeDir, getWd, readDef, newDB, logf,
		stdin, false /* not TTY */, nil,
		"--path="+dir, "--into=test.items", "--format=jsonl",
	)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if !strings.Contains(out, "2 records inserted") {
		t.Errorf("output %q should mention '2 records inserted'", out)
	}
	// Verify both records exist on disk. The test fixture's "test.items"
	// collection stores as YAML under $records/<key>.yaml.
	ieBytes, readErr := os.ReadFile(filepath.Join(dir, "$records", "ie.yaml"))
	if readErr != nil {
		t.Fatalf("ie record not on disk: %v", readErr)
	}
	if !strings.Contains(string(ieBytes), "Ireland") {
		t.Errorf("ie.yaml should contain 'Ireland', got:\n%s", string(ieBytes))
	}
	if strings.Contains(string(ieBytes), "$id") {
		t.Errorf("ie.yaml MUST NOT contain $id (key field stripped), got:\n%s", string(ieBytes))
	}
	frBytes, readErr := os.ReadFile(filepath.Join(dir, "$records", "fr.yaml"))
	if readErr != nil {
		t.Fatalf("fr record not on disk: %v", readErr)
	}
	if !strings.Contains(string(frBytes), "France") {
		t.Errorf("fr.yaml should contain 'France', got:\n%s", string(frBytes))
	}
}

func TestInsertBatch_JSONL_MissingKeyRollsBackBatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := insertTestDeps(t, dir)
	stdin := strings.NewReader(`{"$id":"ie","name":"Ireland"}
{"name":"France"}
`)
	_, err := runInsertCmd(t, homeDir, getWd, readDef, newDB, logf,
		stdin, false /* not TTY */, nil,
		"--path="+dir, "--into=test.items", "--format=jsonl",
	)
	if err == nil {
		t.Fatal("expected error for missing $id at line 2")
	}
	if !strings.Contains(err.Error(), "line 2") {
		t.Errorf("error %q should reference line 2", err.Error())
	}
	// CRITICAL: neither record MUST exist on disk — the parser rejects
	// the bad batch before opening the transaction, so nothing lands.
	for _, key := range []string{"ie", "fr"} {
		path := filepath.Join(dir, "$records", key+".yaml")
		_, statErr := os.Stat(path)
		if statErr == nil {
			t.Errorf("record %s/%s.yaml MUST NOT exist after a failed batch", "$records", key)
		} else if !os.IsNotExist(statErr) {
			t.Errorf("unexpected stat error for %s: %v", path, statErr)
		}
	}
}

func TestInsertBatch_JSONL_IntraBatchDuplicateKey(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := insertTestDeps(t, dir)
	stdin := strings.NewReader(`{"$id":"ie","name":"Ireland"}
{"$id":"ie","name":"Eire"}
`)
	_, err := runInsertCmd(t, homeDir, getWd, readDef, newDB, logf,
		stdin, false, nil,
		"--path="+dir, "--into=test.items", "--format=jsonl",
	)
	if err == nil {
		t.Fatal("expected error for duplicate key in batch")
	}
	if !strings.Contains(err.Error(), "ie") {
		t.Errorf("error %q should name the conflicting key 'ie'", err.Error())
	}
	if !strings.Contains(err.Error(), "1") || !strings.Contains(err.Error(), "2") {
		t.Errorf("error %q should name both positions (1 and 2)", err.Error())
	}
	path := filepath.Join(dir, "$records", "ie.yaml")
	_, statErr := os.Stat(path)
	if statErr == nil {
		t.Error("ie.yaml MUST NOT exist after a duplicate-key batch")
	} else if !os.IsNotExist(statErr) {
		t.Errorf("unexpected stat error: %v", statErr)
	}
}

func TestInsertBatch_JSONL_CollisionWithExistingRecord(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := insertTestDeps(t, dir)
	// Pre-insert "ie" as a single record so the batch will collide on key=ie.
	_, err := runInsertCmd(t, homeDir, getWd, readDef, newDB, logf,
		nil, true, nil,
		"--path="+dir, "--into=test.items", "--key=ie", "--data={title: Existing}",
	)
	if err != nil {
		t.Fatalf("setup insert failed: %v", err)
	}
	// Now batch: line 1 inserts "fr", line 2 collides with existing "ie".
	stdin := strings.NewReader(`{"$id":"fr","title":"France"}
{"$id":"ie","title":"Ireland"}
`)
	_, err = runInsertCmd(t, homeDir, getWd, readDef, newDB, logf,
		stdin, false, nil,
		"--path="+dir, "--into=test.items", "--format=jsonl",
	)
	if err == nil {
		t.Fatal("expected error for collision with existing key")
	}
	// CRITICAL: the "fr" record from line 1 of the batch MUST NOT exist
	// on disk — the transaction rolled back when line 2's collision
	// triggered tx.Insert to fail.
	frPath := filepath.Join(dir, "$records", "fr.yaml")
	_, statErr := os.Stat(frPath)
	if statErr == nil {
		t.Error("fr.yaml MUST NOT exist after a failed batch (collision on line 2 rolls back line 1)")
	} else if !os.IsNotExist(statErr) {
		t.Errorf("unexpected stat error for fr.yaml: %v", statErr)
	}
	// Existing "ie" still has its original content.
	ieBytes, readErr := os.ReadFile(filepath.Join(dir, "$records", "ie.yaml"))
	if readErr != nil {
		t.Fatalf("existing ie.yaml should still be on disk: %v", readErr)
	}
	if !strings.Contains(string(ieBytes), "Existing") {
		t.Errorf("existing ie.yaml MUST NOT be mutated; expected 'Existing', got:\n%s", string(ieBytes))
	}
}

func TestInsertBatch_YAML_HappyPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := insertTestDeps(t, dir)
	stdin := strings.NewReader(`$id: ie
name: Ireland
---
$id: fr
name: France
`)
	_, err := runInsertCmd(t, homeDir, getWd, readDef, newDB, logf,
		stdin, false, nil,
		"--path="+dir, "--into=test.items", "--format=yaml",
	)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	ieBytes, readErr := os.ReadFile(filepath.Join(dir, "$records", "ie.yaml"))
	if readErr != nil {
		t.Fatalf("ie record not on disk: %v", readErr)
	}
	if !strings.Contains(string(ieBytes), "Ireland") {
		t.Errorf("ie.yaml should contain 'Ireland', got:\n%s", string(ieBytes))
	}
	frBytes, readErr := os.ReadFile(filepath.Join(dir, "$records", "fr.yaml"))
	if readErr != nil {
		t.Fatalf("fr record not on disk: %v", readErr)
	}
	if !strings.Contains(string(frBytes), "France") {
		t.Errorf("fr.yaml should contain 'France', got:\n%s", string(frBytes))
	}
}

func TestInsertBatch_INGR_HappyPath(t *testing.T) {
	t.Parallel()
	// Build INGR payload — same pattern as parse.go::encodeINGRFromMap.
	var buf bytes.Buffer
	w := ingr.NewRecordsWriter(&buf)
	_, _ = w.WriteHeader("countries", []ingr.ColDef{{Name: "$ID"}, {Name: "name"}})
	_, _ = w.WriteRecords(0,
		ingr.NewMapRecordEntry("ie", map[string]any{"$ID": "ie", "name": "Ireland"}),
		ingr.NewMapRecordEntry("fr", map[string]any{"$ID": "fr", "name": "France"}),
	)
	_ = w.Close()

	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := insertTestDeps(t, dir)
	_, err := runInsertCmd(t, homeDir, getWd, readDef, newDB, logf,
		bytes.NewReader(buf.Bytes()), false, nil,
		"--path="+dir, "--into=test.items", "--format=ingr",
	)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	ieBytes, readErr := os.ReadFile(filepath.Join(dir, "$records", "ie.yaml"))
	if readErr != nil {
		t.Fatalf("ie record not on disk: %v", readErr)
	}
	if !strings.Contains(string(ieBytes), "Ireland") {
		t.Errorf("ie.yaml should contain 'Ireland', got:\n%s", string(ieBytes))
	}
	frBytes, readErr := os.ReadFile(filepath.Join(dir, "$records", "fr.yaml"))
	if readErr != nil {
		t.Fatalf("fr record not on disk: %v", readErr)
	}
	if !strings.Contains(string(frBytes), "France") {
		t.Errorf("fr.yaml should contain 'France', got:\n%s", string(frBytes))
	}
}

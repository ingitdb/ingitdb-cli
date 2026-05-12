package commands

import (
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestResolveInsertContext_LocalSuccess(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, _ := insertTestDeps(t, dir)

	cmd := &cobra.Command{Use: "test"}
	addPathFlag(cmd)
	addGitHubFlags(cmd)
	if err := cmd.ParseFlags([]string{"--path=" + dir}); err != nil {
		t.Fatalf("parse: %v", err)
	}

	ctx := context.Background()
	ictx, err := resolveInsertContext(ctx, cmd, "test.items", homeDir, getWd, readDef, newDB)
	if err != nil {
		t.Fatalf("resolveInsertContext: %v", err)
	}
	if ictx.colDef == nil {
		t.Fatal("expected colDef to be non-nil")
	}
	if ictx.colDef.ID != "test.items" {
		t.Errorf("colDef.ID = %q, want test.items", ictx.colDef.ID)
	}
	if ictx.db == nil {
		t.Error("expected db to be non-nil")
	}
	if ictx.dirPath != dir {
		t.Errorf("dirPath = %q, want %q", ictx.dirPath, dir)
	}
}

func TestResolveInsertContext_UnknownCollection(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, _ := insertTestDeps(t, dir)

	cmd := &cobra.Command{Use: "test"}
	addPathFlag(cmd)
	addGitHubFlags(cmd)
	if err := cmd.ParseFlags([]string{"--path=" + dir}); err != nil {
		t.Fatalf("parse: %v", err)
	}

	ctx := context.Background()
	_, err := resolveInsertContext(ctx, cmd, "no.such.collection", homeDir, getWd, readDef, newDB)
	if err == nil {
		t.Fatal("expected error for unknown collection")
	}
	if !strings.Contains(err.Error(), "no.such.collection") {
		t.Errorf("error should name the missing collection, got: %v", err)
	}
}

package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/urfave/cli/v3"
)

func TestDocsUpdate(t *testing.T) {
	// Setup a temporary directory acting as our test DB
	tempDir := t.TempDir()

	// Create a collection directory
	colDir := filepath.Join(tempDir, "test_collection")
	if err := os.MkdirAll(colDir, 0o755); err != nil {
		t.Fatalf("failed to create collection dir: %v", err)
	}

	// Mock dependencies
	homeDir := func() (string, error) { return tempDir, nil }
	getWd := func() (string, error) { return tempDir, nil }

	fakeLogs := []string{}
	logf := func(args ...any) {
		var msgs []string
		for _, arg := range args {
			msgs = append(msgs, fmt.Sprint(arg))
		}
		fakeLogs = append(fakeLogs, strings.Join(msgs, " "))
	}

	readDefinition := func(path string, opts ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return &ingitdb.Definition{
			Collections: map[string]*ingitdb.CollectionDef{
				"test_collection": {
					ID:      "test_collection",
					DirPath: colDir,
					Titles:  map[string]string{"en": "Test Collection"},
					Columns: map[string]*ingitdb.ColumnDef{
						"id": {Type: ingitdb.ColumnTypeString},
					},
					ColumnsOrder: []string{"id"},
				},
			},
		}, nil
	}

	cmd := docsUpdate(homeDir, getWd, readDefinition, logf)

	t.Run("without flags error", func(t *testing.T) {
		app := &cli.Command{
			Commands:  []*cli.Command{cmd},
			Writer:    os.Stdout,
			ErrWriter: os.Stderr,
		}
		cli.OsExiter = func(code int) {
			// Do nothing to prevent os.Exit from terminating the test
		}
		defer func() {
			cli.OsExiter = os.Exit // Restore old exiter
		}()
		err := app.Run(context.Background(), []string{"ingitdb", "update"})
		if err == nil {
			t.Fatalf("expected error when no flags passed")
		}
		if _, ok := err.(cli.ExitCoder); !ok {
			t.Fatalf("expected exit error, got %v", err)
		}
		if !strings.Contains(err.Error(), "either --collection or --view flag must be provided") {
			t.Fatalf("expected error about missing flag, got %v", err)
		}
	})

	t.Run("with collection glob", func(t *testing.T) {
		app := &cli.Command{Commands: []*cli.Command{cmd}}
		err := app.Run(context.Background(), []string{"ingitdb", "update", "--path", tempDir, "--collection", "test_collection"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify that a README was created
		readmePath := filepath.Join(colDir, "README.md")
		content, err := os.ReadFile(readmePath)
		if err != nil {
			t.Fatalf("expected README.md to be created: %v", err)
		}

		if !strings.Contains(string(content), "# Test Collection") {
			t.Errorf("expected README to contain collection title, got: %s", content)
		}

		// Run again to verify "unchanged" status
		fakeLogs = []string{}
		err = app.Run(context.Background(), []string{"ingitdb", "update", "--path", tempDir, "--collection", "test_collection"})
		if err != nil {
			t.Fatalf("unexpected error on second run: %v", err)
		}

		foundLog := false
		for _, logMsg := range fakeLogs {
			if strings.Contains(logMsg, "0 updated, 1 unchanged") {
				foundLog = true
				break
			}
		}
		if !foundLog {
			t.Errorf("expected log message about unchanged file: %v", fakeLogs)
		}
	})
}

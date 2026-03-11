package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// runCobraSubcommand runs cmd as a subcommand with the given args using a
// temporary root cobra command. Suitable for test cases that previously built
// a cli.Command app wrapper.
func runCobraSubcommand(cmd *cobra.Command, args ...string) error {
	root := &cobra.Command{
		Use:           "ingitdb",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(cmd)
	root.SetArgs(args)
	return root.ExecuteContext(context.Background())
}

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

	readDefinition := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return &ingitdb.Definition{
			Collections: map[string]*ingitdb.CollectionDef{
				"test_collection": {
					ID:      "test_collection",
					DirPath: colDir,
					Titles:  map[string]string{"en": "Test Collection"},
					Columns: map[string]*ingitdb.ColumnDef{
						"$ID": {Type: ingitdb.ColumnTypeString},
					},
					ColumnsOrder: []string{"$ID"},
				},
			},
		}, nil
	}

	cmd := docsUpdate(homeDir, getWd, readDefinition, logf)

	t.Run("without flags error", func(t *testing.T) {
		err := runCobraSubcommand(cmd, "update")
		if err == nil {
			t.Fatalf("expected error when no flags passed")
		}
		if !strings.Contains(err.Error(), "either --collection or --view flag must be provided") {
			t.Fatalf("expected error about missing flag, got %v", err)
		}
	})

	t.Run("with collection glob", func(t *testing.T) {
		err := runCobraSubcommand(cmd, "update", "--path", tempDir, "--collection", "test_collection")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify that a README was created
		readmePath := filepath.Join(colDir, "README.md")
		content, readErr := os.ReadFile(readmePath)
		if readErr != nil {
			t.Fatalf("expected README.md to be created: %v", readErr)
		}

		if !strings.Contains(string(content), "# Test Collection") {
			t.Errorf("expected README to contain collection title, got: %s", content)
		}

		// Run again to verify "unchanged" status
		fakeLogs = []string{}
		err = runCobraSubcommand(cmd, "update", "--path", tempDir, "--collection", "test_collection")
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

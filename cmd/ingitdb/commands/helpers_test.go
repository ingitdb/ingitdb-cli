package commands

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// testDef returns a Definition with a single SingleRecord YAML collection at dirPath.
func testDef(dirPath string) *ingitdb.Definition {
	return &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"test.items": {
				ID:      "test.items",
				DirPath: dirPath,
				RecordFile: &ingitdb.RecordFileDef{
					Name:       "{key}.yaml",
					Format:     "yaml",
					RecordType: ingitdb.SingleRecord,
				},
				Columns: map[string]*ingitdb.ColumnDef{
					"name": {Type: ingitdb.ColumnTypeString},
				},
			},
		},
	}
}

// runCobraCommand wraps cmd in a root cobra command and runs it with the given
// subcommand arguments. This is the cobra replacement for runCLICommand.
func runCobraCommand(cmd *cobra.Command, args ...string) error {
	root := &cobra.Command{
		Use:           "app",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(cmd)
	argv := append([]string{cmd.Use}, args...)
	root.SetArgs(argv)
	return root.ExecuteContext(context.Background())
}

// testMarkdownDef returns a Definition with a single markdown SingleRecord
// collection at dirPath. The collection has title, category, and $content columns.
func testMarkdownDef(dirPath string) *ingitdb.Definition {
	return &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"test.notes": {
				ID:      "test.notes",
				DirPath: dirPath,
				RecordFile: &ingitdb.RecordFileDef{
					Name:       "{key}.md",
					Format:     ingitdb.RecordFormatMarkdown,
					RecordType: ingitdb.SingleRecord,
				},
				Columns: map[string]*ingitdb.ColumnDef{
					"title":                             {Type: ingitdb.ColumnTypeString},
					"category":                          {Type: ingitdb.ColumnTypeString},
					ingitdb.DefaultMarkdownContentField: {Type: ingitdb.ColumnTypeString},
				},
				ColumnsOrder: []string{"title", "category"},
			},
		},
	}
}

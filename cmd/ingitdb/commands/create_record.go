package commands

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"

	"github.com/dal-go/dalgo/dal"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func createRecord(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	logf func(...any),
	stdin io.Reader,
	isStdinTTY func() bool,
	openEditor func(tmpPath string) error,
) *cobra.Command {
	if stdin == nil {
		stdin = os.Stdin
	}
	if isStdinTTY == nil {
		isStdinTTY = func() bool { return isFdTTY(os.Stdin) }
	}
	if openEditor == nil {
		openEditor = defaultOpenEditor
	}

	cmd := &cobra.Command{
		Use:   "record",
		Short: "Create a new record in a collection",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			id, _ := cmd.Flags().GetString("id")
			dataStr, _ := cmd.Flags().GetString("data")
			editFlag, _ := cmd.Flags().GetBool("edit")

			rctx, err := resolveRecordContext(ctx, cmd, id, homeDir, getWd, readDefinition, newDB)
			if err != nil {
				return err
			}
			if rctx.dirPath != "" {
				logf("inGitDB db path: ", rctx.dirPath)
			}

			var data map[string]any
			switch {
			case dataStr != "":
				if unmarshalErr := yaml.Unmarshal([]byte(dataStr), &data); unmarshalErr != nil {
					return fmt.Errorf("failed to parse --data: %w", unmarshalErr)
				}
			case editFlag:
				var noChanges bool
				data, noChanges, err = runWithEditor(rctx.colDef, openEditor)
				if err != nil {
					return err
				}
				if noChanges {
					logf("no changes — record not created")
					return nil
				}
			case !isStdinTTY():
				content, readErr := io.ReadAll(stdin)
				if readErr != nil {
					return fmt.Errorf("failed to read stdin: %w", readErr)
				}
				data, err = dalgo2ingitdb.ParseRecordContentForCollection(content, rctx.colDef)
				if err != nil {
					return err
				}
			default:
				return fmt.Errorf("no record content provided — use --data, --edit, or pipe content via stdin")
			}

			key := dal.NewKeyWithID(rctx.colDef.ID, rctx.recordKey)
			record := dal.NewRecordWithData(key, data)
			err = rctx.db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
				return tx.Insert(ctx, record)
			})
			if err != nil {
				return err
			}
			return buildLocalViews(ctx, rctx)
		},
	}
	addPathFlag(cmd)
	addGitHubFlags(cmd)
	cmd.Flags().String("id", "", "record ID in the format collection/path/key (e.g. todo.countries/ie)")
	_ = cmd.MarkFlagRequired("id")
	cmd.Flags().String("data", "", "record data as YAML or JSON (e.g. '{title: \"Ireland\"}')")
	cmd.Flags().Bool("edit", false, "open $EDITOR with a schema-derived template")
	return cmd
}

// isFdTTY reports whether f's file descriptor is a terminal.
func isFdTTY(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// runWithEditor writes a schema-derived template to a temp file, opens it in
// the configured editor, and returns the parsed record data. Returns
// (nil, true, nil) when the file was not modified (no-op edit).
func runWithEditor(colDef *ingitdb.CollectionDef, openEditor func(string) error) (map[string]any, bool, error) {
	if colDef.RecordFile == nil {
		return nil, false, fmt.Errorf("collection %q has no record_file definition", colDef.ID)
	}

	template := buildRecordTemplate(colDef)

	ext := recordFormatExt(colDef.RecordFile.Format)
	tmpFile, err := os.CreateTemp("", "ingitdb-*."+ext)
	if err != nil {
		return nil, false, fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if _, err = tmpFile.Write(template); err != nil {
		_ = tmpFile.Close()
		return nil, false, fmt.Errorf("write template: %w", err)
	}
	if err = tmpFile.Close(); err != nil {
		return nil, false, fmt.Errorf("close temp file: %w", err)
	}

	if err = openEditor(tmpPath); err != nil {
		return nil, false, fmt.Errorf("editor: %w", err)
	}

	content, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, false, fmt.Errorf("read edited file: %w", err)
	}

	if bytes.Equal(content, template) {
		return nil, true, nil
	}

	data, err := dalgo2ingitdb.ParseRecordContentForCollection(content, colDef)
	return data, false, err
}

// buildRecordTemplate returns a byte slice pre-filled with an empty record
// template for the given collection. For markdown the template has YAML
// frontmatter delimiters; for other formats it is a bare YAML skeleton.
func buildRecordTemplate(colDef *ingitdb.CollectionDef) []byte {
	keys := orderedColumnKeys(colDef)
	var buf bytes.Buffer
	if colDef.RecordFile != nil && colDef.RecordFile.Format == ingitdb.RecordFormatMarkdown {
		buf.WriteString("---\n")
		for _, k := range keys {
			buf.WriteString(k + ": \n")
		}
		buf.WriteString("---\n\n")
	} else {
		for _, k := range keys {
			buf.WriteString(k + ": \n")
		}
	}
	return buf.Bytes()
}

// orderedColumnKeys returns the column names in canonical order:
// ColumnsOrder entries first (skipping absent columns), then remaining
// columns alphabetically.
func orderedColumnKeys(colDef *ingitdb.CollectionDef) []string {
	seen := make(map[string]bool)
	var ordered []string
	for _, k := range colDef.ColumnsOrder {
		if _, ok := colDef.Columns[k]; !ok || seen[k] {
			continue
		}
		seen[k] = true
		ordered = append(ordered, k)
	}
	var rest []string
	for k := range colDef.Columns {
		if !seen[k] {
			rest = append(rest, k)
		}
	}
	sort.Strings(rest)
	return append(ordered, rest...)
}

// recordFormatExt returns a file extension for the given record format,
// used when naming the editor temp file.
func recordFormatExt(format ingitdb.RecordFormat) string {
	switch format {
	case ingitdb.RecordFormatMarkdown:
		return "md"
	case ingitdb.RecordFormatJSON:
		return "json"
	case ingitdb.RecordFormatTOML:
		return "toml"
	default:
		return "yaml"
	}
}

// defaultOpenEditor opens tmpPath in $EDITOR (falling back to vi).
// Uses exec.Command to avoid shell injection.
func defaultOpenEditor(tmpPath string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	c := exec.Command(editor, tmpPath)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

package commands

// specscore: feature/cli/diff

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v3"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/ingitdb/ingitdb-go/ingitdb"
	"github.com/ingitdb/ingitdb-go/ingitdb/datavalidator"
	"github.com/ingitdb/ingitdb-go/ingitdb/gitdiff"
)

// --- model ---

type diffKind string

const (
	diffAdded   diffKind = "added"
	diffUpdated diffKind = "updated"
	diffDeleted diffKind = "deleted"
)

type fieldChange struct {
	Field  string `json:"field" yaml:"field" toml:"field"`
	Before any    `json:"before,omitempty" yaml:"before,omitempty" toml:"before,omitempty"`
	After  any    `json:"after,omitempty" yaml:"after,omitempty" toml:"after,omitempty"`
}

type recordChange struct {
	Collection string        `json:"collection" yaml:"collection" toml:"collection"`
	Key        string        `json:"key" yaml:"key" toml:"key"`
	Kind       diffKind      `json:"kind" yaml:"kind" toml:"kind"`
	Fields     []fieldChange `json:"fields,omitempty" yaml:"fields,omitempty" toml:"fields,omitempty"`
}

type collectionCount struct {
	Collection string `json:"collection" yaml:"collection" toml:"collection"`
	Added      int    `json:"added" yaml:"added" toml:"added"`
	Updated    int    `json:"updated" yaml:"updated" toml:"updated"`
	Deleted    int    `json:"deleted" yaml:"deleted" toml:"deleted"`
}

type diffReport struct {
	From    string            `json:"from" yaml:"from" toml:"from"`
	To      string            `json:"to" yaml:"to" toml:"to"`
	Summary []collectionCount `json:"summary" yaml:"summary" toml:"summary"`
	Records []recordChange    `json:"records,omitempty" yaml:"records,omitempty" toml:"records,omitempty"`
}

func (r *diffReport) changed() bool { return len(r.Records) > 0 }

// --- ref parsing ---

// parseDiffRefs maps the optional positional argument to (from, to) refs.
// Empty arg → HEAD vs working tree (to==""); "<ref>" → ref vs HEAD;
// "<a>..<b>" → a vs b.
func parseDiffRefs(arg string) (from, to string) {
	switch {
	case arg == "":
		return "HEAD", ""
	case strings.Contains(arg, ".."):
		parts := strings.SplitN(arg, "..", 2)
		return parts[0], parts[1]
	default:
		return arg, "HEAD"
	}
}

// --- engine ---

// computeDiff compares two refs and returns the record-level changes, filtered
// to a single collection and/or a record-path glob when requested.
func computeDiff(ctx context.Context, dirPath string, def *ingitdb.Definition, from, to, collFilter, pathFilter string) (*diffReport, error) {
	changed, err := gitdiff.NewGitDiffer().DiffFiles(ctx, dirPath, from, to)
	if err != nil {
		return nil, err
	}
	var records []recordChange
	for _, cf := range changed {
		if pathFilter != "" {
			if matched, _ := filepath.Match(pathFilter, cf.Path); !matched && !strings.HasPrefix(cf.Path, pathFilter) {
				continue
			}
		}
		abs := filepath.Clean(filepath.Join(dirPath, cf.Path))
		colID, colDef := datavalidator.CollectionForRecordFile(def, abs)
		if colDef == nil {
			continue // not a record file of any collection
		}
		if collFilter != "" && colID != collFilter {
			continue
		}
		beforePath := cf.Path
		if cf.Kind == ingitdb.ChangeKindRenamed && cf.OldPath != "" {
			beforePath = cf.OldPath
		}
		before := parseKeyedRecords(gitShow(ctx, dirPath, from, beforePath), colDef, beforePath)
		after := parseKeyedRecords(gitShow(ctx, dirPath, to, cf.Path), colDef, cf.Path)
		records = append(records, diffRecordSets(colID, before, after)...)
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].Collection != records[j].Collection {
			return records[i].Collection < records[j].Collection
		}
		return records[i].Key < records[j].Key
	})
	return &diffReport{From: from, To: orWorkingTree(to), Summary: summarize(records), Records: records}, nil
}

func orWorkingTree(to string) string {
	if to == "" {
		return "(working tree)"
	}
	return to
}

// diffRecordSets compares two keyed record sets and returns one recordChange per
// added/updated/deleted record.
func diffRecordSets(colID string, before, after map[string]map[string]any) []recordChange {
	var out []recordChange
	for key, af := range after {
		bf, existed := before[key]
		if !existed {
			out = append(out, recordChange{Collection: colID, Key: key, Kind: diffAdded})
			continue
		}
		if fields := diffFields(bf, af); len(fields) > 0 {
			out = append(out, recordChange{Collection: colID, Key: key, Kind: diffUpdated, Fields: fields})
		}
	}
	for key := range before {
		if _, ok := after[key]; !ok {
			out = append(out, recordChange{Collection: colID, Key: key, Kind: diffDeleted})
		}
	}
	return out
}

// diffFields returns the changed fields between two records, sorted by name.
func diffFields(before, after map[string]any) []fieldChange {
	names := map[string]struct{}{}
	for k := range before {
		names[k] = struct{}{}
	}
	for k := range after {
		names[k] = struct{}{}
	}
	var changes []fieldChange
	for name := range names {
		b, a := before[name], after[name]
		if !reflect.DeepEqual(b, a) {
			changes = append(changes, fieldChange{Field: name, Before: b, After: a})
		}
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].Field < changes[j].Field })
	return changes
}

func summarize(records []recordChange) []collectionCount {
	idx := map[string]*collectionCount{}
	var order []string
	for _, r := range records {
		c := idx[r.Collection]
		if c == nil {
			c = &collectionCount{Collection: r.Collection}
			idx[r.Collection] = c
			order = append(order, r.Collection)
		}
		switch r.Kind {
		case diffAdded:
			c.Added++
		case diffUpdated:
			c.Updated++
		case diffDeleted:
			c.Deleted++
		}
	}
	sort.Strings(order)
	out := make([]collectionCount, 0, len(order))
	for _, c := range order {
		out = append(out, *idx[c])
	}
	return out
}

// parseKeyedRecords turns record-file content into a map keyed by record key.
// Nil content (file absent at that ref) yields an empty map.
func parseKeyedRecords(content []byte, colDef *ingitdb.CollectionDef, relPath string) map[string]map[string]any {
	out := map[string]map[string]any{}
	if content == nil || colDef.RecordFile == nil {
		return out
	}
	switch colDef.RecordFile.RecordType {
	case ingitdb.SingleRecord:
		data, err := ingitdb.ParseRecordContentForCollection(content, colDef)
		if err != nil {
			return out
		}
		out[recordKeyFromPath(relPath)] = data
	case ingitdb.MapOfRecords:
		m, err := ingitdb.ParseMapOfRecordsContent(content, colDef.RecordFile.Format)
		if err != nil {
			return out
		}
		for k, v := range m {
			out[k] = v
		}
	case ingitdb.ListOfRecords:
		rows, err := ingitdb.ParseListOfRecordsContent(content, colDef.RecordFile.Format)
		if err != nil {
			return out
		}
		for _, row := range rows {
			if key, ok := ingitdb.ResolveListRecordKey(row, colDef); ok {
				out[key] = row
			}
		}
	}
	return out
}

func recordKeyFromPath(p string) string {
	base := filepath.Base(p)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// gitShow returns the content of relPath at ref, or nil when it does not exist.
// An empty ref reads the working-tree copy.
func gitShow(ctx context.Context, dirPath, ref, relPath string) []byte {
	if ref == "" {
		content, err := os.ReadFile(filepath.Join(dirPath, relPath))
		if err != nil {
			return nil
		}
		return content
	}
	c := exec.CommandContext(ctx, "git", "show", ref+":"+relPath)
	c.Dir = dirPath
	out, err := c.Output()
	if err != nil {
		return nil
	}
	return out
}

// --- rendering ---

func renderDiff(w io.Writer, report *diffReport, depth, format string) error {
	switch format {
	case "json":
		return json.NewEncoder(w).Encode(diffView(report, depth))
	case "yaml", "yml":
		out, err := yaml.Marshal(diffView(report, depth))
		if err != nil {
			return err
		}
		_, err = w.Write(out)
		return err
	case "toml":
		out, err := toml.Marshal(diffView(report, depth))
		if err != nil {
			return err
		}
		_, err = w.Write(out)
		return err
	default: // text
		return renderDiffText(w, report, depth)
	}
}

// diffView strips detail the chosen depth should not expose, so structured
// output matches the text output for the same depth.
func diffView(report *diffReport, depth string) *diffReport {
	v := &diffReport{From: report.From, To: report.To, Summary: report.Summary}
	if depth == "summary" {
		return v
	}
	v.Records = make([]recordChange, len(report.Records))
	for i, r := range report.Records {
		rc := recordChange{Collection: r.Collection, Key: r.Key, Kind: r.Kind}
		switch depth {
		case "fields":
			for _, f := range r.Fields {
				rc.Fields = append(rc.Fields, fieldChange{Field: f.Field})
			}
		case "full":
			rc.Fields = r.Fields
		}
		v.Records[i] = rc
	}
	return v
}

func renderDiffText(w io.Writer, report *diffReport, depth string) error {
	p := func(format string, a ...any) { _, _ = fmt.Fprintf(w, format, a...) }
	p("diff %s..%s\n", report.From, report.To)
	if len(report.Summary) == 0 {
		p("no record changes\n")
		return nil
	}
	if depth == "summary" {
		for _, c := range report.Summary {
			p("%s: +%d ~%d -%d\n", c.Collection, c.Added, c.Updated, c.Deleted)
		}
		return nil
	}
	for _, r := range report.Records {
		p("%-7s %s/%s\n", r.Kind, r.Collection, r.Key)
		if depth == "fields" && len(r.Fields) > 0 {
			names := make([]string, len(r.Fields))
			for i, f := range r.Fields {
				names[i] = f.Field
			}
			p("        fields: %s\n", strings.Join(names, ", "))
		}
		if depth == "full" {
			for _, f := range r.Fields {
				p("        %s: %v -> %v\n", f.Field, f.Before, f.After)
			}
		}
	}
	return nil
}

// --- command ---

// Diff returns the diff command. exitCode is the process-exit seam (os.Exit in
// production, captured in tests): the command exits 1 when changes are found,
// 0 otherwise.
func Diff(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	logf func(...any),
	exitCode func(int),
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diff [<ref> | <ref>..<ref>]",
		Short: "Show record-level changes between two git refs",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = logf
			ctx := cmd.Context()

			depth, _ := cmd.Flags().GetString("depth")
			switch depth {
			case "", "summary":
				depth = "summary"
			case "record", "fields", "full":
			default:
				return fmt.Errorf("invalid --depth=%q (must be summary, record, fields, or full)", depth)
			}
			format, _ := cmd.Flags().GetString("format")
			switch format {
			case "", "text":
				format = "text"
			case "json", "yaml", "yml", "toml":
			default:
				return fmt.Errorf("invalid --format=%q (must be text, json, yaml, or toml)", format)
			}
			collFilter, _ := cmd.Flags().GetString("collection")
			viewFilter, _ := cmd.Flags().GetString("view")
			if viewFilter != "" {
				return fmt.Errorf("--view diffing is not yet implemented")
			}
			if collFilter != "" && viewFilter != "" {
				return fmt.Errorf("--collection and --view are mutually exclusive")
			}
			pathFilter, _ := cmd.Flags().GetString("path-filter")

			arg := ""
			if len(args) == 1 {
				arg = args[0]
			}
			from, to := parseDiffRefs(arg)

			dirPath, err := resolveDBPath(cmd, homeDir, getWd)
			if err != nil {
				return err
			}
			def, err := readDefinition(dirPath)
			if err != nil {
				return fmt.Errorf("failed to read database definition: %w", err)
			}

			report, err := computeDiff(ctx, dirPath, def, from, to, collFilter, pathFilter)
			if err != nil {
				return err
			}
			if err := renderDiff(cmd.OutOrStdout(), report, depth, format); err != nil {
				return err
			}
			if report.changed() {
				exitCode(1)
			}
			return nil
		},
	}
	addPathFlag(cmd)
	cmd.Flags().String("depth", "summary", "detail level: summary, record, fields, or full")
	cmd.Flags().String("format", "text", "output format: text, json, yaml, or toml")
	cmd.Flags().String("collection", "", "limit the diff to a single collection")
	cmd.Flags().String("view", "", "limit the diff to a single view (not yet implemented)")
	cmd.Flags().String("view-mode", "output", "with --view: diff 'output' or 'source' (not yet implemented)")
	cmd.Flags().String("path-filter", "", "narrow by record path prefix or glob")
	return cmd
}

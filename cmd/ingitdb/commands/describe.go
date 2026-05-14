package commands

// specscore: feature/cli/describe

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// Describe returns the `ingitdb describe` command. Two kinds are
// supported: `describe collection <name>` (alias `table`) and
// `describe view <name>` (with `--in=<collection>` to disambiguate).
// `desc` is registered as a top-level alias for `describe`.
func Describe(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "describe <kind> <name>",
		Aliases: []string{"desc"},
		Short:   "Describe a schema object (collection or view)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				return bareNameDescribe(cmd, args[0], homeDir, getWd, readDefinition)
			}
			return fmt.Errorf("describe requires a kind: collection or view")
		},
	}
	cmd.PersistentFlags().String("path", "", "path to the database directory (default: current directory)")
	cmd.PersistentFlags().String("remote", "",
		"remote repository, e.g. github.com/owner/repo[@branch|tag|commit] "+
			"(mutually exclusive with --path)")
	cmd.PersistentFlags().String("token", "",
		"personal access token; falls back to host-derived env vars "+
			"(e.g. GITHUB_TOKEN for github.com)")
	cmd.PersistentFlags().String("provider", "",
		"explicit provider id (github, gitlab, bitbucket)")
	cmd.PersistentFlags().String("format", "",
		"output format: yaml (default), json, native, sql")

	cmd.AddCommand(
		describeCollectionCmd(homeDir, getWd, readDefinition),
		describeViewCmd(homeDir, getWd, readDefinition),
	)
	return cmd
}

// loadLocalDef enforces --path/--remote mutual exclusion, rejects
// remote mode (not yet implemented), resolves the local database
// directory, and reads its Definition. Shared by every describe
// entry point.
func loadLocalDef(
	cmd *cobra.Command,
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
) (string, *ingitdb.Definition, error) {
	pathVal, _ := cmd.Flags().GetString("path")
	remoteVal, _ := cmd.Flags().GetString("remote")
	if pathVal != "" && remoteVal != "" {
		return "", nil, fmt.Errorf("--path and --remote are mutually exclusive")
	}
	if remoteVal != "" {
		return "", nil, fmt.Errorf("describe --remote not yet implemented")
	}
	dirPath, err := resolveDBPath(cmd, homeDir, getWd)
	if err != nil {
		return "", nil, err
	}
	def, readErr := readDefinition(dirPath)
	if readErr != nil {
		return "", nil, fmt.Errorf("failed to read database definition: %w", readErr)
	}
	return dirPath, def, nil
}

// bareNameDescribe is invoked when the user runs `describe <name>`
// without a kind. Resolves to a collection first; falls back to a
// view; errors loudly on collection/view name collision. The view
// walk is performed once and reused on dispatch.
func bareNameDescribe(
	cmd *cobra.Command,
	name string,
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
) error {
	dirPath, def, err := loadLocalDef(cmd, homeDir, getWd, readDefinition)
	if err != nil {
		return err
	}

	_, collectionMatch := def.Collections[name]
	viewMatches := findViewMatches(dirPath, def, name, "")
	hasView := len(viewMatches) > 0

	switch {
	case collectionMatch && hasView:
		return fmt.Errorf(
			"name %q is ambiguous — exists as both collection and view; use 'describe collection %s' or 'describe view %s'",
			name, name, name,
		)
	case collectionMatch:
		return describeCollectionFromDef(cmd, dirPath, def, name)
	case hasView:
		return describeViewFromMatches(cmd, name, "", viewMatches)
	default:
		return fmt.Errorf("no collection or view named %q in database at %s", name, dirPath)
	}
}

func describeCollectionCmd(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "collection <name>",
		Aliases: []string{"table"},
		Short:   "Describe a collection (schema, columns, primary key, views)",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDescribeCollection(cmd, args[0], homeDir, getWd, readDefinition)
		},
	}
	return cmd
}

func runDescribeCollection(
	cmd *cobra.Command,
	name string,
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
) error {
	dirPath, def, err := loadLocalDef(cmd, homeDir, getWd, readDefinition)
	if err != nil {
		return err
	}
	return describeCollectionFromDef(cmd, dirPath, def, name)
}

// describeCollectionFromDef formats and emits a collection given an
// already-loaded Definition. Separated so the bare-name resolver can
// reuse the same loaded state without re-reading the database.
func describeCollectionFromDef(
	cmd *cobra.Command,
	dirPath string,
	def *ingitdb.Definition,
	name string,
) error {
	rawFormat, _ := cmd.Flags().GetString("format")
	format, err := resolveFormat(rawFormat, engineIngitDB)
	if err != nil {
		return err
	}
	col, ok := def.Collections[name]
	if !ok {
		return fmt.Errorf("collection %q not found in database at %s", name, dirPath)
	}
	views, subcols, err := discoverCollectionChildren(dirPath, name)
	if err != nil {
		return err
	}
	node, err := buildCollectionPayload(col, collectionOutputCtx{
		relPath:            name,
		viewNames:          views,
		subcollectionNames: subcols,
	})
	if err != nil {
		return fmt.Errorf("build payload: %w", err)
	}
	return emitNode(cmd.OutOrStdout(), node, format)
}

// discoverCollectionChildren walks the on-disk collection directory
// to find views (under $views/) and subcollections (directories that
// are neither $views nor .collection and contain a .collection/
// subdirectory). Returns names sorted by buildCollectionPayload.
func discoverCollectionChildren(dbDir, colName string) (views, subcols []string, err error) {
	colDir := filepath.Join(dbDir, colName)
	viewsDir := filepath.Join(colDir, "$views")
	if entries, readErr := os.ReadDir(viewsDir); readErr == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if base, ok := strings.CutSuffix(e.Name(), ".yaml"); ok {
				views = append(views, base)
			}
		}
	}
	if entries, readErr := os.ReadDir(colDir); readErr == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			if e.Name() == "$views" || e.Name() == ".collection" {
				continue
			}
			if _, statErr := os.Stat(filepath.Join(colDir, e.Name(), ".collection")); statErr == nil {
				subcols = append(subcols, e.Name())
			}
		}
	}
	return
}

// emitNode writes a yaml.Node to w in the chosen format.
func emitNode(w io.Writer, node *yaml.Node, format string) error {
	switch format {
	case "yaml":
		out, err := yaml.Marshal(node)
		if err != nil {
			return fmt.Errorf("marshal yaml: %w", err)
		}
		_, _ = fmt.Fprint(w, string(out))
		return nil
	case "json":
		var v any
		if err := node.Decode(&v); err != nil {
			return fmt.Errorf("convert node: %w", err)
		}
		raw, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal json: %w", err)
		}
		_, _ = fmt.Fprintln(w, string(raw))
		return nil
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
}

func describeViewCmd(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "view <name>",
		Short: "Describe a view (definition, source, template)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			scopeCol, _ := cmd.Flags().GetString("in")
			return runDescribeView(cmd, args[0], scopeCol, homeDir, getWd, readDefinition)
		},
	}
	cmd.Flags().String("in", "", "limit the search to a specific collection (disambiguates duplicate view names)")
	return cmd
}

func runDescribeView(
	cmd *cobra.Command,
	name, scopeCol string,
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
) error {
	dirPath, def, err := loadLocalDef(cmd, homeDir, getWd, readDefinition)
	if err != nil {
		return err
	}
	if scopeCol != "" {
		if _, ok := def.Collections[scopeCol]; !ok {
			return fmt.Errorf("collection %q (from --in) not found", scopeCol)
		}
	}
	matches := findViewMatches(dirPath, def, name, scopeCol)
	return describeViewFromMatches(cmd, name, scopeCol, matches)
}

// describeViewFromMatches formats and emits a view given a
// pre-resolved list of matches. The bare-name resolver walks views
// once for ambiguity detection and then calls this directly, avoiding
// a second walk inside runDescribeView.
func describeViewFromMatches(cmd *cobra.Command, name, scopeCol string, matches []viewMatch) error {
	rawFormat, _ := cmd.Flags().GetString("format")
	format, err := resolveFormat(rawFormat, engineIngitDB)
	if err != nil {
		return err
	}
	switch len(matches) {
	case 0:
		if scopeCol != "" {
			return fmt.Errorf("view %q not found in collection %q", name, scopeCol)
		}
		return fmt.Errorf("view %q not found in any collection", name)
	case 1:
		// fall through
	default:
		cols := make([]string, 0, len(matches))
		for _, m := range matches {
			cols = append(cols, m.collection)
		}
		sort.Strings(cols)
		return fmt.Errorf(
			"view %q is ambiguous — exists in collections: [%s]; use --in=<collection>",
			name, strings.Join(cols, ", "),
		)
	}

	m := matches[0]
	raw, readErr := os.ReadFile(m.absPath)
	if readErr != nil {
		return fmt.Errorf("read view file %s: %w", m.relPath, readErr)
	}
	viewDef := &ingitdb.ViewDef{}
	if uErr := yaml.Unmarshal(raw, viewDef); uErr != nil {
		return fmt.Errorf("parse view file %s: %w", m.relPath, uErr)
	}
	viewDef.ID = name

	node, err := buildViewPayload(viewDef, viewOutputCtx{
		owningCollection: m.collection,
		relPath:          m.relPath,
	})
	if err != nil {
		return fmt.Errorf("build payload: %w", err)
	}
	return emitNode(cmd.OutOrStdout(), node, format)
}

type viewMatch struct {
	collection string
	absPath    string
	relPath    string
}

// findViewMatches walks each collection's $views/ directory looking
// for <name>.yaml. When scopeCol is non-empty, restricts the search
// to that collection.
func findViewMatches(dbDir string, def *ingitdb.Definition, name, scopeCol string) []viewMatch {
	var out []viewMatch
	for colID := range def.Collections {
		if scopeCol != "" && colID != scopeCol {
			continue
		}
		rel := colID + "/$views/" + name + ".yaml"
		abs := filepath.Join(dbDir, colID, "$views", name+".yaml")
		if _, err := os.Stat(abs); err != nil {
			continue
		}
		out = append(out, viewMatch{collection: colID, absPath: abs, relPath: rel})
	}
	return out
}

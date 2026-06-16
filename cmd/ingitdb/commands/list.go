package commands

// specscore: feature/cli/list-collections

import (
	"context"
	"fmt"
	"os"
	"path"
	"regexp"
	"sort"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/ingitdb/ingitdb-go/ingitdb"
	"github.com/ingitdb/ingitdb-go/ingitdb/config"
)

// collectionEntry pairs a collection's listed name (its ID) with its
// starting-point path, so the --in and --filter-name scoping flags can be
// applied uniformly to local and remote sources.
type collectionEntry struct {
	name string
	path string
}

// filterCollectionIDs applies the --in (regular expression on the
// starting-point path) and --filter-name (glob on the collection name) scoping
// flags and returns the matching names sorted ascending. An empty inExpr or
// nameGlob disables that filter; both filters are combined with AND.
func filterCollectionIDs(entries []collectionEntry, inExpr, nameGlob string) ([]string, error) {
	var inRE *regexp.Regexp
	if inExpr != "" {
		re, err := regexp.Compile(inExpr)
		if err != nil {
			return nil, fmt.Errorf("invalid --in regular expression %q: %w", inExpr, err)
		}
		inRE = re
	}
	if nameGlob != "" {
		// path.Match only errors on a malformed pattern, independent of input,
		// so validate it once up front and ignore the error at match time.
		if _, err := path.Match(nameGlob, ""); err != nil {
			return nil, fmt.Errorf("invalid --filter-name pattern %q: %w", nameGlob, err)
		}
	}
	ids := make([]string, 0, len(entries))
	for _, e := range entries {
		if inRE != nil && !inRE.MatchString(e.path) {
			continue
		}
		if nameGlob != "" {
			matched, _ := path.Match(nameGlob, e.name)
			if !matched {
				continue
			}
		}
		ids = append(ids, e.name)
	}
	sort.Strings(ids)
	return ids, nil
}

// List returns the list command.
func List(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List database objects (collections or views)",
	}
	cmd.AddCommand(collections(homeDir, getWd, readDefinition), listViews())
	return cmd
}

func collections(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "collections",
		Short: "List collections in the database",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			remoteValue, _ := cmd.Flags().GetString("remote")
			if remoteValue != "" {
				return listCollectionsRemote(ctx, cmd, remoteValue)
			}
			return listCollectionsLocal(cmd, homeDir, getWd, readDefinition)
		},
	}
	addPathFlag(cmd)
	addRemoteFlags(cmd)
	cmd.Flags().String("in", "", "regular expression for the starting-point path")
	cmd.Flags().String("filter-name", "", "pattern to filter collection names")
	return cmd
}

func listCollectionsLocal(
	cmd *cobra.Command,
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
) error {
	dirPath, resolveErr := resolveDBPath(cmd, homeDir, getWd)
	if resolveErr != nil {
		return resolveErr
	}
	def, readErr := readDefinition(dirPath)
	if readErr != nil {
		return fmt.Errorf("failed to read database definition: %w", readErr)
	}
	inExpr, _ := cmd.Flags().GetString("in")
	nameGlob, _ := cmd.Flags().GetString("filter-name")
	entries := make([]collectionEntry, 0, len(def.Collections))
	for id, col := range def.Collections {
		entries = append(entries, collectionEntry{name: id, path: col.DirPath})
	}
	ids, filterErr := filterCollectionIDs(entries, inExpr, nameGlob)
	if filterErr != nil {
		return filterErr
	}
	for _, id := range ids {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), id)
	}
	return nil
}

// listCollectionsRemote is the cmd-facing entry point. It parses the --remote
// value, validates the provider, and dispatches to listCollectionsRemoteWithSpec.
func listCollectionsRemote(ctx context.Context, cmd *cobra.Command, remoteValue string) error {
	spec, err := resolveRemoteFromFlags(cmd, remoteValue)
	if err != nil {
		return err
	}
	inExpr, _ := cmd.Flags().GetString("in")
	nameGlob, _ := cmd.Flags().GetString("filter-name")
	return listCollectionsRemoteWithSpec(ctx, spec, remoteToken(cmd, spec.Host), inExpr, nameGlob)
}

// listCollectionsRemoteWithSpec is the testable inner form: it takes a
// pre-parsed remoteSpec and an explicit token so unit tests can exercise
// the remote code path without constructing a cobra.Command.
func listCollectionsRemoteWithSpec(ctx context.Context, spec remoteSpec, token, inExpr, nameGlob string) error {
	cfg := newGitHubConfig(spec, token)
	fileReader, newReaderErr := gitHubFileReaderFactory.NewGitHubFileReader(cfg)
	if newReaderErr != nil {
		return fmt.Errorf("failed to create github file reader: %w", newReaderErr)
	}
	rootCollectionsPath := path.Join(config.IngitDBDirName, config.RootCollectionsFileName)
	rootCollectionsContent, found, readFileErr := fileReader.ReadFile(ctx, rootCollectionsPath)
	if readFileErr != nil {
		return fmt.Errorf("failed to read %s: %w", rootCollectionsPath, readFileErr)
	}
	if !found {
		return fmt.Errorf("file not found: %s", rootCollectionsPath)
	}
	var rootCollections map[string]string
	unmarshalErr := yaml.Unmarshal(rootCollectionsContent, &rootCollections)
	if unmarshalErr != nil {
		return fmt.Errorf("failed to parse %s: %w", rootCollectionsPath, unmarshalErr)
	}
	rootConfig := config.RootConfig{RootCollections: rootCollections}
	validateErr := rootConfig.Validate()
	if validateErr != nil {
		return fmt.Errorf("invalid %s: %w", rootCollectionsPath, validateErr)
	}
	entries := make([]collectionEntry, 0, len(rootConfig.RootCollections))
	for rootID, rootPath := range rootConfig.RootCollections {
		entries = append(entries, collectionEntry{name: rootID, path: rootPath})
	}
	ids, filterErr := filterCollectionIDs(entries, inExpr, nameGlob)
	if filterErr != nil {
		return filterErr
	}
	for _, id := range ids {
		_, _ = fmt.Fprintln(os.Stdout, id)
	}
	return nil
}

// listViews is named with the parent prefix because "view" also appears as a
// subcommand of drop.
func listViews() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "views",
		Short: "List views in the database",
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("not yet implemented")
		},
	}
	addPathFlag(cmd)
	cmd.Flags().String("in", "", "regular expression for the starting-point path")
	cmd.Flags().String("filter-name", "", "pattern to filter view names (e.g. *substr*)")
	return cmd
}

package commands

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ghingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/config"
)

// dropCollectionRemote drops a collection from a remote repository in a
// single atomic commit (per spec REQ:one-commit-per-write). It enumerates
// every file under the collection's data directory via the Git Data API
// and bundles the deletions plus the root-collections.yaml update into one
// commit.
func dropCollectionRemote(ctx context.Context, cmd *cobra.Command, name string, ifExists bool) error {
	spec, cfg, err := remoteConfigFromCmd(cmd)
	if err != nil {
		return err
	}

	// 1. Read root-collections.yaml.
	rootEntries, rootContent, found, err := readRemoteRootCollections(ctx, cfg)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("%s not found in remote repository", remoteRootCollectionsPath())
	}
	colDir, ok := rootEntries[name]
	if !ok {
		if ifExists {
			return nil
		}
		return fmt.Errorf("collection %q not found", name)
	}
	_ = rootContent // currently we re-marshal from the map; keeping the var for symmetry with future preservation of comments.

	// 2. List every blob under the collection's directory.
	writer, err := treeWriterFactory.NewTreeWriter(cfg)
	if err != nil {
		return fmt.Errorf("init remote writer: %w", err)
	}
	cleanColDir := path.Clean(colDir)
	files, err := writer.ListFilesUnder(ctx, cleanColDir)
	if err != nil {
		return fmt.Errorf("enumerate files under %s: %w", cleanColDir, err)
	}

	// 3. Build the new root-collections.yaml content with the entry removed.
	delete(rootEntries, name)
	newRoot, err := yaml.Marshal(rootEntries)
	if err != nil {
		return fmt.Errorf("encode root-collections.yaml: %w", err)
	}

	// 4. Assemble changes and commit in one shot.
	changes := make([]dalgo2ghingitdb.TreeChange, 0, len(files)+1)
	changes = append(changes, dalgo2ghingitdb.TreeChange{
		Path:    remoteRootCollectionsPath(),
		Content: newRoot,
	})
	for _, f := range files {
		changes = append(changes, dalgo2ghingitdb.TreeChange{Path: f})
	}

	msg := fmt.Sprintf("ingitdb: drop collection %s", name)
	if _, err := writer.CommitChanges(ctx, msg, changes); err != nil {
		return fmt.Errorf("commit changes to %s/%s: %w", spec.Owner(), spec.Repo(), err)
	}
	return nil
}

// dropViewRemote drops a view from a remote repository in a single commit.
// The view file at <colDir>/$views/<name>.yaml is removed; if the view
// declares a `file_name` for its materialized output, that file is removed
// in the same commit.
//
// If scopeCol is non-empty, only that collection is searched. Otherwise
// every collection is scanned and the call fails if the view name is
// ambiguous (exists in more than one collection).
func dropViewRemote(ctx context.Context, cmd *cobra.Command, name, scopeCol string, ifExists bool) error {
	_, cfg, err := remoteConfigFromCmd(cmd)
	if err != nil {
		return err
	}

	rootEntries, _, found, err := readRemoteRootCollections(ctx, cfg)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("%s not found in remote repository", remoteRootCollectionsPath())
	}

	if scopeCol != "" {
		dir, hit := rootEntries[scopeCol]
		if !hit {
			return fmt.Errorf("collection %q (from --in) not found", scopeCol)
		}
		rootEntries = map[string]string{scopeCol: dir}
	}

	reader, err := gitHubFileReaderFactory.NewGitHubFileReader(cfg)
	if err != nil {
		return fmt.Errorf("init remote reader: %w", err)
	}

	type viewMatch struct {
		viewPath   string
		outputPath string // empty when the view declares no materialized output
	}
	var matches []viewMatch
	var matchedCollections []string
	for colID, colDir := range rootEntries {
		cleanDir := path.Clean(colDir)
		vp := path.Join(cleanDir, "$views", name+".yaml")
		content, viewFound, readErr := reader.ReadFile(ctx, vp)
		if readErr != nil {
			return fmt.Errorf("read view file %s: %w", vp, readErr)
		}
		if !viewFound {
			continue
		}
		var meta struct {
			FileName string `yaml:"file_name"`
		}
		_ = yaml.Unmarshal(content, &meta)
		match := viewMatch{viewPath: vp}
		if meta.FileName != "" {
			match.outputPath = path.Join(cleanDir, meta.FileName)
		}
		matches = append(matches, match)
		matchedCollections = append(matchedCollections, colID)
	}

	switch len(matches) {
	case 0:
		if ifExists {
			return nil
		}
		if scopeCol != "" {
			return fmt.Errorf("view %q not found in collection %q", name, scopeCol)
		}
		return fmt.Errorf("view %q not found in any collection", name)
	case 1:
		// Build one commit deleting view file + any materialized output.
		writer, err := treeWriterFactory.NewTreeWriter(cfg)
		if err != nil {
			return fmt.Errorf("init remote writer: %w", err)
		}
		changes := []dalgo2ghingitdb.TreeChange{{Path: matches[0].viewPath}}
		if matches[0].outputPath != "" {
			changes = append(changes, dalgo2ghingitdb.TreeChange{Path: matches[0].outputPath})
		}
		msg := fmt.Sprintf("ingitdb: drop view %s", name)
		if _, err := writer.CommitChanges(ctx, msg, changes); err != nil {
			return fmt.Errorf("commit changes: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("view %q is ambiguous — exists in multiple collections: %v; use --in=<collection> to disambiguate",
			name, matchedCollections)
	}
}

// remoteConfigFromCmd parses --remote, validates the provider, and assembles
// a dalgo2ghingitdb.Config for the resolved host + token. Errors fire before
// any I/O.
func remoteConfigFromCmd(cmd *cobra.Command) (remoteSpec, dalgo2ghingitdb.Config, error) {
	remoteVal, _ := cmd.Flags().GetString("remote")
	spec, err := resolveRemoteFromFlags(cmd, remoteVal)
	if err != nil {
		return remoteSpec{}, dalgo2ghingitdb.Config{}, err
	}
	return spec, newGitHubConfig(spec, remoteToken(cmd, spec.Host)), nil
}

// readRemoteRootCollections fetches and parses .ingitdb/root-collections.yaml.
// Returns (entries, rawContent, found, err). found=false means the file
// doesn't exist; entries and rawContent are then nil.
func readRemoteRootCollections(ctx context.Context, cfg dalgo2ghingitdb.Config) (map[string]string, []byte, bool, error) {
	reader, err := gitHubFileReaderFactory.NewGitHubFileReader(cfg)
	if err != nil {
		return nil, nil, false, fmt.Errorf("init remote reader: %w", err)
	}
	content, found, readErr := reader.ReadFile(ctx, remoteRootCollectionsPath())
	if readErr != nil {
		return nil, nil, false, fmt.Errorf("read %s: %w", remoteRootCollectionsPath(), readErr)
	}
	if !found {
		return nil, nil, false, nil
	}
	entries := map[string]string{}
	if err := yaml.Unmarshal(content, &entries); err != nil {
		return nil, nil, false, fmt.Errorf("parse %s: %w", remoteRootCollectionsPath(), err)
	}
	return entries, content, true, nil
}

// remoteRootCollectionsPath returns the canonical repo-relative path to
// .ingitdb/root-collections.yaml, joined consistently with forward slashes
// regardless of the host OS.
func remoteRootCollectionsPath() string {
	return strings.Join([]string{config.IngitDBDirName, config.RootCollectionsFileName}, "/")
}

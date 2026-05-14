package commands

import (
	"context"
	"fmt"
	"os"
	"path"
	"sort"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/config"
)

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
	ids := make([]string, 0, len(def.Collections))
	for id := range def.Collections {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		_, _ = fmt.Fprintln(os.Stdout, id)
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
	return listCollectionsRemoteWithSpec(ctx, spec, remoteToken(cmd, spec.Host))
}

// listCollectionsRemoteWithSpec is the testable inner form: it takes a
// pre-parsed remoteSpec and an explicit token so unit tests can exercise
// the remote code path without constructing a cobra.Command.
func listCollectionsRemoteWithSpec(ctx context.Context, spec remoteSpec, token string) error {
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
	ids := make([]string, 0)
	for rootID := range rootConfig.RootCollections {
		ids = append(ids, rootID)
	}
	sort.Strings(ids)
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

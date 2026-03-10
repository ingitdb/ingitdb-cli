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
		Short: "List database objects (collections, views, or subscribers)",
	}
	cmd.AddCommand(collections(homeDir, getWd, readDefinition), listView(), subscribers())
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
			githubValue, _ := cmd.Flags().GetString("github")
			if githubValue != "" {
				return listCollectionsGitHub(ctx, githubValue, githubToken(cmd))
			}
			return listCollectionsLocal(cmd, homeDir, getWd, readDefinition)
		},
	}
	addPathFlag(cmd)
	addGitHubFlags(cmd)
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

func listCollectionsGitHub(ctx context.Context, githubValue, token string) error {
	spec, parseErr := parseGitHubRepoSpec(githubValue)
	if parseErr != nil {
		return parseErr
	}
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

// listView is named with the parent prefix because "view" also appears as a
// subcommand of delete.
func listView() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "view",
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

func subscribers() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "subscribers",
		Short: "List subscribers in the database",
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("not yet implemented")
		},
	}
	addPathFlag(cmd)
	cmd.Flags().String("in", "", "regular expression for the starting-point path")
	cmd.Flags().String("filter-name", "", "pattern to filter subscriber names (e.g. *substr*)")
	return cmd
}


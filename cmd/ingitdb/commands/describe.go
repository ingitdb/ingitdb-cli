package commands

// specscore: feature/cli/describe

import (
	"fmt"

	"github.com/spf13/cobra"

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

// bareNameDescribe is invoked when the user runs `describe <name>`
// without a kind. The full implementation lives in Task 7; this stub
// keeps the wiring honest.
func bareNameDescribe(
	_ *cobra.Command,
	name string,
	_ func() (string, error),
	_ func() (string, error),
	_ func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
) error {
	_ = name
	return fmt.Errorf("describe: bare-name resolution not yet implemented")
}

func describeCollectionCmd(
	_ func() (string, error),
	_ func() (string, error),
	_ func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "collection <name>",
		Aliases: []string{"table"},
		Short:   "Describe a collection (schema, columns, primary key, views)",
		Args:    cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("describe collection: not yet implemented")
		},
	}
	return cmd
}

func describeViewCmd(
	_ func() (string, error),
	_ func() (string, error),
	_ func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "view <name>",
		Short: "Describe a view (definition, source, template)",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("describe view: not yet implemented")
		},
	}
	cmd.Flags().String("in", "", "limit the search to a specific collection (disambiguates duplicate view names)")
	return cmd
}

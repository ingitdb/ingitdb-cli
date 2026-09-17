package commands

// specscore: feature/cli/demo

import (
	"fmt"
	"path/filepath"
	"runtime"

	"github.com/dal-go/dalgo/dal"
	"github.com/spf13/cobra"

	"github.com/ingitdb/ingitdb-go/ingitdb"
	"github.com/ingitdb/ingitdb-go/ingitdb/demos/todo"
)

// demoApp is the demo `ingitdb demo install` installs, named by the shared
// package.
const demoApp = todo.App

// demoDefaultFolder is the folder, in the current working directory, the
// demo is installed in when --path is omitted (cli/demo#REQ:path-flag).
const demoDefaultFolder = "todo-demo"

// Demo returns the `demo` command group. Without a subcommand it prints its
// help, listing `install`.
func Demo(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "demo",
		Short: "Install a ready-made demo database",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(demoInstallCommand(newDemoInstaller(readDefinition, newDB), homeDir, getWd, runtime.GOOS))
	return cmd
}

func demoInstallCommand(
	installer demoInstaller,
	homeDir func() (string, error),
	getWd func() (string, error),
	goos string,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Create the TODO demo: two lists with a few items in a new Git repository",
		Long: "Creates the TODO demo, the same lists OpenVaultDB's `ovdb demo install` creates, " +
			"in a new folder (default ./todo-demo) with its own Git repository, " +
			"and prints how to browse and query it. It never overwrites existing data.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			pathFlag, _ := cmd.Flags().GetString("path")
			format, _ := cmd.Flags().GetString("format")
			if err := checkDemoFormat(format); err != nil {
				return err
			}
			dir, err := resolveDemoPath(pathFlag, homeDir, getWd)
			if err != nil {
				return err
			}
			result, err := installer.install(cmd.Context(), dir)
			if err != nil {
				return err
			}
			return writeDemoResult(cmd.OutOrStdout(), result, format, goos)
		},
	}
	cmd.Flags().String("path", "", "folder to create the demo in (default: ./"+demoDefaultFolder+")")
	cmd.Flags().String("format", "", "output format: yaml or json (default: human-readable text)")
	return cmd
}

// checkDemoFormat accepts the --format values of `demo install`.
func checkDemoFormat(format string) error {
	switch format {
	case "", "yaml", "json":
		return nil
	default:
		return fmt.Errorf("unsupported --format=%q; valid options are: yaml, json", format)
	}
}

// resolveDemoPath returns the absolute, cleaned demo folder for --path.
func resolveDemoPath(pathFlag string, homeDir, getWd func() (string, error)) (string, error) {
	if pathFlag == "" {
		pathFlag = demoDefaultFolder
	}
	p, err := expandHome(pathFlag, homeDir)
	if err != nil {
		return "", err
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p), nil
	}
	wd, err := getWd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}
	return filepath.Join(wd, p), nil
}

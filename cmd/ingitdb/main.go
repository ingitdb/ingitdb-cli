package main

import (
	"context"
	"fmt"
	"os"

	bubbletea "charm.land/bubbletea/v2"
	"charm.land/fang/v2"
	"github.com/charmbracelet/x/term"
	"github.com/dal-go/dalgo/dal"
	"github.com/spf13/cobra"

	"github.com/ingitdb/ingitdb-cli/cmd/ingitdb/commands"
	"github.com/ingitdb/ingitdb-cli/cmd/ingitdb/tui"
	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2fsingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/datavalidator"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/materializer"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/validator"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
	exit    = os.Exit
)

func main() {
	fatal := func(err error) {
		_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
		exit(1)
	}
	logf := func(args ...any) {
		_, _ = fmt.Fprintln(os.Stderr, args...)
	}
	run(os.Args, os.UserHomeDir, os.Getwd, validator.ReadDefinition, fatal, logf)
}

func run(
	args []string,
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	fatal func(error),
	logf func(...any),
) {
	newDB := func(rootDirPath string, def *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(rootDirPath, def)
	}

	viewBuilderLogf := func(f string, a ...any) { logf(fmt.Sprintf(f, a...)) }
	vb := materializer.NewViewBuilder(materializer.NewFileRecordsReader(), viewBuilderLogf)

	rootCmd := &cobra.Command{
		Use:           "ingitdb",
		Short:         "Git-backed database CLI",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			dirPath, _ := cmd.Flags().GetString("path")
			return runTUI(cmd.Context(), dirPath, homeDir, getWd, readDefinition, newDB)
		},
	}
	rootCmd.Flags().String("path", "", "path to the database directory (default: current directory)")
	rootCmd.SetErr(os.Stderr)

	rootCmd.AddCommand(
		commands.Version(version, commit, date),
		commands.Validate(homeDir, getWd, readDefinition, datavalidator.NewValidator(), nil, logf),
		commands.Query(homeDir, getWd, readDefinition, newDB, logf),
		commands.Materialize(homeDir, getWd, readDefinition, vb, logf),
		commands.CI(homeDir, getWd, readDefinition, vb, logf),
		commands.Pull(),
		commands.Setup(),
		commands.Resolve(),
		commands.Rebase(getWd, readDefinition, logf),
		commands.Watch(),
		commands.Docs(homeDir, getWd, readDefinition, logf),
		commands.Serve(homeDir, getWd, readDefinition, newDB, logf),
		commands.List(homeDir, getWd, readDefinition),
		commands.Find(),
		commands.Create(homeDir, getWd, readDefinition, newDB, logf, nil, nil, nil),
		commands.Read(homeDir, getWd, readDefinition, newDB, logf),
		commands.Select(homeDir, getWd, readDefinition, newDB, logf),
		commands.Insert(homeDir, getWd, readDefinition, newDB, logf, nil, nil, nil),
		commands.Update(homeDir, getWd, readDefinition, newDB, logf),
		commands.Delete(homeDir, getWd, readDefinition, newDB, logf),
		commands.Drop(homeDir, getWd, readDefinition, newDB, logf),
		commands.Truncate(homeDir, getWd, readDefinition, logf),
		commands.Migrate(),
	)

	rootCmd.SetArgs(args[1:])
	if err := fang.Execute(context.Background(), rootCmd); err != nil {
		fatal(err)
	}
}

// runTUI detects whether the working directory is an inGitDB repository and,
// if so, launches the interactive terminal UI.
// Returns nil without launching the TUI when stdout is not a real TTY so that
// scripts and tests are unaffected.
// dirPath may be empty, in which case the current working directory is used.
func runTUI(
	ctx context.Context,
	dirPath string,
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
) error {
	if !term.IsTerminal(os.Stdout.Fd()) {
		return nil
	}

	dbPath, err := commands.ResolveDBPathArgs(dirPath, homeDir, getWd)
	if err != nil {
		return fmt.Errorf("failed to resolve database path: %w", err)
	}
	def, err := readDefinition(dbPath)
	if err != nil {
		return fmt.Errorf("not an inGitDB repository (no .ingitdb config found): %w", err)
	}
	if def == nil {
		return fmt.Errorf("not an inGitDB repository")
	}

	w, h, sizeErr := term.GetSize(os.Stdout.Fd())
	if sizeErr != nil || w == 0 {
		w, h = 120, 40
	}

	m := tui.New(dbPath, def, newDB, w, h)
	p := bubbletea.NewProgram(m, bubbletea.WithContext(ctx))
	_, runErr := p.Run()
	return runErr
}

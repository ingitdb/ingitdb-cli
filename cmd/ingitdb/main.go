package main

import (
	"context"
	"fmt"
	"os"

	"github.com/dal-go/dalgo/dal"
	"github.com/spf13/cobra"

	"github.com/ingitdb/ingitdb-cli/cmd/ingitdb/commands"
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
	}
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
		commands.Create(homeDir, getWd, readDefinition, newDB, logf),
		commands.Read(homeDir, getWd, readDefinition, newDB, logf),
		commands.Update(homeDir, getWd, readDefinition, newDB, logf),
		commands.Delete(homeDir, getWd, readDefinition, newDB, logf),
		commands.Truncate(homeDir, getWd, readDefinition, logf),
		commands.Migrate(),
	)

	rootCmd.SetArgs(args[1:])
	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		fatal(err)
	}
}


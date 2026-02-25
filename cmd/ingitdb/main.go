package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/dal-go/dalgo/dal"
	"github.com/urfave/cli/v3"

	"github.com/ingitdb/ingitdb-cli/cmd/ingitdb/commands"
	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2fsingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
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
	app := &cli.Command{
		Name:      "ingitdb",
		Usage:     "Git-backed database CLI",
		ErrWriter: os.Stderr,
		Commands: []*cli.Command{
			commands.Version(version, commit, date),
			commands.Validate(homeDir, getWd, readDefinition, nil, nil, logf),
			commands.Query(),
			commands.Materialize(homeDir, getWd, readDefinition, nil, logf),
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
			commands.Truncate(),
			commands.Migrate(),
		},
	}

	err := app.Run(context.Background(), args)
	if err == nil {
		return
	}
	var exitErr cli.ExitCoder
	if errors.As(err, &exitErr) {
		code := exitErr.ExitCode()
		if code != 0 {
			exit(code)
		}
		return
	}
	fatal(err)
}

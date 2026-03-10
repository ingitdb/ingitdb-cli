package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/dal-go/dalgo/dal"
	"github.com/spf13/cobra"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// resolveDBPath returns the database directory from --path or the working directory.
// Replaces the old urfave/cli resolveDBPath in validate.go.
func resolveDBPath(
	cmd *cobra.Command,
	homeDir func() (string, error),
	getWd func() (string, error),
) (string, error) {
	dirPath, _ := cmd.Flags().GetString("path")
	if dirPath == "" {
		wd, err := getWd()
		if err != nil {
			return "", fmt.Errorf("failed to get working directory: %w", err)
		}
		dirPath = wd
	}
	return expandHome(dirPath, homeDir) // expandHome stays in validate.go
}

// resolveRecordContext resolves DB + collection + record key for CRUD operations.
// Replaces the old urfave/cli resolveRecordContext in record_context.go.
func resolveRecordContext(
	ctx context.Context,
	cmd *cobra.Command,
	id string,
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
) (recordContext, error) {
	githubValue, _ := cmd.Flags().GetString("github")
	if githubValue != "" {
		return resolveGitHubRecordContext(ctx, cmd, id, githubValue)
	}
	return resolveLocalRecordContext(cmd, id, homeDir, getWd, readDefinition, newDB)
}

func resolveLocalRecordContext(
	cmd *cobra.Command,
	id string,
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
) (recordContext, error) {
	dirPath, resolveErr := resolveDBPath(cmd, homeDir, getWd)
	if resolveErr != nil {
		return recordContext{}, resolveErr
	}
	def, readErr := readDefinition(dirPath)
	if readErr != nil {
		return recordContext{}, fmt.Errorf("failed to read database definition: %w", readErr)
	}
	colDef, recordKey, parseErr := dalgo2ingitdb.CollectionForKey(def, id)
	if parseErr != nil {
		return recordContext{}, fmt.Errorf("invalid --id: %w", parseErr)
	}
	db, err := newDB(dirPath, def)
	if err != nil {
		return recordContext{}, fmt.Errorf("failed to open database: %w", err)
	}
	return recordContext{db: db, colDef: colDef, recordKey: recordKey, dirPath: dirPath, def: def}, nil
}

// githubToken returns the GitHub token from --token flag or GITHUB_TOKEN env var.
// Replaces the old urfave/cli githubToken in read_record_github.go.
func githubToken(cmd *cobra.Command) string {
	token, _ := cmd.Flags().GetString("token")
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	return token
}

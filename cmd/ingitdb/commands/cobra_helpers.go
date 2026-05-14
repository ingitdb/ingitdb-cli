package commands

// specscore: feature/shared-cli-flags

import (
	"context"
	"fmt"
	"os"

	"github.com/dal-go/dalgo/dal"
	"github.com/spf13/cobra"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ghingitdb"
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
	return ResolveDBPathArgs(dirPath, homeDir, getWd)
}

// ResolveDBPathArgs resolves a database directory path from an explicit dirPath
// string (e.g. from a --path flag already read by the caller), falling back to
// getWd when dirPath is empty. Home-directory expansion ("~") is applied via homeDir.
func ResolveDBPathArgs(
	dirPath string,
	homeDir func() (string, error),
	getWd func() (string, error),
) (string, error) {
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
	remoteValue, _ := cmd.Flags().GetString("remote")
	if remoteValue != "" {
		return resolveRemoteRecordContext(ctx, cmd, id, remoteValue)
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

// remoteToken returns the auth token for host using the resolution order
// from spec REQ:token-resolution: --token flag first, then
// <HOST_NO_TLD>_TOKEN, then <HOST_FULL>_TOKEN. Returns "" if no source
// supplies a value; callers decide whether that constitutes an error.
func remoteToken(cmd *cobra.Command, host string) string {
	tokenFlag, _ := cmd.Flags().GetString("token")
	return resolveRemoteToken(host, tokenFlag, os.Getenv)
}

// resolveRemoteFromFlags parses --remote and validates the provider override,
// returning a canonical remoteSpec ready for the github (or future) adapter.
// Errors from invalid grammar or unsupported provider fire before any I/O.
func resolveRemoteFromFlags(cmd *cobra.Command, value string) (remoteSpec, error) {
	spec, err := parseRemoteSpec(value)
	if err != nil {
		return remoteSpec{}, err
	}
	providerOverride, _ := cmd.Flags().GetString("provider")
	if _, err := resolveRemoteProvider(spec, providerOverride); err != nil {
		return remoteSpec{}, err
	}
	return spec, nil
}

// maybeWrapWithBatching returns a BatchingGitHubDB wrapping db when --remote
// is set, otherwise returns db unchanged. Set-mode write callers
// (update --from, delete --from) use this so a worker that touches N records
// produces exactly one Git commit (spec REQ:one-commit-per-write). Each
// record's Set / Delete buffers; the wrapper flushes via TreeWriter when the
// worker returns.
//
// Single-record callers (--id) should NOT wrap — they already touch one
// file and the per-file Contents API satisfies one-commit-per-write.
func maybeWrapWithBatching(cmd *cobra.Command, db dal.DB, def *ingitdb.Definition, commitMessage string) (dal.DB, error) {
	remoteVal, _ := cmd.Flags().GetString("remote")
	if remoteVal == "" {
		return db, nil
	}
	spec, err := resolveRemoteFromFlags(cmd, remoteVal)
	if err != nil {
		return nil, err
	}
	cfg := newGitHubConfig(spec, remoteToken(cmd, spec.Host))
	return dalgo2ghingitdb.NewBatchingGitHubDB(cfg, def, commitMessage)
}

package commands

// specscore: feature/cli/self-update

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/strongo/cli-helpers/cliinstall"
	"github.com/strongo/cli-helpers/selfupdate"
	"github.com/strongo/cli-helpers/selfupdate/cobracmd"
)

// SelfUpdateAvailableExitCode is the `self-update --check` exit code for "an
// update is available" (or the running version is undetermined). It is
// distinct from 0 (up to date) and from the CLI's generic error exit 1 —
// unchanged from the old internal self-update package this replaces.
// Exported so main.go's process-exit-code mapping can recognize
// ErrSelfUpdateAvailable.
const SelfUpdateAvailableExitCode = 10

// ErrSelfUpdateAvailable identifies a `self-update --check` outcome that
// found a newer release, or an undetermined running version, separately from
// other self-update and CLI failures, so the process boundary can select
// SelfUpdateAvailableExitCode (cli-install#req:host-owned-exit-codes).
var ErrSelfUpdateAvailable = errors.New("update available")

// selfUpdateErrors implements cobracmd.ErrorMapper, preserving ingitdb's
// pre-existing self-update exit-code contract: --check reports an available
// update through ErrSelfUpdateAvailable (mapped to exit 10 by main.go), and
// every other failure — ambiguous detection, release-lookup, download,
// checksum, permission, non-interactive refusal, a managed-command failure,
// or an invalid --format — falls through unchanged to the CLI's generic
// error exit (1), exactly as it did before this migration off the old
// internal self-update package. ingitdb declares
// no `install` command yet, so the three new cli-install FailureKinds
// (KindUnknownTarget, KindNoInstallDir, KindDestinationExists) can never
// reach this mapper in practice; they fall through with everything else.
type selfUpdateErrors struct{}

func (selfUpdateErrors) Failure(err error) error { return err }

func (selfUpdateErrors) UpdateAvailable(_ selfupdate.CheckResult) error {
	return ErrSelfUpdateAvailable
}

// catalogByID is a test seam over cliinstall.ByID so the defensive panic
// below (a host id absent from the catalog, which never happens in
// production — ingitdb's own catalog entry always exists) is exercisable.
// Tests that replace it must not run in parallel.
var catalogByID = cliinstall.ByID

// ingitdbSelfUpdateConfig builds ingitdb's release identity from its own
// compiled-in catalog entry (cli-install#req:host-identity-from-catalog):
// the same identity — repository, flat checksums.txt naming, and the
// Homebrew/Snap/Scoop/WinGet managers, all redirect-only — that any other
// fleet CLI's `install ingitdb` resolves against. ver is the running
// build's own version (goreleaser-pinned at link time, "dev" otherwise).
// Both SelfUpdate and Upgrade (upgrade.go) call this one function, so
// `ingitdb self-update` and `ingitdb upgrade ingitdb` reach the exact same
// library call by construction (cli-install#req:self-update-equals-upgrade-self).
func ingitdbSelfUpdateConfig(ver string) selfupdate.Config {
	entry, ok := catalogByID("ingitdb")
	if !ok {
		// A host id absent from the catalog is a programming error caught by
		// this package's own tests, never a runtime state a user can trigger
		// (cli-install#req:host-identity-from-catalog).
		panic(fmt.Sprintf("cliinstall: no catalog entry for %q", "ingitdb"))
	}
	return entry.Config(ver)
}

// SelfUpdate returns the "self-update" command, built from
// github.com/strongo/cli-helpers/selfupdate/cobracmd against
// ingitdbSelfUpdateConfig's identity.
//
// Note: there is deliberately no "update" alias — `ingitdb update` is the
// SQL UPDATE verb command.
func SelfUpdate(ver string) *cobra.Command {
	return cobracmd.New(ingitdbSelfUpdateConfig(ver), cobracmd.CommandOptions{
		Short:      "Update the installed ingitdb binary in place",
		JSONFormat: true,
		Errors:     selfUpdateErrors{},
	})
}

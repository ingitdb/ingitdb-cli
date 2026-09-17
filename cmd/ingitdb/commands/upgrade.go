package commands

// specscore: feature/cli/install

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/strongo/cli-helpers/cliinstall"
	"github.com/strongo/cli-helpers/cliinstall/cobracmd"
	"github.com/strongo/cli-helpers/selfupdate"
)

// Upgrade returns the "upgrade" command, built from
// github.com/strongo/cli-helpers/cliinstall/cobracmd against ingitdb's own
// catalog id. HostConfig comes from the exact same ingitdbSelfUpdateConfig
// helper SelfUpdate uses, so `ingitdb self-update` and
// `ingitdb upgrade ingitdb` reach the exact same library call
// (cli-install#req:self-update-equals-upgrade-self). No "update" alias:
// `ingitdb update` is, and stays, the SQL UPDATE verb command
// (cli-install#req:update-alias-policy: "ingitdb MUST NOT gain one, because
// its `update` command edits records").
func Upgrade(ver string) *cobra.Command {
	return cobracmd.NewUpgrade(cobracmd.UpgradeCommandOptions{
		Short:      "Upgrade installed fleet CLIs, including ingitdb itself",
		Errors:     upgradeErrors{},
		HostID:     "ingitdb",
		HostConfig: ingitdbSelfUpdateConfig(ver),
		// No HostAfterUpdate: ingitdb's self-update has no after-update hook
		// (see SelfUpdate), so neither does upgrade ingitdb.
	})
}

// upgradeErrors implements both cobracmd.ErrorMapper and
// cobracmd.UpgradeErrorMapper, keeping the same exit-code contract install
// and self-update already keep (cli-install#req:host-owned-exit-codes).
type upgradeErrors struct{}

// Failure maps every upgrade failure the same way install already maps its
// own ("upgrade:"-prefixed, general exit 1) — for the three cli-install-only
// kinds (cli-install#req:host-owned-exit-codes: "MUST map the three new
// kinds explicitly") AND for every kind self-update also handles: the
// "upgrade:" prefix names the command the user actually ran, never
// self-update's own bare passthrough (task-22 review M2/M6 fleet ruling).
// selfUpdateErrors.Failure is intentionally NOT reused here — unlike
// UpgradesAvailable below, a *selfupdate.Failure/*cliinstall.BatchFailure
// carries no target identity (selfupdate.Failure has only Kind, Path and
// Err, never a catalog id), so Failure has no reliable way to tell "this
// failure was ingitdb's own" from "this failure was some other upgraded
// target's" — always using "upgrade:" is the one choice that is never
// wrong about which command produced the message. The exit code is
// unaffected: main.go's exitCodeForError still falls back to the same
// generic 1 for every kind here, matching self-update's own passthrough
// exit code exactly (cli-install#req:self-update-equals-upgrade-self).
func (upgradeErrors) Failure(err error) error {
	var usage *cobracmd.UsageError
	if errors.As(err, &usage) {
		return errors.New("upgrade: " + err.Error())
	}
	switch selfupdate.KindOf(err) {
	case selfupdate.KindUnknownTarget:
		return errors.New("upgrade: " + err.Error())
	case selfupdate.KindNoInstallDir, selfupdate.KindDestinationExists:
		return errors.New("upgrade: " + err.Error())
	default:
		return errors.New("upgrade: " + err.Error())
	}
}

// UpgradesAvailable always returns ErrSelfUpdateAvailable, exactly like
// selfUpdateErrors.UpdateAvailable, so main.go's exitCodeForError maps it to
// SelfUpdateAvailableExitCode (10) — quietly, the same way `self-update
// --check` already is: main.go's fatalMessage and quietErrorHandler both
// test errors.Is(err, ErrSelfUpdateAvailable) and print nothing for it, so
// `upgrade --check`/`upgrade ingitdb --check` finding an update available
// is silent too, regardless of which or how many targets it names
// (cli-install#req:upgrade-check, cli-install#req:self-update-equals-upgrade-self).
func (upgradeErrors) UpgradesAvailable(_ []cliinstall.UpgradeResult) error {
	return ErrSelfUpdateAvailable
}

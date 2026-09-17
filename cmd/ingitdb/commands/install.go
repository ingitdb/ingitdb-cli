package commands

// specscore: feature/cli/install

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/strongo/cli-helpers/cliinstall/cobracmd"
)

// Install returns the "install" command, built from
// github.com/strongo/cli-helpers/cliinstall/cobracmd against ingitdb's own
// catalog id (cli-install#req:host-identity-from-catalog). `ingitdb install`
// lists the fleet CLIs relevant to ingitdb (datatug, ovdb, synchestra,
// specscore) with their live status, and `ingitdb install <name>...`
// installs them the same way ingitdb itself was installed. cobracmd.New
// panics if "ingitdb" is absent from the compiled catalog — a programming
// error TestInstall_Registration and TestSelfUpdate_PanicsWhenCatalogEntryMissing's
// sibling below both catch, never a runtime state a user sees.
func Install() *cobra.Command {
	return cobracmd.New(cobracmd.CommandOptions{
		Short:  "List and install fleet CLIs relevant to ingitdb",
		Errors: installErrors{},
		HostID: "ingitdb",
	})
}

// installErrors implements cobracmd.ErrorMapper for ingitdb's own install
// command, keeping the same generic-exit-1 contract selfUpdateErrors already
// keeps for self-update (cli-install#req:host-owned-exit-codes): ingitdb has
// no distinct usage or invalid-state exit code beyond its two special cases
// — ValidationFailedExitCode (2) and SelfUpdateAvailableExitCode (10),
// neither of which install can ever produce — so every install failure maps
// onto the same generic exit 1 that already covers every other ingitdb
// error via main.go's exitCodeForError default branch. Every new kind
// cli-install added (selfupdate.KindUnknownTarget, KindNoInstallDir,
// KindDestinationExists) is mapped explicitly below, even though ingitdb's
// own exit code for all of them is the same 1 a bare default branch would
// already produce, per that REQ's "MUST map ... explicitly, never through a
// self-update default branch".
type installErrors struct{}

// Failure maps every install failure onto ingitdb's neutral general-failure
// message, with no "self-update:" prefix; main.exitCodeForError falls back
// to exit 1 for any error without a recognized sentinel, which every branch
// below returns.
//
// cliinstall/cobracmd v0.21.0's own mapFailure short-circuits a nil error
// before ever calling opts.Errors.Failure (see that package's doc comment
// on mapFailure and its TestMapFailure_NeverCallsMapperWithNil), so the
// v0.19.0-era nil-guard this method used to carry is gone.
func (installErrors) Failure(err error) error {
	var usage *cobracmd.UsageError
	if errors.As(err, &usage) {
		// The one class of failure that IS a usage mistake (an invalid
		// --format, or --all combined with names): ingitdb has no
		// dedicated usage exit code, so this is exit 1, chosen explicitly
		// rather than falling through with everything else.
		return errors.New("install: " + err.Error())
	}

	// selfupdate.KindUnknownTarget's underlying error already names the
	// unknown target and lists valid catalog ids
	// (cli-install#req:unknown-target-refused), and
	// KindNoInstallDir/KindDestinationExists carry their own remedy text.
	// Both new kinds, and every self-update-shared kind
	// (KindAmbiguous, KindReleaseLookup, KindDownload, KindChecksum,
	// KindPermission, KindNonInteractive, KindManagedCommand, ...),
	// resolve to the same exit 1 as ingitdb's pre-existing self-update
	// passthrough — no extra wrapping beyond the shared "install:" prefix
	// is needed for any kind.
	return errors.New("install: " + err.Error())
}

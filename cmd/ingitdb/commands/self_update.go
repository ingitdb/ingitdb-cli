package commands

// specscore: feature/cli/self-update

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ingitdb/ingitdb-cli/internal/selfupdate"
)

// ambiguousGuidance is shown when the install method cannot be confidently
// classified. It tells the user the situation is ambiguous and how to update
// manually instead of letting self-update guess.
const ambiguousGuidance = `ingitdb could not determine how this binary was installed, so the install method is ambiguous.
To avoid replacing a binary that may be managed by a package manager, self-update will not modify it.

To update manually, either:
  - re-download the latest release from https://github.com/ingitdb/ingitdb-cli/releases, or
  - upgrade via the package manager you used to install ingitdb
    (brew upgrade --cask ingitdb / snap refresh ingitdb).`

// selfUpdateAvailableExitCode is the `--check` exit code for "an update is
// available" (or the running version is undetermined). It is distinct from 0
// (up to date) and from the generic error exit 1.
const selfUpdateAvailableExitCode = 10

// Package-level test seams for the self-update command. Tests that replace
// these variables MUST NOT run in parallel.
var (
	// detectInstall resolves how the running binary was installed.
	detectInstall = selfupdate.DetectSelf

	// resolveLatest resolves the latest stable release tag from GitHub.
	resolveLatest = func(ctx context.Context) (string, error) {
		r := selfupdate.Resolver{}
		return r.LatestStableTag(ctx)
	}

	// isInteractive reports whether the process is attached to an interactive
	// terminal. The default inspects stdin's mode: a character device
	// indicates a TTY rather than a pipe/file.
	isInteractive = func() bool {
		fi, err := os.Stdin.Stat()
		if err != nil {
			return false
		}
		return fi.Mode()&os.ModeCharDevice != 0
	}

	// osExecutableFn resolves the running executable's path.
	osExecutableFn = os.Executable

	// selfupdateDownloadFn downloads and sha256-verifies the release asset for
	// the host OS/arch, returning the path of the extracted binary.
	selfupdateDownloadFn = func(ctx context.Context, version string) (string, error) {
		d := selfupdate.Downloader{}
		return d.DownloadAndVerify(ctx, version, "", "")
	}

	// selfupdateReplaceFn atomically swaps the new binary into place.
	selfupdateReplaceFn = selfupdate.ReplaceExecutable

	// selfupdateVerifyVerFn runs the swapped-in binary's version subcommand as
	// a best-effort post-swap sanity check.
	selfupdateVerifyVerFn = selfupdate.VerifyBinaryVersion
)

// doSelfReplace performs the actual download → verify → swap for a manual
// install, replacing the running executable with the release identified by
// tag. It is a package-level variable so tests can substitute a spy and never
// touch the network or filesystem. The download/verify step runs before any
// swap, so a failure there leaves the binary untouched.
var doSelfReplace = func(ctx context.Context, tag string) error {
	target, err := osExecutableFn()
	if err != nil {
		return fmt.Errorf("self-update: could not resolve the running executable: %w", err)
	}
	trimmed := strings.TrimPrefix(tag, "v")
	tmp, err := selfupdateDownloadFn(ctx, trimmed)
	if err != nil {
		return err
	}
	if err := selfupdateReplaceFn(target, tmp); err != nil {
		return err
	}
	// Best-effort post-swap sanity check; a mismatch is not fatal because the
	// swap has already succeeded.
	_ = selfupdateVerifyVerFn(target, trimmed)
	return nil
}

// SelfUpdate returns the "self-update" command, which updates the installed
// ingitdb binary in place.
//
// Package-managed installs (Homebrew cask, Snap) are never overwritten: the
// command prints the manager's exact upgrade command instead. Manual installs
// (release archive, go install) are updated in place by downloading the
// matching release asset, verifying its sha256 against the release's per-OS
// checksum file, and atomically replacing the executable. The --check flag
// reports availability without modifying anything; --yes (-y) skips the
// confirmation prompt; --version pins an exact release tag;
// --allow-downgrade permits a pinned target older than the running build.
//
// ver is the running build's version (the value pinned by goreleaser ldflags,
// "dev" otherwise). exitCode is the process-exit seam (os.Exit in production,
// captured in tests): --check uses it to exit 10 when an update is available
// or the running version is undetermined.
//
// Note: there is deliberately no "update" alias — `ingitdb update` is the SQL
// UPDATE verb command.
func SelfUpdate(ver string, exitCode func(int)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "self-update",
		Short: "Update the installed ingitdb binary in place",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			detection, err := detectInstall()
			if err != nil {
				return err
			}

			if check, _ := cmd.Flags().GetBool("check"); check {
				return runSelfUpdateCheck(cmd, ver, detection, exitCode)
			}

			switch detection.Method {
			case selfupdate.Managed:
				// Redirect to the owning package manager. No filesystem
				// writes/downloads happen on this path; we print and exit 0.
				upgrade, ok := selfupdate.UpgradeCommand(detection.Manager)
				if !ok {
					return errors.New("self-update: managed install detected but no upgrade command is known")
				}
				managerName := selfupdate.ManagerName(detection.Manager)
				_, _ = fmt.Fprintf(cmd.OutOrStdout(),
					"ingitdb was installed via %s. Run the following to upgrade:\n\n    %s\n",
					managerName, upgrade)
				return nil
			case selfupdate.Manual:
				return runSelfReplace(cmd, ver)
			default:
				// Ambiguous: refuse to self-replace, print manual-update
				// guidance, and exit non-zero. Ambiguity must never resolve to
				// the self-replace path, so we return before any download/write.
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), ambiguousGuidance)
				return errors.New("self-update: install method is ambiguous; refusing to self-replace")
			}
		},
	}
	cmd.Flags().Bool("check", false, "report whether a newer release is available without applying it")
	cmd.Flags().BoolP("yes", "y", false, "skip the interactive confirmation prompt")
	// --version here is self-update-local: it pins the release tag to install
	// (leading "v" optional). It is distinct from the `ingitdb version`
	// command, which prints the CLI's own build version.
	cmd.Flags().String("version", "", "install a specific release tag (leading \"v\" optional) instead of the latest")
	cmd.Flags().Bool("allow-downgrade", false, "permit installing a --version older than the running build")
	return cmd
}

// runSelfReplace handles the manual-install action path: pinned (--version)
// or latest-stable target, confirmation, then download → verify → swap.
func runSelfReplace(cmd *cobra.Command, ver string) error {
	out := cmd.OutOrStdout()

	pinned, _ := cmd.Flags().GetString("version")
	pinnedTag := strings.TrimSpace(pinned)
	if pinnedTag != "" {
		// Pinned target: install exactly this tag, bypassing the stable-only
		// latest resolution so an explicitly requested prerelease installs
		// as-is.
		//
		// Downgrade guard: when the running version is known (not the "dev"
		// placeholder) and the pinned target is strictly lower, refuse unless
		// --allow-downgrade is set. Direction can't be determined for a dev
		// build, so the guard does not trigger there.
		allowDowngrade, _ := cmd.Flags().GetBool("allow-downgrade")
		isDowngrade := ver != selfupdate.DevVersion &&
			selfupdate.CompareVersions(pinnedTag, ver) < 0
		if isDowngrade && !allowDowngrade {
			return fmt.Errorf(
				"self-update: refusing to downgrade from %s to %s; pass --allow-downgrade to proceed",
				ver, pinnedTag)
		}

		if isDowngrade {
			_, _ = fmt.Fprintf(out, "downgrade: %s → %s\n", ver, pinnedTag)
		} else {
			_, _ = fmt.Fprintf(out, "%s → %s\n", ver, pinnedTag)
		}

		proceed, err := confirmSelfReplace(cmd)
		if err != nil {
			return err
		}
		if !proceed {
			return nil
		}

		if err := doSelfReplace(cmd.Context(), pinnedTag); err != nil {
			// Annotate with the pinned tag so an unknown-tag failure (no
			// matching published release or asset) names the tag the user
			// requested. The download/verify step runs before any swap, so a
			// failure here leaves the binary untouched.
			wrapped := fmt.Errorf("release %s not found: %w", pinnedTag, err)
			return classifySelfReplaceError(wrapped)
		}
		_, _ = fmt.Fprintf(out, "ingitdb updated to %s.\n", pinnedTag)
		return nil
	}

	latest, err := resolveLatest(cmd.Context())
	if err != nil {
		return fmt.Errorf("self-update: could not resolve the latest release: %w", err)
	}
	result := selfupdate.Compare(ver, latest)
	if result.Verdict == selfupdate.UpToDate {
		// Already on the latest stable release. Report and exit 0 without
		// downloading or replacing anything.
		_, _ = fmt.Fprintf(out, "ingitdb is already up to date (%s).\n", result.Current)
		return nil
	}

	// Available/Undetermined: confirm (unless --yes), then swap.
	_, _ = fmt.Fprintf(out, "%s → %s\n", result.Current, result.Latest)
	proceed, err := confirmSelfReplace(cmd)
	if err != nil {
		return err
	}
	if !proceed {
		return nil
	}

	if err := doSelfReplace(cmd.Context(), latest); err != nil {
		return classifySelfReplaceError(err)
	}
	_, _ = fmt.Fprintf(out, "ingitdb updated to %s.\n", result.Latest)
	return nil
}

// confirmSelfReplace applies the confirmation contract for the self-replace
// path: --yes proceeds; otherwise an interactive terminal is prompted, and a
// non-interactive run is refused with a non-nil error. It returns
// (false, nil) when the user declines at the prompt.
func confirmSelfReplace(cmd *cobra.Command) (bool, error) {
	yes, _ := cmd.Flags().GetBool("yes")
	if yes {
		return true, nil
	}
	if !isInteractive() {
		// Refuse to block on input when there's no terminal and no explicit
		// consent. Leave the binary unchanged.
		return false, errors.New("self-update: --yes is required for non-interactive use; refusing to replace the binary")
	}
	out := cmd.OutOrStdout()
	_, _ = fmt.Fprint(out, "Proceed? [y/N] ")
	reader := bufio.NewReader(cmd.InOrStdin())
	line, _ := reader.ReadString('\n')
	trimmed := strings.TrimSpace(line)
	answer := strings.ToLower(trimmed)
	if answer != "y" && answer != "yes" {
		_, _ = fmt.Fprintln(out, "self-update: aborted; binary left unchanged.")
		return false, nil
	}
	return true, nil
}

// classifySelfReplaceError converts a non-nil error from doSelfReplace into a
// clear, actionable error, distinguishing the two write-path failure modes.
// The atomic swap is gated behind download + verification, so on any of these
// errors the original binary is untouched.
//
// Permission-denied: the executable's directory is not writable. The error
// names the executable path (best-effort via os.Executable) and suggests a
// remedy. Anything else (connection refused, rate limit, missing asset,
// checksum mismatch) is reported as a release/download failure.
func classifySelfReplaceError(err error) error {
	if errors.Is(err, fs.ErrPermission) {
		target, exeErr := osExecutableFn()
		if exeErr != nil || target == "" {
			target = "the ingitdb executable"
		}
		return fmt.Errorf(
			"self-update: permission denied writing %s; re-run with elevated permissions (sudo) or update via your package manager",
			target)
	}
	return fmt.Errorf("self-update: failed to download the release: %w", err)
}

// runSelfUpdateCheck implements the read-only --check mode for any install
// method. It resolves the latest stable release, reports availability and the
// appropriate next step, and performs no download or filesystem write.
// Exit-code contract: up-to-date returns nil (exit 0); available/undetermined
// exits 10 via the exitCode seam; a release-lookup failure returns a non-nil
// error (generic non-zero exit, distinct from 10).
func runSelfUpdateCheck(cmd *cobra.Command, ver string, detection selfupdate.Detection, exitCode func(int)) error {
	latest, err := resolveLatest(cmd.Context())
	if err != nil {
		return fmt.Errorf("self-update --check: could not resolve the latest release: %w", err)
	}

	result := selfupdate.Compare(ver, latest)
	out := cmd.OutOrStdout()

	switch result.Verdict {
	case selfupdate.UpToDate:
		_, _ = fmt.Fprintf(out, "ingitdb is up to date (%s).\n", result.Current)
		return nil
	case selfupdate.Undetermined:
		_, _ = fmt.Fprintf(out, "current ingitdb version is undetermined (%s); latest stable is %s.\n", result.Current, result.Latest)
	default:
		_, _ = fmt.Fprintf(out, "update available: %s → %s\n", result.Current, result.Latest)
	}

	// Print the appropriate next step for the detected install method. No
	// download or write happens on any of these paths.
	switch detection.Method {
	case selfupdate.Managed:
		if upgrade, ok := selfupdate.UpgradeCommand(detection.Manager); ok {
			managerName := selfupdate.ManagerName(detection.Manager)
			_, _ = fmt.Fprintf(out,
				"ingitdb was installed via %s. Run the following to upgrade:\n\n    %s\n",
				managerName, upgrade)
		}
	case selfupdate.Manual:
		_, _ = fmt.Fprintln(out, "To upgrade, run: ingitdb self-update")
	default:
		_, _ = fmt.Fprintln(out, ambiguousGuidance)
	}

	exitCode(selfUpdateAvailableExitCode)
	return nil
}

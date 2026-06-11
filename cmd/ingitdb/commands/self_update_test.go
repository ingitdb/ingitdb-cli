package commands

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-cli/internal/selfupdate"
)

// exitRecorder captures calls to the command's exit-code seam.
type exitRecorder struct{ codes []int }

func (r *exitRecorder) fn(code int) { r.codes = append(r.codes, code) }

// runSelfUpdate executes the self-update command with the given running
// version and args, returning stdout, stderr, the recorded exit codes, and
// the Execute error.
func runSelfUpdate(t *testing.T, ver string, args ...string) (string, string, *exitRecorder, error) {
	t.Helper()
	rec := &exitRecorder{}
	cmd := SelfUpdate(ver, rec.fn)
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), errOut.String(), rec, err
}

// withDetection overrides the package-level detection seam for the duration
// of the test and restores it afterward, so tests never depend on the real
// os.Executable path. Tests using it must not run in parallel.
func withDetection(t *testing.T, d selfupdate.Detection) {
	t.Helper()
	prev := detectInstall
	detectInstall = func() (selfupdate.Detection, error) { return d, nil }
	t.Cleanup(func() { detectInstall = prev })
}

// withLatest overrides the package-level release-resolution seam so tests
// never hit the network. Tests using it must not run in parallel.
func withLatest(t *testing.T, tag string, err error) {
	t.Helper()
	prev := resolveLatest
	resolveLatest = func(context.Context) (string, error) { return tag, err }
	t.Cleanup(func() { resolveLatest = prev })
}

// withInteractive overrides the package-level TTY-detection seam so
// confirmation tests never depend on the test runner's stdin. Tests using it
// must not run in parallel.
func withInteractive(t *testing.T, interactive bool) {
	t.Helper()
	prev := isInteractive
	isInteractive = func() bool { return interactive }
	t.Cleanup(func() { isInteractive = prev })
}

// withSelfReplace overrides the package-level self-replace seam with a spy so
// tests never download or replace anything. It returns a pointer to a bool
// recording whether the seam was invoked. Tests using it must not run in
// parallel.
func withSelfReplace(t *testing.T, err error) *bool {
	t.Helper()
	called := false
	prev := doSelfReplace
	doSelfReplace = func(context.Context, string) error {
		called = true
		return err
	}
	t.Cleanup(func() { doSelfReplace = prev })
	return &called
}

// withSelfReplaceTag overrides the self-replace seam with a spy that captures
// the tag it was called with. Tests using it must not run in parallel.
func withSelfReplaceTag(t *testing.T, err error) *string {
	t.Helper()
	var gotTag string
	prev := doSelfReplace
	doSelfReplace = func(_ context.Context, tag string) error {
		gotTag = tag
		return err
	}
	t.Cleanup(func() { doSelfReplace = prev })
	return &gotTag
}

// AC: cli/self-update#ac:canonical-name — the canonical command name is
// "self-update" and there is deliberately NO "update" alias: `ingitdb update`
// is the SQL UPDATE verb command. --check output must be deterministic.
func TestSelfUpdate_CanonicalNameNoUpdateAlias(t *testing.T) {
	withDetection(t, selfupdate.Detection{Method: selfupdate.Managed, Manager: selfupdate.Homebrew})
	withLatest(t, "v1.2.3", nil)

	cmd := SelfUpdate("1.2.3", func(int) {})
	if cmd.Name() != "self-update" {
		t.Errorf("canonical name = %q; want %q", cmd.Name(), "self-update")
	}
	if cmd.HasAlias("update") {
		t.Error("self-update must not alias \"update\": it would collide with the SQL update verb command")
	}

	out, _, _, err := runSelfUpdate(t, "1.2.3", "--check")
	if err != nil {
		t.Fatalf("self-update --check returned error: %v", err)
	}
	if out == "" {
		t.Error("expected deterministic output on stdout, got empty")
	}

	out2, _, _, err2 := runSelfUpdate(t, "1.2.3", "--check")
	if err2 != nil {
		t.Fatalf("second run returned error: %v", err2)
	}
	if out != out2 {
		t.Errorf("output not deterministic: %q != %q", out, out2)
	}
}

// AC: cli/self-update#ac:managed-is-redirected — when the executable lives in
// a Homebrew-cask or Snap managed location, self-update MUST print the
// detected manager and its exact upgrade command, exit 0, and leave the
// executable unchanged (no filesystem writes).
func TestSelfUpdate_ManagedIsRedirected(t *testing.T) {
	cases := []struct {
		name        string
		manager     selfupdate.Manager
		wantName    string
		wantCommand string
	}{
		{"homebrew cask", selfupdate.Homebrew, "Homebrew", "brew upgrade --cask ingitdb"},
		{"snap", selfupdate.Snap, "Snap", "snap refresh ingitdb"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			withDetection(t, selfupdate.Detection{Method: selfupdate.Managed, Manager: c.manager})

			out, _, _, err := runSelfUpdate(t, "1.0.0")
			if err != nil {
				t.Fatalf("managed redirect returned error (want nil/exit 0): %v", err)
			}
			if !strings.Contains(out, c.wantName) {
				t.Errorf("stdout %q does not name detected manager %q", out, c.wantName)
			}
			if !strings.Contains(out, c.wantCommand) {
				t.Errorf("stdout %q does not contain exact upgrade command %q", out, c.wantCommand)
			}
		})
	}
}

// AC: cli/self-update#ac:ambiguous-falls-back-safe — when the install method
// cannot be confidently classified, self-update MUST NOT replace the binary:
// it states the install method is ambiguous, prints manual-update guidance,
// and exits non-zero.
func TestSelfUpdate_AmbiguousFallsBackSafe(t *testing.T) {
	withDetection(t, selfupdate.Detection{Method: selfupdate.Ambiguous, Manager: selfupdate.ManagerNone})
	called := withSelfReplace(t, nil)

	out, errOut, rec, err := runSelfUpdate(t, "1.0.0")
	if err == nil {
		t.Fatal("expected non-nil error for ambiguous detection (must exit non-zero)")
	}
	if *called {
		t.Error("doSelfReplace was called; ambiguity must never resolve to self-replace")
	}
	if len(rec.codes) != 0 {
		t.Errorf("exitCode seam called with %v; the ambiguous path must not use the --check exit code", rec.codes)
	}

	combined := strings.ToLower(out + errOut + err.Error())
	if !strings.Contains(combined, "ambiguous") {
		t.Errorf("output/error %q does not state the install method is ambiguous", combined)
	}
	if !strings.Contains(combined, "github.com") {
		t.Errorf("output/error %q does not contain manual-update guidance", combined)
	}
}

// Extra positional args must be rejected to keep the call shape stable.
func TestSelfUpdate_RejectsExtraArgs(t *testing.T) {
	withDetection(t, selfupdate.Detection{Method: selfupdate.Manual, Manager: selfupdate.ManagerNone})
	_, _, _, err := runSelfUpdate(t, "1.0.0", "extra-positional")
	if err == nil {
		t.Fatal("expected error for extra positional argument")
	}
}

// AC: cli/self-update#ac:check-is-readonly — for any install method, running
// self-update --check with a newer release available MUST print availability
// and the appropriate next step, and MUST NOT download or replace the binary.
func TestSelfUpdate_CheckIsReadonly(t *testing.T) {
	withDetection(t, selfupdate.Detection{Method: selfupdate.Manual, Manager: selfupdate.ManagerNone})
	withLatest(t, "v1.1.0", nil)
	called := withSelfReplace(t, nil)

	out, _, rec, err := runSelfUpdate(t, "1.0.0", "--check")
	if err != nil {
		t.Fatalf("--check returned error: %v", err)
	}
	if *called {
		t.Error("doSelfReplace was called; --check must be read-only")
	}
	if len(rec.codes) != 1 || rec.codes[0] != 10 {
		t.Errorf("exit codes = %v; want [10] when an update is available", rec.codes)
	}

	lower := strings.ToLower(out)
	if !strings.Contains(lower, "1.0.0") || !strings.Contains(lower, "1.1.0") {
		t.Errorf("stdout %q does not report availability (current → latest)", out)
	}
	if !strings.Contains(lower, "self-update") {
		t.Errorf("stdout %q does not name the manual self-update next step", out)
	}
}

// AC: cli/self-update#ac:check-exit-code-contract — --check exit codes MUST be
// 0 (up to date), 10 (update available or undetermined), and a distinct
// non-zero code for a release-lookup error (the generic error exit).
func TestSelfUpdate_CheckExitCodeContract(t *testing.T) {
	t.Run("up to date → exit 0", func(t *testing.T) {
		withDetection(t, selfupdate.Detection{Method: selfupdate.Manual, Manager: selfupdate.ManagerNone})
		withLatest(t, "v2.0.0", nil)

		_, _, rec, err := runSelfUpdate(t, "2.0.0", "--check")
		if err != nil {
			t.Fatalf("up-to-date --check returned error (want nil/exit 0): %v", err)
		}
		if len(rec.codes) != 0 {
			t.Errorf("exit codes = %v; want none (process exits 0)", rec.codes)
		}
	})

	t.Run("update available → exit 10", func(t *testing.T) {
		withDetection(t, selfupdate.Detection{Method: selfupdate.Manual, Manager: selfupdate.ManagerNone})
		withLatest(t, "v2.1.0", nil)

		_, _, rec, err := runSelfUpdate(t, "2.0.0", "--check")
		if err != nil {
			t.Fatalf("update-available --check returned error (want exit via seam): %v", err)
		}
		if len(rec.codes) != 1 || rec.codes[0] != 10 {
			t.Errorf("exit codes = %v; want [10]", rec.codes)
		}
	})

	t.Run("dev build → undetermined, exit 10", func(t *testing.T) {
		withDetection(t, selfupdate.Detection{Method: selfupdate.Manual, Manager: selfupdate.ManagerNone})
		withLatest(t, "v2.1.0", nil)

		out, _, rec, err := runSelfUpdate(t, "dev", "--check")
		if err != nil {
			t.Fatalf("dev --check returned error: %v", err)
		}
		if len(rec.codes) != 1 || rec.codes[0] != 10 {
			t.Errorf("exit codes = %v; want [10]", rec.codes)
		}
		if !strings.Contains(strings.ToLower(out), "undetermined") {
			t.Errorf("stdout %q does not report the version as undetermined", out)
		}
	})

	t.Run("release-lookup error → distinct non-zero", func(t *testing.T) {
		withDetection(t, selfupdate.Detection{Method: selfupdate.Manual, Manager: selfupdate.ManagerNone})
		withLatest(t, "", errors.New("github releases request failed"))

		_, _, rec, err := runSelfUpdate(t, "2.0.0", "--check")
		if err == nil {
			t.Fatal("release-lookup error --check returned nil (want non-nil error → exit 1)")
		}
		if len(rec.codes) != 0 {
			t.Errorf("exit codes = %v; a lookup error must not exit 10", rec.codes)
		}
	})
}

// AC: cli/self-update#ac:already-current-noop — a manual install already on
// the latest stable release MUST report it is up to date and exit 0 without
// downloading or replacing anything.
func TestSelfUpdate_AlreadyCurrentNoop(t *testing.T) {
	withDetection(t, selfupdate.Detection{Method: selfupdate.Manual, Manager: selfupdate.ManagerNone})
	withLatest(t, "v1.2.3", nil)
	called := withSelfReplace(t, nil)

	out, _, _, err := runSelfUpdate(t, "1.2.3")
	if err != nil {
		t.Fatalf("already-current self-update returned error (want nil/exit 0): %v", err)
	}
	if *called {
		t.Error("doSelfReplace was called; an up-to-date install must not be touched")
	}
	if !strings.Contains(strings.ToLower(out), "up to date") {
		t.Errorf("stdout %q does not report the binary is up to date", out)
	}
}

// AC: cli/self-update#ac:confirm-prompt-and-yes — with --yes the replacement
// runs without prompting, after printing the current → latest transition.
func TestSelfUpdate_ConfirmPromptAndYes_WithYes(t *testing.T) {
	withDetection(t, selfupdate.Detection{Method: selfupdate.Manual, Manager: selfupdate.ManagerNone})
	withLatest(t, "v1.1.0", nil)
	withInteractive(t, false) // --yes must work regardless of TTY state
	called := withSelfReplace(t, nil)

	out, _, _, err := runSelfUpdate(t, "1.0.0", "--yes")
	if err != nil {
		t.Fatalf("self-update --yes returned error (want nil): %v", err)
	}
	if !*called {
		t.Error("doSelfReplace was not called with --yes")
	}
	if !strings.Contains(out, "→") || !strings.Contains(out, "1.0.0") || !strings.Contains(out, "1.1.0") {
		t.Errorf("stdout %q does not contain the current → latest transition", out)
	}
}

// AC: cli/self-update#ac:confirm-prompt-and-yes — without --yes but attached
// to an interactive terminal, the command prompts and (on "y") proceeds.
func TestSelfUpdate_ConfirmPromptAndYes_InteractiveConfirms(t *testing.T) {
	withDetection(t, selfupdate.Detection{Method: selfupdate.Manual, Manager: selfupdate.ManagerNone})
	withLatest(t, "v1.1.0", nil)
	withInteractive(t, true)
	called := withSelfReplace(t, nil)

	rec := &exitRecorder{}
	cmd := SelfUpdate("1.0.0", rec.fn)
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetIn(strings.NewReader("y\n"))
	cmd.SetArgs(nil)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("interactive confirm returned error (want nil): %v", err)
	}
	if !*called {
		t.Error("doSelfReplace was not called after interactive confirmation")
	}
	lower := strings.ToLower(out.String())
	if !strings.Contains(lower, "proceed") {
		t.Errorf("stdout %q does not contain a confirmation prompt", out.String())
	}
	if !strings.Contains(out.String(), "→") {
		t.Errorf("stdout %q does not contain the current → latest transition", out.String())
	}
}

// Declining the interactive prompt aborts without replacing, exit 0.
func TestSelfUpdate_InteractiveDeclineAborts(t *testing.T) {
	withDetection(t, selfupdate.Detection{Method: selfupdate.Manual, Manager: selfupdate.ManagerNone})
	withLatest(t, "v1.1.0", nil)
	withInteractive(t, true)
	called := withSelfReplace(t, nil)

	cmd := SelfUpdate("1.0.0", func(int) {})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetIn(strings.NewReader("n\n"))
	cmd.SetArgs(nil)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("declined confirm returned error (want nil): %v", err)
	}
	if *called {
		t.Error("doSelfReplace was called after the user declined")
	}
	if !strings.Contains(strings.ToLower(out.String()), "aborted") {
		t.Errorf("stdout %q does not report the abort", out.String())
	}
}

// AC: cli/self-update#ac:noninteractive-without-yes-refuses — without --yes
// and without a terminal, the command refuses to replace and exits non-zero.
func TestSelfUpdate_NonInteractiveWithoutYesRefuses(t *testing.T) {
	withDetection(t, selfupdate.Detection{Method: selfupdate.Manual, Manager: selfupdate.ManagerNone})
	withLatest(t, "v1.1.0", nil)
	withInteractive(t, false)
	called := withSelfReplace(t, nil)

	out, errOut, _, err := runSelfUpdate(t, "1.0.0")
	if err == nil {
		t.Fatal("expected non-nil error for non-interactive run without --yes")
	}
	if *called {
		t.Error("doSelfReplace was called; binary must be left unchanged")
	}
	combined := strings.ToLower(out + errOut + err.Error())
	if !strings.Contains(combined, "--yes") {
		t.Errorf("output/error %q does not mention that --yes is required", combined)
	}
	if !strings.Contains(combined, "non-interactive") && !strings.Contains(combined, "noninteractive") {
		t.Errorf("output/error %q does not mention non-interactive use", combined)
	}
}

// AC: cli/self-update#ac:network-failure-is-safe — an unreachable release
// source MUST produce a clear error, a non-zero exit, and no modification.
func TestSelfUpdate_NetworkFailureIsSafe(t *testing.T) {
	t.Run("release lookup fails", func(t *testing.T) {
		withDetection(t, selfupdate.Detection{Method: selfupdate.Manual, Manager: selfupdate.ManagerNone})
		withLatest(t, "", errors.New("dial tcp: connection refused"))
		called := withSelfReplace(t, nil)

		out, errOut, _, err := runSelfUpdate(t, "1.0.0", "--yes")
		if err == nil {
			t.Fatal("expected non-nil error when the release source is unreachable")
		}
		if *called {
			t.Error("doSelfReplace was called; the binary must be left unchanged on a lookup failure")
		}
		combined := strings.ToLower(out + errOut + err.Error())
		if !strings.Contains(combined, "release") {
			t.Errorf("output/error %q does not mention the release-lookup failure", combined)
		}
	})

	t.Run("download fails", func(t *testing.T) {
		withDetection(t, selfupdate.Detection{Method: selfupdate.Manual, Manager: selfupdate.ManagerNone})
		withLatest(t, "v1.1.0", nil)
		withInteractive(t, false)
		// A network-ish, non-permission error from the download/verify step.
		_ = withSelfReplace(t, errors.New("dial tcp: connection refused"))

		out, errOut, _, err := runSelfUpdate(t, "1.0.0", "--yes")
		if err == nil {
			t.Fatal("expected non-nil error when the asset download fails")
		}
		combined := strings.ToLower(out + errOut + err.Error())
		if !strings.Contains(combined, "download") && !strings.Contains(combined, "release") {
			t.Errorf("output/error %q does not mention the download/release failure", combined)
		}
		// The happy-path "updated to" line must NOT appear: nothing was replaced.
		if strings.Contains(strings.ToLower(out), "updated to") {
			t.Errorf("stdout %q claims an update succeeded after a download failure", out)
		}
	})
}

// AC: cli/self-update#ac:permission-denied-is-safe — a non-writable install
// location MUST be reported with the path and a suggested remedy, exit
// non-zero, leaving the original binary intact.
func TestSelfUpdate_PermissionDeniedIsSafe(t *testing.T) {
	withDetection(t, selfupdate.Detection{Method: selfupdate.Manual, Manager: selfupdate.ManagerNone})
	withLatest(t, "v1.1.0", nil)
	withInteractive(t, false)
	wrapped := fmt.Errorf("rename: %w", fs.ErrPermission)
	_ = withSelfReplace(t, wrapped)

	out, errOut, _, err := runSelfUpdate(t, "1.0.0", "--yes")
	if err == nil {
		t.Fatal("expected non-nil error on a permission-denied replacement")
	}
	combined := strings.ToLower(out + errOut + err.Error())
	if !strings.Contains(combined, "permission") {
		t.Errorf("output/error %q does not report a permission failure", combined)
	}
	// A remedy hint: elevated permissions (sudo) or the package manager.
	if !strings.Contains(combined, "sudo") && !strings.Contains(combined, "package manager") {
		t.Errorf("output/error %q does not suggest a remedy (sudo / package manager)", combined)
	}
	// The error must reference a path. os.Executable() should resolve in the
	// test process; assert a path separator is present as a proxy.
	if !strings.Contains(combined, "/") && !strings.Contains(combined, `\`) {
		t.Errorf("output/error %q does not include the executable path", combined)
	}
	if strings.Contains(strings.ToLower(out), "updated to") {
		t.Errorf("stdout %q claims an update succeeded after a permission failure", out)
	}
}

// AC: cli/self-update#ac:version-flag-selects-tag — `--version 0.0.3 --yes`
// MUST install exactly that release (tag accepted with or without the leading
// v), bypassing the stable-only latest resolver for the target.
func TestSelfUpdate_VersionFlagSelectsTag(t *testing.T) {
	withDetection(t, selfupdate.Detection{Method: selfupdate.Manual, Manager: selfupdate.ManagerNone})
	withLatest(t, "v9.9.9", nil) // sentinel: must NOT become the install target
	withInteractive(t, false)    // --yes must work regardless of TTY
	gotTag := withSelfReplaceTag(t, nil)

	out, _, _, err := runSelfUpdate(t, "0.0.1", "--version", "0.0.3", "--yes")
	if err != nil {
		t.Fatalf("pinned self-update returned error (want nil): %v", err)
	}
	if *gotTag != "0.0.3" {
		t.Errorf("doSelfReplace tag = %q; want pinned %q (not the sentinel latest)", *gotTag, "0.0.3")
	}
	if !strings.Contains(out, "→") || !strings.Contains(out, "0.0.1") || !strings.Contains(out, "0.0.3") {
		t.Errorf("stdout %q does not contain the current → pinned transition", out)
	}
	if strings.Contains(out, "9.9.9") {
		t.Errorf("stdout %q references the sentinel latest; pinned path must bypass resolveLatest", out)
	}
}

// AC: cli/self-update#ac:pinned-tag-allows-prerelease — a pinned prerelease
// installs exactly, even though the unpinned latest path would skip it.
func TestSelfUpdate_PinnedTagAllowsPrerelease(t *testing.T) {
	withDetection(t, selfupdate.Detection{Method: selfupdate.Manual, Manager: selfupdate.ManagerNone})
	withLatest(t, "v0.0.2", nil) // sentinel stable latest; prerelease would be skipped here
	withInteractive(t, false)
	gotTag := withSelfReplaceTag(t, nil)

	out, _, _, err := runSelfUpdate(t, "0.0.1", "--version", "v0.1.0-rc.1", "--yes")
	if err != nil {
		t.Fatalf("pinned prerelease self-update returned error (want nil): %v", err)
	}
	if *gotTag != "v0.1.0-rc.1" {
		t.Errorf("doSelfReplace tag = %q; want pinned prerelease %q", *gotTag, "v0.1.0-rc.1")
	}
	if !strings.Contains(out, "→") || !strings.Contains(out, "v0.1.0-rc.1") {
		t.Errorf("stdout %q does not contain the current → pinned-prerelease transition", out)
	}
}

// AC: cli/self-update#ac:downgrade-requires-flag — a pinned target lower than
// the running version is refused without --allow-downgrade; with the flag the
// downgrade proceeds; a dev build cannot determine direction so the guard
// does not trigger.
func TestSelfUpdate_DowngradeRequiresFlag(t *testing.T) {
	t.Run("refuses without flag", func(t *testing.T) {
		withDetection(t, selfupdate.Detection{Method: selfupdate.Manual, Manager: selfupdate.ManagerNone})
		withInteractive(t, false)
		called := withSelfReplace(t, nil)

		out, errOut, _, err := runSelfUpdate(t, "v0.5.0", "--version", "v0.3.0")
		if err == nil {
			t.Fatal("expected non-nil error refusing the downgrade")
		}
		if *called {
			t.Error("doSelfReplace was called; binary must be left unchanged on a refused downgrade")
		}
		combined := out + errOut + err.Error()
		if !strings.Contains(combined, "0.5.0") {
			t.Errorf("output/error %q does not name the current version 0.5.0", combined)
		}
		if !strings.Contains(combined, "0.3.0") {
			t.Errorf("output/error %q does not name the target version 0.3.0", combined)
		}
		if !strings.Contains(combined, "--allow-downgrade") {
			t.Errorf("output/error %q does not mention the --allow-downgrade flag", combined)
		}
	})

	t.Run("proceeds with --allow-downgrade --yes", func(t *testing.T) {
		withDetection(t, selfupdate.Detection{Method: selfupdate.Manual, Manager: selfupdate.ManagerNone})
		withInteractive(t, false)
		gotTag := withSelfReplaceTag(t, nil)

		out, _, _, err := runSelfUpdate(t, "v0.5.0", "--version", "v0.3.0", "--allow-downgrade", "--yes")
		if err != nil {
			t.Fatalf("downgrade with --allow-downgrade --yes returned error (want nil): %v", err)
		}
		if *gotTag != "v0.3.0" {
			t.Errorf("doSelfReplace tag = %q; want downgrade target %q", *gotTag, "v0.3.0")
		}
		if !strings.Contains(strings.ToLower(out), "downgrade") {
			t.Errorf("stdout %q does not indicate a downgrade transition", out)
		}
	})

	t.Run("dev current does not trigger guard", func(t *testing.T) {
		withDetection(t, selfupdate.Detection{Method: selfupdate.Manual, Manager: selfupdate.ManagerNone})
		withInteractive(t, false)
		called := withSelfReplace(t, nil)

		_, _, _, err := runSelfUpdate(t, "dev", "--version", "v0.3.0", "--yes")
		if err != nil {
			t.Fatalf("dev current pinned install returned error (want nil): %v", err)
		}
		if !*called {
			t.Error("doSelfReplace was not called; the guard must not trigger for a dev build")
		}
	})
}

// AC: cli/self-update#ac:pinned-unknown-tag-errors — a pinned tag with no
// matching published release or asset (e.g. a release missing the darwin
// assets) MUST print a clear error, exit non-zero, and leave the existing
// binary untouched.
func TestSelfUpdate_PinnedUnknownTagErrors(t *testing.T) {
	withDetection(t, selfupdate.Detection{Method: selfupdate.Manual, Manager: selfupdate.ManagerNone})
	withInteractive(t, false) // --yes must work regardless of TTY
	// Simulate the download/verify step failing because the pinned release/
	// asset does not exist. The error deliberately omits the tag so the
	// assertion proves the CLI layer itself surfaces the pinned tag.
	_ = withSelfReplace(t, errors.New("no matching release or asset"))

	out, errOut, _, err := runSelfUpdate(t, "1.0.0", "--version", "v9.9.9", "--yes")
	if err == nil {
		t.Fatal("expected non-nil error for an unknown pinned tag")
	}
	combined := strings.ToLower(out + errOut + err.Error())
	if !strings.Contains(combined, "v9.9.9") && !strings.Contains(combined, "not found") {
		t.Errorf("output/error %q does not clearly reference the unknown tag or 'not found'", combined)
	}
	// Nothing was replaced: the happy-path "updated to" line must not appear.
	if strings.Contains(strings.ToLower(out), "updated to") {
		t.Errorf("stdout %q claims an update succeeded for an unknown tag", out)
	}
}

// AC: cli/self-update#ac:pinned-managed-still-redirects — a managed install
// run with --version still follows the redirect path: it prints the manager
// and its upgrade command, exits 0, and never self-replaces.
func TestSelfUpdate_PinnedManagedStillRedirects(t *testing.T) {
	cases := []struct {
		name        string
		manager     selfupdate.Manager
		wantName    string
		wantCommand string
	}{
		{"homebrew cask", selfupdate.Homebrew, "Homebrew", "brew upgrade --cask ingitdb"},
		{"snap", selfupdate.Snap, "Snap", "snap refresh ingitdb"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			withDetection(t, selfupdate.Detection{Method: selfupdate.Managed, Manager: c.manager})
			called := withSelfReplace(t, nil)

			out, _, _, err := runSelfUpdate(t, "1.0.0", "--version", "v0.0.3")
			if err != nil {
				t.Fatalf("pinned managed redirect returned error (want nil/exit 0): %v", err)
			}
			if *called {
				t.Error("doSelfReplace was called; a managed install must redirect, never self-replace")
			}
			if !strings.Contains(out, c.wantName) {
				t.Errorf("stdout %q does not name detected manager %q", out, c.wantName)
			}
			if !strings.Contains(out, c.wantCommand) {
				t.Errorf("stdout %q does not contain exact upgrade command %q", out, c.wantCommand)
			}
		})
	}
}

// The --allow-downgrade flag exists and defaults to false.
func TestSelfUpdate_AllowDowngradeFlag(t *testing.T) {
	t.Parallel()
	cmd := SelfUpdate("dev", func(int) {})
	f := cmd.Flags().Lookup("allow-downgrade")
	if f == nil {
		t.Fatal("missing --allow-downgrade flag")
	}
	if f.DefValue != "false" {
		t.Errorf("--allow-downgrade default = %q; want false", f.DefValue)
	}
}

// The --version flag exists as a self-update-local string flag and defaults
// to empty (distinct from the `ingitdb version` command).
func TestSelfUpdate_VersionFlag(t *testing.T) {
	t.Parallel()
	cmd := SelfUpdate("dev", func(int) {})
	v := cmd.Flags().Lookup("version")
	if v == nil {
		t.Fatal("missing --version flag")
	}
	if v.DefValue != "" {
		t.Errorf("--version default = %q; want empty", v.DefValue)
	}
}

// The --yes flag has a -y shorthand and both --check and --yes default false.
func TestSelfUpdate_Flags(t *testing.T) {
	t.Parallel()
	cmd := SelfUpdate("dev", func(int) {})
	check := cmd.Flags().Lookup("check")
	if check == nil {
		t.Fatal("missing --check flag")
	}
	if check.DefValue != "false" {
		t.Errorf("--check default = %q; want false", check.DefValue)
	}
	yes := cmd.Flags().Lookup("yes")
	if yes == nil {
		t.Fatal("missing --yes flag")
	}
	if yes.Shorthand != "y" {
		t.Errorf("--yes shorthand = %q; want y", yes.Shorthand)
	}
	if yes.DefValue != "false" {
		t.Errorf("--yes default = %q; want false", yes.DefValue)
	}
}

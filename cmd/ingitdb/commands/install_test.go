package commands

import (
	"errors"
	"strings"
	"testing"

	"github.com/strongo/cli-helpers/cliinstall/cobracmd"
)

func TestInstall_Registration(t *testing.T) {
	t.Parallel()

	cmd := Install()
	if !strings.HasPrefix(cmd.Use, "install") {
		t.Errorf("Use = %q, want it to start with install", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("Short is empty, want a description")
	}
	for _, name := range []string{"all", "yes", "dry-run", "dir", "format"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("flag %q is not registered", name)
		}
	}
	if f := cmd.Flags().Lookup("yes"); f.Shorthand != "y" {
		t.Errorf("--yes shorthand = %q, want y", f.Shorthand)
	}
}

// installErrorsFailure_NilReturnsNil proves the defensive nil guard:
// cliinstall/cobracmd v0.19.0's runInstall calls mapFailure(opts,
// plan.Failure()) and mapFailure(opts, result.Failure()) unconditionally,
// and both return nil for a fully successful batch (including a successful
// --dry-run), so installErrors.Failure(nil) is a real, reachable call on the
// ordinary success path, not just a defensive guard against a hypothetical
// caller (known cli-helpers v0.19.0 bug, to be fixed in its next release —
// see install.go's doc comment).
func TestInstallErrorsFailure_NilReturnsNil(t *testing.T) {
	t.Parallel()

	if got := (installErrors{}).Failure(nil); got != nil {
		t.Errorf("Failure(nil) = %v, want nil", got)
	}
}

func TestInstallErrorsFailure_UsageError(t *testing.T) {
	t.Parallel()

	usage := &cobracmd.UsageError{Err: errors.New("invalid --format \"yaml\": expected text or json")}
	got := (installErrors{}).Failure(usage)
	if got == nil {
		t.Fatal("Failure(usage error) = nil, want a non-nil error")
	}
	if !strings.Contains(got.Error(), "--format") {
		t.Errorf("Failure(usage error) = %q, want it to mention --format", got.Error())
	}
}

// TestInstallErrorsFailure_MapsEveryKindToOne proves every failure — usage,
// an unknown target, the two new cli-install-only kinds, and a self-update-
// shared kind — maps onto ingitdb's single general-failure exit code, 1
// (cli-install#req:host-owned-exit-codes), via main.exitCodeForError's
// default branch (no ErrValidationFailed / ErrSelfUpdateAvailable sentinel
// is ever produced by install.go).
func TestInstallErrorsFailure_MapsEveryKindToOne(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
	}{
		{"unknown target", errors.New("nosuchcli: not a known install target; valid ids: datatug, ovdb, specscore, synchestra")},
		{"no install dir", errors.New("no per-user bin directory on PATH")},
		{"destination exists", errors.New("destination already exists")},
		{"plain error", errors.New("network unavailable")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := (installErrors{}).Failure(c.err)
			if got == nil {
				t.Fatal("Failure(...) = nil, want a non-nil error")
			}
			if errors.Is(got, ErrValidationFailed) || errors.Is(got, ErrSelfUpdateAvailable) {
				t.Errorf("Failure(%v) = %v, must not match a sentinel with its own exit code", c.err, got)
			}
		})
	}
}

// TestInstallCmdNoSuchTarget_ExitCodeContract runs the real command built
// exactly as main.go wires it (real, un-injected catalog and env) against an
// unknown target name. cliinstall.Plan validates every name against the
// compiled-in catalog BEFORE probing anything
// (cli-install#req:unknown-target-refused: "MUST fail before any
// confirmation, network request or write"), so this is inherently offline —
// no network/env seam is needed to keep it safe for CI.
func TestInstallCmdNoSuchTarget_ExitCodeContract(t *testing.T) {
	t.Parallel()

	cmd := Install()
	cmd.SetOut(&strings.Builder{})
	cmd.SetErr(&strings.Builder{})
	cmd.SetArgs([]string{"nosuchcli"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected a non-nil error for an unknown install target")
	}
	if !strings.Contains(err.Error(), "nosuchcli") {
		t.Errorf("error %q does not name the unknown target", err.Error())
	}
	if errors.Is(err, ErrValidationFailed) || errors.Is(err, ErrSelfUpdateAvailable) {
		t.Errorf("error %v must not match a sentinel with its own exit code (want the generic exit 1)", err)
	}
}

func TestInstallCmdInvalidFormat_IsUsageError(t *testing.T) {
	t.Parallel()

	cmd := Install()
	cmd.SetOut(&strings.Builder{})
	cmd.SetErr(&strings.Builder{})
	cmd.SetArgs([]string{"--format", "yaml"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected a non-nil error for --format yaml")
	}
	if !strings.Contains(err.Error(), "--format") {
		t.Errorf("error %q does not mention --format", err.Error())
	}
}

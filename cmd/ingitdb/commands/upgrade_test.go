package commands

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"strings"
	"testing"

	"github.com/strongo/cli-helpers/cliinstall"
	cliinstallcobracmd "github.com/strongo/cli-helpers/cliinstall/cobracmd"
	"github.com/strongo/cli-helpers/selfupdate"
	selfupdatecobracmd "github.com/strongo/cli-helpers/selfupdate/cobracmd"
)

// --- command shape: name, no "update" alias, flag surface ---

func TestUpgrade_CommandShape(t *testing.T) {
	t.Parallel()

	cmd := Upgrade("1.2.3")
	if !strings.HasPrefix(cmd.Use, "upgrade") {
		t.Errorf("Use = %q, want it to start with upgrade", cmd.Use)
	}
	if cmd.HasAlias("update") {
		t.Error(`upgrade must not alias "update": ingitdb's update command edits records (cli-install#req:update-alias-policy)`)
	}
	for _, name := range []string{"all", "check", "yes", "dry-run", "format"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("missing --%s flag", name)
		}
	}
	if cmd.Flags().Lookup("dir") != nil {
		t.Error("unexpected --dir flag; upgrade has no --dir")
	}
	if f := cmd.Flags().Lookup("yes"); f.Shorthand != "y" {
		t.Errorf("--yes shorthand = %q, want y", f.Shorthand)
	}
	if cmd.Short == "" {
		t.Error("Short is empty, want a description")
	}
}

// --- upgradeErrors: reuses selfUpdateErrors.Failure for shared kinds ---

func TestUpgradeErrorsFailure_UsageError(t *testing.T) {
	t.Parallel()

	usage := &cliinstallcobracmd.UsageError{Err: errors.New(`invalid --format "yaml": expected text or json`)}
	got := (upgradeErrors{}).Failure(usage)
	if got == nil {
		t.Fatal("Failure(usage error) = nil, want a non-nil error")
	}
	if !strings.Contains(got.Error(), "--format") {
		t.Errorf("Failure(usage error) = %q, want it to mention --format", got.Error())
	}
	if errors.Is(got, ErrValidationFailed) || errors.Is(got, ErrSelfUpdateAvailable) {
		t.Errorf("Failure(%v) = %v, must not match a sentinel with its own exit code", usage, got)
	}
}

// TestUpgradeErrorsFailure_NewKindsMapExplicitly proves the three
// cli-install-only kinds map through upgrade's own explicit "upgrade:"
// branch (cli-install#req:host-owned-exit-codes).
func TestUpgradeErrorsFailure_NewKindsMapExplicitly(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
	}{
		{"unknown target", &selfupdate.Failure{Kind: selfupdate.KindUnknownTarget, Err: errors.New("nosuchcli: not a known install target")}},
		{"no install dir", &selfupdate.Failure{Kind: selfupdate.KindNoInstallDir, Err: errors.New("no per-user bin directory on PATH")}},
		{"destination exists", &selfupdate.Failure{Kind: selfupdate.KindDestinationExists, Err: errors.New("destination already exists")}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := (upgradeErrors{}).Failure(c.err)
			if got == nil {
				t.Fatal("Failure(...) = nil, want a non-nil error")
			}
			if !strings.HasPrefix(got.Error(), "upgrade: ") {
				t.Errorf("Failure(%v) = %q, want an \"upgrade: \" prefix", c.err, got.Error())
			}
			if errors.Is(got, ErrValidationFailed) || errors.Is(got, ErrSelfUpdateAvailable) {
				t.Errorf("Failure(%v) = %v, must not match a sentinel with its own exit code", c.err, got)
			}
		})
	}
}

// TestUpgradeErrorsFailure_SharedKindsUseUpgradePrefix proves every kind
// self-update also handles gets the SAME "upgrade:" prefix as the three
// cli-install-only kinds, never self-update's own bare (unprefixed)
// passthrough: a *selfupdate.Failure/*cliinstall.BatchFailure carries no
// target identity, so Failure has no reliable way to tell "this failure
// was ingitdb's own" from "some other upgraded target's" — the "upgrade:"
// prefix is the one choice that is never wrong about which command
// produced the message (task-22 review M2/M6 fleet ruling). The exit code
// stays the same generic 1 self-update's own passthrough also produces
// (cli-install#req:self-update-equals-upgrade-self).
func TestUpgradeErrorsFailure_SharedKindsUseUpgradePrefix(t *testing.T) {
	t.Parallel()

	cases := []error{
		&selfupdate.Failure{Kind: selfupdate.KindAmbiguous, Err: errors.New("ambiguous")},
		&selfupdate.Failure{Kind: selfupdate.KindReleaseLookup, Err: errors.New("lookup failed")},
		&selfupdate.Failure{Kind: selfupdate.KindChecksum, Err: errors.New("checksum mismatch")},
		&selfupdate.Failure{Kind: selfupdate.KindPermission, Err: errors.New("permission denied")},
		&selfupdate.Failure{Kind: selfupdate.KindManagedCommand, Err: errors.New("brew failed")},
		errors.New("plain error"),
	}
	for _, err := range cases {
		got := (upgradeErrors{}).Failure(err)
		if !strings.HasPrefix(got.Error(), "upgrade: ") {
			t.Errorf("Failure(%v) = %q, want an \"upgrade: \" prefix", err, got.Error())
		}
		if errors.Is(got, ErrValidationFailed) || errors.Is(got, ErrSelfUpdateAvailable) {
			t.Errorf("Failure(%v) = %v, must not match a sentinel with its own exit code", err, got)
		}
	}
}

// UpgradesAvailable must always return ErrSelfUpdateAvailable, which
// main.go's exitCodeForError maps to SelfUpdateAvailableExitCode (10),
// quietly (plan task-14: "map upgrade --check's upgrades-available signal
// to 10 as well").
func TestUpgradeErrorsUpgradesAvailable_ReturnsSentinel(t *testing.T) {
	t.Parallel()

	cases := [][]cliinstall.UpgradeResult{
		nil,
		{{Target: "ingitdb", Verdict: selfupdate.UpdateAvailable, Current: "1.0.0", Latest: "1.1.0"}},
		{
			{Target: "datatug", Verdict: selfupdate.UpdateAvailable},
			{Target: "ovdb", Verdict: selfupdate.Undetermined},
		},
	}
	for _, res := range cases {
		err := (upgradeErrors{}).UpgradesAvailable(res)
		if !errors.Is(err, ErrSelfUpdateAvailable) {
			t.Errorf("UpgradesAvailable(%+v) = %v, want ErrSelfUpdateAvailable", res, err)
		}
	}
}

// --- self-update ≡ upgrade ingitdb: same Config by construction ---

func TestIngitdbSelfUpdateConfig_SameForBothCommands(t *testing.T) {
	t.Parallel()

	a := ingitdbSelfUpdateConfig("1.2.3")
	b := ingitdbSelfUpdateConfig("1.2.3")
	if a.Repository != b.Repository || a.CurrentVersion != b.CurrentVersion || a.BinaryName != b.BinaryName {
		t.Errorf("ingitdbSelfUpdateConfig(\"1.2.3\") is not stable/shared: %+v != %+v", a, b)
	}
}

// --- end-to-end exit-code contract, fully offline ---

// `upgrade nosuchcli` never reaches a status probe or a release lookup:
// cliinstall.CheckUpgrades/PlanUpgrade validate every name against the
// catalog before probing anything, mirroring install's own unknown-target
// path, so this is inherently offline (REQ: no-network-in-tests).
func TestUpgradeCmdNoSuchTarget_ExitCodeContract(t *testing.T) {
	t.Parallel()

	cmd := Upgrade("1.2.3")
	cmd.SetOut(&strings.Builder{})
	cmd.SetErr(&strings.Builder{})
	cmd.SetArgs([]string{"nosuchcli"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected a non-nil error for an unknown upgrade target")
	}
	if !strings.Contains(err.Error(), "nosuchcli") {
		t.Errorf("error %q does not name the unknown target", err.Error())
	}
	if errors.Is(err, ErrValidationFailed) || errors.Is(err, ErrSelfUpdateAvailable) {
		t.Errorf("error %v must not match a sentinel with its own exit code (want the generic exit 1)", err)
	}
}

func TestUpgradeCmdInvalidFormat_IsUsageError(t *testing.T) {
	t.Parallel()

	cmd := Upgrade("1.2.3")
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
	if errors.Is(err, ErrValidationFailed) || errors.Is(err, ErrSelfUpdateAvailable) {
		t.Errorf("error %v must not match a sentinel with its own exit code (want the generic exit 1)", err)
	}
}

// --- self-update-equals-upgrade-self, against a fake releases server ---

// fakeUpgradeEnv is a fully hermetic cliinstall.InstallEnv: no real PATH
// scan, no real filesystem, no real process execution
// (REQ: no-network-in-tests). The host row's classification/version come
// from HostConfig, never from this Env, but resolveUpgradeCandidates still
// calls Probe over every named entry for diagnostics, so every field must
// be set to avoid a nil-func panic.
func fakeUpgradeEnv() cliinstall.InstallEnv {
	return cliinstall.InstallEnv{
		Env: cliinstall.Env{
			PathDirs:     func() []string { return nil },
			HostDir:      func() (string, error) { return "", errors.New("no host dir in test") },
			IsExecutable: func(string) bool { return false },
			EvalSymlinks: func(p string) (string, error) { return p, nil },
			Run: func(context.Context, string, []string) ([]byte, error) {
				return nil, errors.New("process execution disabled in test")
			},
		},
		UserHomeDir: func() (string, error) { return "", errors.New("disabled in test") },
		Getenv:      func(string) string { return "" },
		MkdirAll:    func(string, fs.FileMode) error { return errors.New("disabled in test") },
	}
}

// TestSelfUpdateEqualsUpgradeSelf_CheckContract proves `ingitdb self-update
// --check` and `ingitdb upgrade ingitdb --check` reach the same exit-code
// contract (0 up to date, 10 update available/undetermined via
// ErrSelfUpdateAvailable, distinct-nonzero for a lookup failure) for the
// same fake releases server, built from the exact same Config
// (selfUpdateConfigForTest, self_update_test.go's own seam)
// (cli-install#req:self-update-equals-upgrade-self,
// cli-install#req:upgrade-check).
func TestSelfUpdateEqualsUpgradeSelf_CheckContract(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		ver           string
		body          string
		wantAvailable bool
	}{
		{name: "up to date", ver: "1.0.0", body: `[{"tag_name":"v1.0.0","prerelease":false,"draft":false}]`, wantAvailable: false},
		{name: "update available", ver: "1.0.0", body: `[{"tag_name":"v1.1.0","prerelease":false,"draft":false}]`, wantAvailable: true},
		{name: "undetermined dev build", ver: "dev", body: `[{"tag_name":"v1.1.0","prerelease":false,"draft":false}]`, wantAvailable: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			srv := releasesServer(t, c.body, http.StatusOK)
			cfg := selfUpdateConfigForTest(t, c.ver, srv.URL, srv.Client())

			selfCmd := selfupdatecobracmd.New(cfg, selfupdatecobracmd.CommandOptions{JSONFormat: true, Errors: selfUpdateErrors{}})
			selfCmd.SetOut(&strings.Builder{})
			selfCmd.SetErr(&strings.Builder{})
			selfCmd.SetArgs([]string{"--check"})
			selfErr := selfCmd.Execute()

			upCmd := cliinstallcobracmd.NewUpgrade(cliinstallcobracmd.UpgradeCommandOptions{
				HostID:     "ingitdb",
				Errors:     upgradeErrors{},
				HostConfig: cfg,
				Env:        fakeUpgradeEnv(),
			})
			upCmd.SetOut(&strings.Builder{})
			upCmd.SetErr(&strings.Builder{})
			upCmd.SetArgs([]string{"ingitdb", "--check"})
			upErr := upCmd.Execute()

			if c.wantAvailable {
				if !errors.Is(selfErr, ErrSelfUpdateAvailable) || !errors.Is(upErr, ErrSelfUpdateAvailable) {
					t.Fatalf("want both ErrSelfUpdateAvailable: self-update=%v upgrade=%v", selfErr, upErr)
				}
				return
			}
			if selfErr != nil || upErr != nil {
				t.Fatalf("want both nil (up to date): self-update=%v upgrade=%v", selfErr, upErr)
			}
		})
	}

	t.Run("release lookup failure fails both the same way", func(t *testing.T) {
		t.Parallel()
		srv := releasesServer(t, `not json`, http.StatusInternalServerError)
		cfg := selfUpdateConfigForTest(t, "1.0.0", srv.URL, srv.Client())

		selfCmd := selfupdatecobracmd.New(cfg, selfupdatecobracmd.CommandOptions{JSONFormat: true, Errors: selfUpdateErrors{}})
		selfCmd.SetOut(&strings.Builder{})
		selfCmd.SetErr(&strings.Builder{})
		selfCmd.SetArgs([]string{"--check"})
		selfErr := selfCmd.Execute()
		if selfErr == nil || errors.Is(selfErr, ErrSelfUpdateAvailable) {
			t.Fatalf("expected self-update --check to fail distinctly on a release-lookup error, got %v", selfErr)
		}

		upCmd := cliinstallcobracmd.NewUpgrade(cliinstallcobracmd.UpgradeCommandOptions{
			HostID:     "ingitdb",
			Errors:     upgradeErrors{},
			HostConfig: cfg,
			Env:        fakeUpgradeEnv(),
		})
		upCmd.SetOut(&strings.Builder{})
		upCmd.SetErr(&strings.Builder{})
		upCmd.SetArgs([]string{"ingitdb", "--check"})
		upErr := upCmd.Execute()
		if upErr == nil || errors.Is(upErr, ErrSelfUpdateAvailable) {
			t.Fatalf("expected upgrade ingitdb --check to fail the same distinct way, got %v", upErr)
		}
	})
}

package commands

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/strongo/cli-helpers/cliinstall"
	"github.com/strongo/cli-helpers/selfupdate"
	"github.com/strongo/cli-helpers/selfupdate/cobracmd"
)

// --- command shape: name, no "update" alias, flag surface ---

// AC: cli/self-update#ac:canonical-name — the canonical command name stays
// "self-update" with deliberately NO "update" alias: `ingitdb update` is the
// SQL UPDATE verb command. The migration to cli-helpers/selfupdate/cobracmd
// also adds --dry-run and --format (JSONFormat: true), per the rewritten
// spec/features/cli/self-update/README.md.
func TestSelfUpdate_CommandShapeUnchangedPlusNewFlags(t *testing.T) {
	t.Parallel()

	cmd := SelfUpdate("1.2.3")
	if cmd.Name() != "self-update" {
		t.Errorf("Name() = %q, want %q", cmd.Name(), "self-update")
	}
	if cmd.HasAlias("update") {
		t.Error("self-update must not alias \"update\": it would collide with the SQL update verb command")
	}

	for _, name := range []string{"check", "yes", "version", "allow-downgrade", "dry-run", "format"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("missing --%s flag", name)
		}
	}
	if f := cmd.Flags().Lookup("yes"); f.Shorthand != "y" {
		t.Errorf("--yes shorthand = %q, want y", f.Shorthand)
	}
	if f := cmd.Flags().Lookup("format"); f.DefValue != "text" {
		t.Errorf("--format default = %q, want text", f.DefValue)
	}
}

// The catalog lookup failing is a programming error the package's own tests
// must catch (cli-install#req:host-identity-from-catalog), never a runtime
// state; TestSelfUpdate_CommandShapeUnchangedPlusNewFlags above proves the
// real "ingitdb" entry resolves. This test seam-swaps catalogByID to reach
// the defensive panic. Must not run in parallel: it mutates a package var.
func TestSelfUpdate_PanicsWhenCatalogEntryMissing(t *testing.T) {
	prev := catalogByID
	catalogByID = func(string) (cliinstall.Entry, bool) { return cliinstall.Entry{}, false }
	t.Cleanup(func() { catalogByID = prev })

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected SelfUpdate to panic when its catalog entry is missing")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "ingitdb") {
			t.Errorf("panic value = %v, want a message naming \"ingitdb\"", r)
		}
	}()
	SelfUpdate("1.2.3")
}

// --- selfUpdateErrors: the exit-code-preserving ErrorMapper ---

// selfUpdateErrors.Failure must never translate or wrap: every self-update
// failure kind (ambiguous detection, release lookup, download, checksum,
// permission, non-interactive refusal, a managed-command failure, or an
// invalid --format usage error) keeps falling through to the CLI's generic
// error exit 1 via main.go's default exitCodeForError branch, exactly as
// the old internal self-update package's failures did before this migration.
func TestSelfUpdateErrors_Failure_PassesThroughUnchanged(t *testing.T) {
	t.Parallel()

	cases := []error{
		&selfupdate.Failure{Kind: selfupdate.KindAmbiguous, Err: errors.New("ambiguous")},
		&selfupdate.Failure{Kind: selfupdate.KindReleaseLookup, Err: errors.New("lookup failed")},
		&selfupdate.Failure{Kind: selfupdate.KindChecksum, Err: errors.New("checksum mismatch")},
		&selfupdate.Failure{Kind: selfupdate.KindPermission, Path: "/usr/local/bin/ingitdb", Err: errors.New("permission denied")},
		&selfupdate.Failure{Kind: selfupdate.KindNonInteractive, Err: errors.New("no tty")},
		&selfupdate.Failure{Kind: selfupdate.KindManagedCommand, Err: errors.New("brew failed")},
		&cobracmd.UsageError{Err: errors.New("invalid --format")},
		errors.New("plain error"),
	}
	for _, err := range cases {
		got := (selfUpdateErrors{}).Failure(err)
		if got != err {
			t.Errorf("Failure(%v) = %v, want the same error returned unchanged", err, got)
		}
	}
}

// selfUpdateErrors.UpdateAvailable always returns ErrSelfUpdateAvailable,
// which main.go's exitCodeForError maps to SelfUpdateAvailableExitCode (10)
// — preserving the exit-10-on-available-update contract regardless of the
// CheckResult's own content.
func TestSelfUpdateErrors_UpdateAvailable_ReturnsSentinel(t *testing.T) {
	t.Parallel()

	cases := []selfupdate.CheckResult{
		{Current: "1.0.0", Latest: "1.1.0", Verdict: selfupdate.UpdateAvailable},
		{Current: "dev", Latest: "1.1.0", Verdict: selfupdate.Undetermined},
		{},
	}
	for _, res := range cases {
		err := (selfUpdateErrors{}).UpdateAvailable(res)
		if !errors.Is(err, ErrSelfUpdateAvailable) {
			t.Errorf("UpdateAvailable(%+v) = %v, want ErrSelfUpdateAvailable", res, err)
		}
	}
}

// --- catalog identity: the flat checksums.txt fix and redirect-only managers ---

// The known bug this migration fixes: the old internal self-update package
// fetched "checksums-darwin.txt" and "checksums-windows.txt", which 404 because
// ingitdb's release publishes one flat "checksums.txt" for every platform
// (.goreleaser.yaml `checksum.name_template: checksums.txt`). The catalog
// entry's ChecksumsName override must always resolve to that flat name.
func TestIngitdbCatalogEntry_FlatChecksumsName(t *testing.T) {
	t.Parallel()

	entry, ok := cliinstall.ByID("ingitdb")
	if !ok {
		t.Fatal("no catalog entry for \"ingitdb\"")
	}
	if entry.ChecksumsName == nil {
		t.Fatal("entry.ChecksumsName is nil; want the flat checksums.txt override")
	}
	for _, goos := range []string{"linux", "darwin", "windows"} {
		if got := entry.ChecksumsName("ingitdb", "1.2.3"); got != "checksums.txt" {
			t.Errorf("ChecksumsName(...) for %s = %q, want %q", goos, got, "checksums.txt")
		}
	}
}

// task-14 fixes a second bug: ingitdb's old internal Classify function
// only recognized Homebrew and Snap, so a Scoop- or WinGet-managed install
// (both published per .goreleaser.yaml) classified Manual and was eligible
// for self-replace — overwriting a binary a package manager owns. Every
// manager on the catalog entry must be present and redirect-only (print the
// upgrade command, never execute it), matching ingitdb's pre-existing
// print-and-exit self-update behavior for every managed channel.
func TestIngitdbCatalogEntry_ManagersAreAllRedirectOnly(t *testing.T) {
	t.Parallel()

	entry, ok := cliinstall.ByID("ingitdb")
	if !ok {
		t.Fatal("no catalog entry for \"ingitdb\"")
	}
	wantNames := map[string]bool{"Homebrew": false, "Snap": false, "Scoop": false, "WinGet": false}
	for _, m := range entry.Managers {
		if _, known := wantNames[m.Name]; !known {
			t.Errorf("unexpected manager %q on the ingitdb catalog entry", m.Name)
			continue
		}
		wantNames[m.Name] = true
		if m.CanExecuteUpgrade() {
			t.Errorf("manager %q is executable; ingitdb's catalog entry must keep every manager redirect-only", m.Name)
		}
		if m.UpgradeCommand == "" {
			t.Errorf("manager %q has no upgrade command to print", m.Name)
		}
	}
	for name, found := range wantNames {
		if !found {
			t.Errorf("catalog entry is missing manager %q", name)
		}
	}
}

// --- end-to-end exit-code contract, against a fake GitHub releases server ---

// selfUpdateConfigForTest builds exactly the selfupdate.Config SelfUpdate
// itself builds (same catalog entry, same running version), with the
// release endpoint redirected to a local httptest.Server so no test makes a
// real network request (REQ: no-network-in-tests).
func selfUpdateConfigForTest(t *testing.T, ver, apiURL string, client *http.Client) selfupdate.Config {
	t.Helper()
	entry, ok := cliinstall.ByID("ingitdb")
	if !ok {
		t.Fatal("no catalog entry for \"ingitdb\"")
	}
	cfg := entry.Config(ver)
	cfg.ReleasesAPIURL = apiURL
	cfg.HTTPClient = client
	return cfg
}

func releasesServer(t *testing.T, body string, status int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// AC: cli/self-update#ac:check-exit-code-contract — --check exit codes must
// stay 0 (up to date), 10 (update available or undetermined, via
// ErrSelfUpdateAvailable), and a code distinct from both (the CLI's generic
// error exit) for a release-lookup failure. This exercises the real
// cobracmd.New wiring built from the same Config and selfUpdateErrors{}
// mapper SelfUpdate uses, proving the migration preserves the contract.
func TestSelfUpdate_CheckExitCodeContract_EndToEnd(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		ver           string
		body          string
		status        int
		wantAvailable bool
	}{
		{
			name:          "up to date",
			ver:           "1.0.0",
			body:          `[{"tag_name":"v1.0.0","prerelease":false,"draft":false}]`,
			status:        http.StatusOK,
			wantAvailable: false,
		},
		{
			name:          "update available",
			ver:           "1.0.0",
			body:          `[{"tag_name":"v1.1.0","prerelease":false,"draft":false}]`,
			status:        http.StatusOK,
			wantAvailable: true,
		},
		{
			name:          "undetermined dev build",
			ver:           "dev",
			body:          `[{"tag_name":"v1.1.0","prerelease":false,"draft":false}]`,
			status:        http.StatusOK,
			wantAvailable: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			srv := releasesServer(t, c.body, c.status)
			cfg := selfUpdateConfigForTest(t, c.ver, srv.URL, srv.Client())
			cmd := cobracmd.New(cfg, cobracmd.CommandOptions{JSONFormat: true, Errors: selfUpdateErrors{}})
			var out strings.Builder
			cmd.SetOut(&out)
			cmd.SetErr(&strings.Builder{})
			cmd.SetArgs([]string{"--check"})

			err := cmd.Execute()
			if c.wantAvailable {
				if !errors.Is(err, ErrSelfUpdateAvailable) {
					t.Fatalf("Execute() error = %v, want ErrSelfUpdateAvailable", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Execute() error = %v, want nil (up to date)", err)
			}
		})
	}

	t.Run("release lookup failure is a distinct non-zero code", func(t *testing.T) {
		t.Parallel()
		srv := releasesServer(t, `not json`, http.StatusInternalServerError)
		cfg := selfUpdateConfigForTest(t, "1.0.0", srv.URL, srv.Client())
		cmd := cobracmd.New(cfg, cobracmd.CommandOptions{JSONFormat: true, Errors: selfUpdateErrors{}})
		cmd.SetOut(&strings.Builder{})
		cmd.SetErr(&strings.Builder{})
		cmd.SetArgs([]string{"--check"})

		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected a non-nil error for a release-lookup failure")
		}
		if errors.Is(err, ErrSelfUpdateAvailable) {
			t.Fatalf("Execute() error = %v, must NOT be ErrSelfUpdateAvailable (would collide with exit 10)", err)
		}
	})
}

// --format json is new (JSONFormat: true); stdout must stay exactly one
// JSON document, matching REQ: machine-readable-output.
func TestSelfUpdate_CheckJSONFormat(t *testing.T) {
	t.Parallel()

	srv := releasesServer(t, `[{"tag_name":"v1.0.0","prerelease":false,"draft":false}]`, http.StatusOK)
	cfg := selfUpdateConfigForTest(t, "1.0.0", srv.URL, srv.Client())
	cmd := cobracmd.New(cfg, cobracmd.CommandOptions{JSONFormat: true, Errors: selfUpdateErrors{}})
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&strings.Builder{})
	cmd.SetArgs([]string{"--check", "--format", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	var got struct {
		Current, Latest, Verdict string
	}
	if err := json.Unmarshal([]byte(out.String()), &got); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\noutput: %s", err, out.String())
	}
	if got.Verdict != "up_to_date" {
		t.Errorf("verdict = %q, want up_to_date", got.Verdict)
	}
}

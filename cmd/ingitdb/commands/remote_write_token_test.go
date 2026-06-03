package commands

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// parseRemoteFlags builds a command with the shared remote flags and parses args.
func parseRemoteFlags(t *testing.T, args ...string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "x"}
	addRemoteFlags(cmd)
	if err := cmd.ParseFlags(args); err != nil {
		t.Fatalf("ParseFlags(%v): %v", args, err)
	}
	return cmd
}

func TestRequireRemoteWriteToken(t *testing.T) {
	// Ensure no ambient token leaks in from the environment (e.g. CI).
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GITHUB_COM_TOKEN", "")

	t.Run("no remote is a no-op", func(t *testing.T) {
		cmd := parseRemoteFlags(t)
		if err := requireRemoteWriteToken(cmd); err != nil {
			t.Errorf("local write must not require a token, got: %v", err)
		}
	})

	t.Run("remote without token errors", func(t *testing.T) {
		cmd := parseRemoteFlags(t, "--remote=github.com/owner/repo")
		err := requireRemoteWriteToken(cmd)
		if err == nil || !strings.Contains(err.Error(), "token") {
			t.Errorf("expected a token-required error, got: %v", err)
		}
	})

	t.Run("remote with --token is allowed", func(t *testing.T) {
		cmd := parseRemoteFlags(t, "--remote=github.com/owner/repo", "--token=abc")
		if err := requireRemoteWriteToken(cmd); err != nil {
			t.Errorf("a supplied --token must satisfy the requirement, got: %v", err)
		}
	})

	t.Run("remote with host env token is allowed", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", "from-env")
		cmd := parseRemoteFlags(t, "--remote=github.com/owner/repo")
		if err := requireRemoteWriteToken(cmd); err != nil {
			t.Errorf("a host-derived env token must satisfy the requirement, got: %v", err)
		}
	})
}

// TestDropRemote_RequiresToken is the command-level check: a remote write
// (drop) with no token fails fast with a clear token error, before any I/O.
func TestDropRemote_RequiresToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GITHUB_COM_TOKEN", "")

	homeDir, getWd, readDef, newDB, logf := emptyDropDeps(t)
	cmd := Drop(homeDir, getWd, readDef, newDB, logf)
	cmd.SetArgs([]string{"collection", "cities", "--remote=github.com/owner/repo"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "token") {
		t.Fatalf("expected a token-required error for a remote drop with no token, got: %v", err)
	}
}

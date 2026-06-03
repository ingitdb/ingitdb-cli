package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMain installs a hermetic global git config for every test in this package
// so test repos never spawn a detached auto-gc / background-maintenance process.
// Such a process can keep writing to a temp repo's .git/objects after a test
// returns, racing with t.TempDir()'s RemoveAll and failing cleanup with
// "directory not empty" (observed flaking the rebase tests on CI).
//
// The temp config INCLUDES the pre-existing global config (so init.defaultBranch,
// identity, safe.directory, etc. are preserved) and only adds the gc/maintenance
// overrides, which — appearing after the include — take precedence. It is
// process-local via GIT_CONFIG_GLOBAL and removed on exit. The rebase tests also
// set these knobs per-repo, so the hotspot stays covered even if a test
// overrides GIT_CONFIG_GLOBAL or HOME for its own purposes.
func TestMain(m *testing.M) {
	restore, err := installHermeticGitConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: hermetic git config setup failed: %v\n", err)
		os.Exit(m.Run())
	}
	code := m.Run()
	restore()
	os.Exit(code)
}

// installHermeticGitConfig writes a temp global git config disabling auto-gc and
// background maintenance, points GIT_CONFIG_GLOBAL at it, and returns a function
// that restores the previous environment and removes the temp file.
func installHermeticGitConfig() (restore func(), err error) {
	dir, err := os.MkdirTemp("", "ingitdb-gitconfig-")
	if err != nil {
		return nil, err
	}

	// Preserve the pre-existing global config via include so we only ADD the gc
	// overrides rather than replacing the user's/CI's global settings.
	var prior string
	if v := os.Getenv("GIT_CONFIG_GLOBAL"); v != "" {
		prior = v
	} else if home, herr := os.UserHomeDir(); herr == nil {
		prior = filepath.Join(home, ".gitconfig")
	}

	var b strings.Builder
	if prior != "" {
		// A missing include path is silently ignored by git, so this is safe even
		// when no global config exists.
		fmt.Fprintf(&b, "[include]\n\tpath = %s\n", prior)
	}
	b.WriteString("[gc]\n\tauto = 0\n[maintenance]\n\tauto = false\n")

	cfgPath := filepath.Join(dir, "gitconfig")
	if writeErr := os.WriteFile(cfgPath, []byte(b.String()), 0o644); writeErr != nil {
		_ = os.RemoveAll(dir)
		return nil, writeErr
	}

	prev, had := os.LookupEnv("GIT_CONFIG_GLOBAL")
	_ = os.Setenv("GIT_CONFIG_GLOBAL", cfgPath)
	return func() {
		if had {
			_ = os.Setenv("GIT_CONFIG_GLOBAL", prev)
		} else {
			_ = os.Unsetenv("GIT_CONFIG_GLOBAL")
		}
		_ = os.RemoveAll(dir)
	}, nil
}

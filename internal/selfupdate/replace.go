package selfupdate

// specscore: feature/cli/self-update

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Test seams: overridable indirections so the Windows rename path and the
// staging error branches are exercisable on any host. Tests that replace
// these MUST NOT run in parallel.
var (
	goosName       = runtime.GOOS
	renameFunc     = os.Rename
	stageCreateTmp = func(dir, pattern string) (tempFile, error) {
		return os.CreateTemp(dir, pattern)
	}
)

// ReplaceExecutable atomically swaps the binary at newBinaryPath into
// targetPath. The new binary is first staged to a temp file in the same
// directory as targetPath (same filesystem, so os.Rename is atomic), with
// executable permissions (0755), then renamed over the target.
//
// On POSIX, os.Rename within one directory atomically replaces the target —
// the install location is never left with a partial or truncated file: it
// holds either the original or the complete new binary. On Windows, a running
// .exe cannot be overwritten, so the existing target is first renamed aside to
// targetPath+".old" before the new binary is moved into place.
//
// Any staging error before the rename returns the error and leaves targetPath
// untouched. Underlying os errors are wrapped with %w so callers can classify
// them (e.g. errors.Is(err, fs.ErrPermission)).
func ReplaceExecutable(targetPath, newBinaryPath string) error {
	dir := filepath.Dir(targetPath)

	// Stage the new binary into a temp file in the target's directory so the
	// subsequent rename stays on the same filesystem and is atomic.
	staged, err := stage(dir, newBinaryPath)
	if err != nil {
		return err
	}

	if goosName == "windows" {
		// A running .exe cannot be overwritten, but it can be renamed. Move the
		// current target aside, then move the new binary into place. The .old
		// copy may stay locked while the old process runs; removing it is
		// best-effort.
		old := targetPath + ".old"
		_ = os.Remove(old)
		if err := renameFunc(targetPath, old); err != nil {
			_ = os.Remove(staged)
			return fmt.Errorf("move aside current executable: %w", err)
		}
		if err := renameFunc(staged, targetPath); err != nil {
			// Best effort: restore the original so the install stays runnable.
			_ = renameFunc(old, targetPath)
			_ = os.Remove(staged)
			return fmt.Errorf("install new executable: %w", err)
		}
		_ = os.Remove(old)
		return nil
	}

	// POSIX: a single rename atomically replaces the target.
	if err := renameFunc(staged, targetPath); err != nil {
		_ = os.Remove(staged)
		return fmt.Errorf("install new executable: %w", err)
	}
	return nil
}

// stage copies src into a new temp file in dir with 0755 permissions and
// returns its path. On any error the temp file is removed so nothing is left
// behind.
func stage(dir, src string) (string, error) {
	in, err := os.Open(src)
	if err != nil {
		return "", fmt.Errorf("open new binary: %w", err)
	}
	defer func() {
		_ = in.Close()
	}()

	tmp, err := stageCreateTmp(dir, ".ingitdb-stage-*")
	if err != nil {
		return "", fmt.Errorf("create staging file: %w", err)
	}
	if _, err := io.Copy(tmp, in); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return "", fmt.Errorf("stage new binary: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return "", fmt.Errorf("close staging file: %w", err)
	}
	if err := os.Chmod(tmp.Name(), 0o755); err != nil {
		_ = os.Remove(tmp.Name())
		return "", fmt.Errorf("chmod staging file: %w", err)
	}
	return tmp.Name(), nil
}

// VerifyBinaryVersion runs "<path> version" (the ingitdb version subcommand;
// the root command has no --version flag) and returns an error unless the
// command output contains wantVersion. It is kept separate from
// ReplaceExecutable so the swap and the post-swap sanity check are
// independently testable.
func VerifyBinaryVersion(path, wantVersion string) error {
	out, err := exec.Command(path, "version").CombinedOutput()
	if err != nil {
		return fmt.Errorf("run %s version: %w", path, err)
	}
	if !strings.Contains(string(out), wantVersion) {
		got := strings.TrimSpace(string(out))
		return fmt.Errorf(
			"version check failed: %s version did not report %q (got %q)",
			path, wantVersion, got)
	}
	return nil
}

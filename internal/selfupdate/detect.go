package selfupdate

// specscore: feature/cli/self-update

import (
	"os"
	"path/filepath"
	"strings"
)

// Manager identifies a package manager that owns a managed install.
type Manager int

const (
	// ManagerNone means no package manager was detected.
	ManagerNone Manager = iota
	// Homebrew is the macOS/Linux Homebrew package manager. ingitdb is
	// distributed as a Homebrew cask, which installs the binary under a
	// Caskroom path and symlinks it into the brew prefix bin directory.
	Homebrew
	// Snap is the Linux snap package manager.
	Snap
)

// InstallMethod classifies how the running binary was installed.
type InstallMethod int

const (
	// Managed means a package manager owns the binary; self-update is not eligible.
	Managed InstallMethod = iota
	// Manual means the binary was placed by the user (release archive or go install);
	// self-replace is eligible.
	Manual
	// Ambiguous means the location does not clearly indicate either method.
	Ambiguous
)

// Detection is the result of classifying an executable path.
type Detection struct {
	Method  InstallMethod
	Manager Manager
}

// Classify decides the install method purely from execPath. It is case-insensitive
// and treats both '/' and '\' as path separators so Windows paths can be tested on
// any host.
func Classify(execPath string) Detection {
	// Normalize: lowercase and unify separators to '/'.
	p := strings.ToLower(execPath)
	p = strings.ReplaceAll(p, `\`, "/")

	// Managed: Homebrew (formula Cellar, cask Caskroom, or any brew prefix).
	if strings.Contains(p, "/cellar/") ||
		strings.Contains(p, "/caskroom/") ||
		strings.Contains(p, "/homebrew/") ||
		strings.Contains(p, "/linuxbrew/") {
		return Detection{Method: Managed, Manager: Homebrew}
	}

	// Managed: Snap (mounted under /snap, data under /var/snap).
	if strings.Contains(p, "/snap/") {
		return Detection{Method: Managed, Manager: Snap}
	}

	// Manual: go install targets.
	if strings.Contains(p, "/go/bin/") {
		return Detection{Method: Manual, Manager: ManagerNone}
	}

	// Manual: release-archive / home bin locations (binary directly under a bin/).
	dir := p
	if i := strings.LastIndex(p, "/"); i >= 0 {
		dir = p[:i]
	}
	if strings.HasSuffix(dir, "/bin") {
		return Detection{Method: Manual, Manager: ManagerNone}
	}

	return Detection{Method: Ambiguous, Manager: ManagerNone}
}

// Test seams: overridable indirections for os/filepath calls so DetectSelf's
// error and symlink-fallback branches are exercisable without real symlinks.
// Tests that replace these MUST NOT run in parallel.
var (
	osExecutable     = os.Executable
	evalSymlinksFunc = filepath.EvalSymlinks
)

// DetectSelf resolves the running executable's path and classifies BOTH the
// raw (possibly symlink) path and its resolved target. The Homebrew cask
// installs a symlink in the brew prefix bin directory pointing into the
// Caskroom, so a managed classification on either path wins. When neither is
// managed, a manual classification on either path is accepted; otherwise the
// result is Ambiguous.
func DetectSelf() (Detection, error) {
	exe, err := osExecutable()
	if err != nil {
		return Detection{}, err
	}
	linkDet := Classify(exe)
	resolved, err := evalSymlinksFunc(exe)
	if err != nil {
		resolved = exe
	}
	resolvedDet := Classify(resolved)

	if linkDet.Method == Managed {
		return linkDet, nil
	}
	if resolvedDet.Method == Managed {
		return resolvedDet, nil
	}
	if linkDet.Method == Manual {
		return linkDet, nil
	}
	return resolvedDet, nil
}

package selfupdate

import (
	"errors"
	"testing"
)

func TestClassify(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		execPath    string
		wantMethod  InstallMethod
		wantManager Manager
	}{
		{
			name:        "manual is eligible: release archive in /usr/local/bin",
			execPath:    "/usr/local/bin/ingitdb",
			wantMethod:  Manual,
			wantManager: ManagerNone,
		},
		{
			name:        "manual is eligible: go install target under go/bin",
			execPath:    "/home/u/go/bin/ingitdb",
			wantMethod:  Manual,
			wantManager: ManagerNone,
		},
		{
			name:        "manual: home bin",
			execPath:    "/home/u/bin/ingitdb",
			wantMethod:  Manual,
			wantManager: ManagerNone,
		},
		{
			name:        "homebrew cask caskroom",
			execPath:    "/opt/homebrew/Caskroom/ingitdb/0.40.1/ingitdb",
			wantMethod:  Managed,
			wantManager: Homebrew,
		},
		{
			name:        "homebrew apple silicon prefix symlink",
			execPath:    "/opt/homebrew/bin/ingitdb",
			wantMethod:  Managed,
			wantManager: Homebrew,
		},
		{
			name:        "homebrew intel cellar",
			execPath:    "/usr/local/Cellar/ingitdb/0.40.1/bin/ingitdb",
			wantMethod:  Managed,
			wantManager: Homebrew,
		},
		{
			name:        "linuxbrew prefix",
			execPath:    "/home/linuxbrew/.linuxbrew/bin/ingitdb",
			wantMethod:  Managed,
			wantManager: Homebrew,
		},
		{
			name:        "snap bin",
			execPath:    "/snap/bin/ingitdb",
			wantMethod:  Managed,
			wantManager: Snap,
		},
		{
			name:        "snap mounted revision",
			execPath:    "/snap/ingitdb/42/ingitdb",
			wantMethod:  Managed,
			wantManager: Snap,
		},
		{
			name:        "ambiguous unrecognized path",
			execPath:    "/tmp/random/ingitdb",
			wantMethod:  Ambiguous,
			wantManager: ManagerNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Classify(tt.execPath)
			if got.Method != tt.wantMethod {
				t.Errorf("Classify(%q).Method = %v, want %v", tt.execPath, got.Method, tt.wantMethod)
			}
			if got.Manager != tt.wantManager {
				t.Errorf("Classify(%q).Manager = %v, want %v", tt.execPath, got.Manager, tt.wantManager)
			}
		})
	}
}

func TestClassifyNeverAmbiguousForClearCases(t *testing.T) {
	t.Parallel()
	clear := []string{
		"/usr/local/bin/ingitdb",
		"/home/u/go/bin/ingitdb",
		"/opt/homebrew/Caskroom/ingitdb/0.40.1/ingitdb",
		"/usr/local/Cellar/ingitdb/0.40.1/bin/ingitdb",
		"/snap/bin/ingitdb",
	}
	for _, p := range clear {
		if got := Classify(p); got.Method == Ambiguous {
			t.Errorf("Classify(%q) returned Ambiguous for a clearly-classified path", p)
		}
	}
}

// DetectSelf must classify BOTH the link path and the resolved target: the
// Homebrew cask installs a symlink in /opt/homebrew/bin pointing into the
// Caskroom, so either side classifying as managed must win.
func TestDetectSelf_CaskSymlinkIsManaged(t *testing.T) {
	origExe, origEval := osExecutable, evalSymlinksFunc
	t.Cleanup(func() { osExecutable, evalSymlinksFunc = origExe, origEval })

	// Link path is a manual-looking bin dir; the resolved target is Caskroom.
	osExecutable = func() (string, error) { return "/usr/local/bin/ingitdb", nil }
	evalSymlinksFunc = func(string) (string, error) {
		return "/usr/local/Caskroom/ingitdb/0.40.1/ingitdb", nil
	}

	got, err := DetectSelf()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Method != Managed || got.Manager != Homebrew {
		t.Fatalf("DetectSelf = %+v, want Managed/Homebrew", got)
	}
}

func TestDetectSelf_LinkPathManagedWins(t *testing.T) {
	origExe, origEval := osExecutable, evalSymlinksFunc
	t.Cleanup(func() { osExecutable, evalSymlinksFunc = origExe, origEval })

	osExecutable = func() (string, error) { return "/opt/homebrew/bin/ingitdb", nil }
	evalSymlinksFunc = func(p string) (string, error) { return p, nil }

	got, err := DetectSelf()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Method != Managed || got.Manager != Homebrew {
		t.Fatalf("DetectSelf = %+v, want Managed/Homebrew", got)
	}
}

func TestDetectSelf_ExecutableError(t *testing.T) {
	origExe := osExecutable
	t.Cleanup(func() { osExecutable = origExe })

	osExecutable = func() (string, error) { return "", errors.New("boom") }
	if _, err := DetectSelf(); err == nil {
		t.Fatal("expected error when os.Executable fails, got nil")
	}
}

func TestDetectSelf_SymlinkFallback(t *testing.T) {
	origExe, origEval := osExecutable, evalSymlinksFunc
	t.Cleanup(func() { osExecutable, evalSymlinksFunc = origExe, origEval })

	osExecutable = func() (string, error) { return "/home/u/go/bin/ingitdb", nil }
	evalSymlinksFunc = func(string) (string, error) { return "", errors.New("no symlink") }

	got, err := DetectSelf()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Falls back to the raw exe path, which classifies as Manual (go/bin).
	if got.Method != Manual {
		t.Fatalf("DetectSelf fallback = %+v, want Manual", got)
	}
}

func TestDetectSelf_ManualLinkAmbiguousTarget(t *testing.T) {
	origExe, origEval := osExecutable, evalSymlinksFunc
	t.Cleanup(func() { osExecutable, evalSymlinksFunc = origExe, origEval })

	// A symlink in a bin dir to an unrecognized location: the manual link path
	// classification is accepted (neither side is managed).
	osExecutable = func() (string, error) { return "/usr/local/bin/ingitdb", nil }
	evalSymlinksFunc = func(string) (string, error) { return "/tmp/random/ingitdb", nil }

	got, err := DetectSelf()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Method != Manual {
		t.Fatalf("DetectSelf = %+v, want Manual", got)
	}
}

func TestDetectSelf_AmbiguousBothSides(t *testing.T) {
	origExe, origEval := osExecutable, evalSymlinksFunc
	t.Cleanup(func() { osExecutable, evalSymlinksFunc = origExe, origEval })

	osExecutable = func() (string, error) { return "/tmp/random/ingitdb", nil }
	evalSymlinksFunc = func(p string) (string, error) { return p, nil }

	got, err := DetectSelf()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Method != Ambiguous {
		t.Fatalf("DetectSelf = %+v, want Ambiguous", got)
	}
}

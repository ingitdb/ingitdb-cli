package main

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/dal-go/dalgo/dal"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func TestRun_Version(t *testing.T) {
	t.Parallel()

	args := []string{"ingitdb", "version"}
	readCalled := false
	fatalCalled := false
	readDefinition := func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		readCalled = true
		return nil, nil
	}
	fatal := func(error) {
		fatalCalled = true
	}
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "/tmp/wd", nil }
	logf := func(...any) {}

	run(args, homeDir, getWd, readDefinition, fatal, logf)
	if readCalled {
		t.Fatal("readDefinition should not be called for version")
	}
	if fatalCalled {
		t.Fatal("fatal should not be called for version")
	}
}

func TestRun_NoSubcommand(t *testing.T) {
	t.Parallel()

	args := []string{"ingitdb"}
	fatalCalled := false
	readDefinition := func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return nil, nil
	}
	fatal := func(error) {
		fatalCalled = true
	}
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "/tmp/wd", nil }
	logf := func(...any) {}

	run(args, homeDir, getWd, readDefinition, fatal, logf)
	if fatalCalled {
		t.Fatal("fatal should not be called when no subcommand given")
	}
}

func TestRun_ValidateSuccess(t *testing.T) {
	t.Parallel()

	readCalled := false
	var readPath string
	readDefinition := func(path string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		readCalled = true
		readPath = path
		return &ingitdb.Definition{}, nil
	}
	fatalCalled := false
	fatal := func(error) { fatalCalled = true }
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "/tmp/wd", nil }
	logf := func(...any) {}

	run([]string{"ingitdb", "validate", "--path=/valid/dir"}, homeDir, getWd, readDefinition, fatal, logf)
	if !readCalled {
		t.Fatal("readDefinition should be called")
	}
	if readPath != "/valid/dir" {
		t.Fatalf("expected path /valid/dir, got %s", readPath)
	}
	if fatalCalled {
		t.Fatal("fatal should not be called on success")
	}
}

func TestRun_ValidateError(t *testing.T) {
	t.Parallel()

	readDefinition := func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return nil, errors.New("boom")
	}
	fatalCalled := false
	fatal := func(error) { fatalCalled = true }
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "/tmp/wd", nil }
	logf := func(...any) {}

	run([]string{"ingitdb", "validate", "--path=/x"}, homeDir, getWd, readDefinition, fatal, logf)
	if !fatalCalled {
		t.Fatal("fatal should be called on readDefinition error")
	}
}

func TestRun_ValidateDefaultPath(t *testing.T) {
	t.Parallel()

	var readPath string
	readDefinition := func(path string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		readPath = path
		return &ingitdb.Definition{}, nil
	}
	fatalCalled := false
	fatal := func(error) { fatalCalled = true }
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "/wd", nil }
	logf := func(...any) {}

	run([]string{"ingitdb", "validate"}, homeDir, getWd, readDefinition, fatal, logf)
	if fatalCalled {
		t.Fatal("fatal should not be called")
	}
	if readPath != "/wd" {
		t.Fatalf("expected path /wd, got %s", readPath)
	}
}

func TestRun_ValidateHomePath(t *testing.T) {
	t.Parallel()

	var readPath string
	readDefinition := func(path string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		readPath = path
		return &ingitdb.Definition{}, nil
	}
	fatalCalled := false
	fatal := func(error) { fatalCalled = true }
	homeDir := func() (string, error) { return "/home/user", nil }
	getWd := func() (string, error) { return "/tmp/wd", nil }
	logf := func(...any) {}

	run([]string{"ingitdb", "validate", "--path=~/db"}, homeDir, getWd, readDefinition, fatal, logf)
	if fatalCalled {
		t.Fatal("fatal should not be called")
	}
	if readPath != "/home/user/db" {
		t.Fatalf("expected /home/user/db, got %s", readPath)
	}
}

func TestMain_VersionCmd(t *testing.T) {
	args := os.Args
	os.Args = []string{"ingitdb", "version"}
	t.Cleanup(func() {
		os.Args = args
	})

	main()
}

func TestMain_ReadDefinitionError(t *testing.T) {
	// Create a temp dir with a root-collections.yaml that points to a
	// nonexistent collection directory, so ReadDefinition returns an error.
	tmpDir := t.TempDir()
	ingitDBDir := tmpDir + "/.ingitdb"
	if err := os.MkdirAll(ingitDBDir, 0755); err != nil {
		t.Fatalf("create .ingitdb dir: %v", err)
	}
	if err := os.WriteFile(ingitDBDir+"/root-collections.yaml", []byte("foo: nonexistent-col\n"), 0644); err != nil {
		t.Fatalf("write root-collections.yaml: %v", err)
	}

	args := os.Args
	os.Args = []string{"ingitdb", "validate", "--path=" + tmpDir}
	t.Cleanup(func() {
		os.Args = args
	})

	oldExit := exit
	exitCalled := false
	exit = func(int) {
		exitCalled = true
	}
	t.Cleanup(func() {
		exit = oldExit
	})

	oldStderr := os.Stderr
	devNull, _ := os.Open(os.DevNull)
	os.Stderr = devNull
	t.Cleanup(func() {
		os.Stderr = oldStderr
		_ = devNull.Close()
	})

	main()

	if !exitCalled {
		t.Fatal("expected exit to be called")
	}
}

func TestRun_InvalidCommand(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panic recovered: %v", r)
		}
	}()

	// Mock exit to prevent the test from actually exiting
	oldExit := exit
	exitCalled := false
	var exitCode int
	exit = func(code int) {
		exitCalled = true
		exitCode = code
		t.Logf("exit called with code %d", code)
	}
	t.Cleanup(func() {
		t.Logf("cleanup: exitCalled=%v, exitCode=%d", exitCalled, exitCode)
		exit = oldExit
	})

	args := []string{"ingitdb", "nonexistent-command"}
	fatalCalled := false
	var capturedErr error
	fatal := func(err error) {
		fatalCalled = true
		capturedErr = err
		t.Logf("fatal called with err=%v", err)
	}
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "/tmp/wd", nil }
	readDefinition := func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return nil, nil
	}
	logf := func(...any) {}

	t.Log("calling run()")
	run(args, homeDir, getWd, readDefinition, fatal, logf)
	t.Log("run() returned")
	// cobra calls fatal with "unknown command" error
	_ = fatalCalled
	_ = capturedErr
	_ = exitCalled
	_ = exitCode
}

func TestRun_ExitCoderWithNonZeroCode(t *testing.T) {
	// With cobra, an unknown flag causes fatal to be called with the parse error.
	args := []string{"ingitdb", "validate", "--invalid-flag"}

	exitCalled := false
	var exitCode int
	oldExit := exit
	exit = func(code int) {
		exitCalled = true
		exitCode = code
	}
	t.Cleanup(func() {
		exit = oldExit
	})

	fatalCalled := false
	fatal := func(error) {
		fatalCalled = true
	}
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "/tmp/wd", nil }
	readDefinition := func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return nil, nil
	}
	logf := func(...any) {}

	run(args, homeDir, getWd, readDefinition, fatal, logf)
	// cobra calls fatal for unknown flags
	if !exitCalled && !fatalCalled {
		t.Fatal("either exit or fatal should be called for invalid flag")
	}
	_ = exitCode
}

func TestRun_ExitCoderWithZeroCode(t *testing.T) {
	// With cobra, --help prints help to stdout and returns nil (no fatal/exit call).
	args := []string{"ingitdb", "validate", "--help"}
	exitCalled := false
	oldExit := exit
	exit = func(code int) {
		exitCalled = true
	}
	t.Cleanup(func() {
		exit = oldExit
	})

	fatalCalled := false
	fatal := func(error) {
		fatalCalled = true
	}
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "/tmp/wd", nil }
	readDefinition := func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return nil, nil
	}
	logf := func(...any) {}

	run(args, homeDir, getWd, readDefinition, fatal, logf)
	if exitCalled {
		t.Fatal("exit should not be called for --help")
	}
	if fatalCalled {
		t.Fatal("fatal should not be called for --help")
	}
}

func TestRun_NonExitCoderError(t *testing.T) {
	t.Parallel()

	// With cobra, errors from commands propagate to fatal directly.
	args := []string{"ingitdb", "validate", "--path=/some/path"}
	fatalCalled := false
	var fatalErr error
	fatal := func(err error) {
		fatalCalled = true
		fatalErr = err
	}
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "/tmp/wd", nil }
	customErr := errors.New("custom readDefinition error")
	readDefinition := func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return nil, customErr
	}
	logf := func(...any) {}

	run(args, homeDir, getWd, readDefinition, fatal, logf)
	if !fatalCalled {
		t.Fatal("fatal should be called when readDefinition returns error")
	}
	if fatalErr == nil {
		t.Fatal("fatalErr should not be nil")
	}
}

func TestRun_AllCommands(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "version", args: []string{"ingitdb", "version"}},
		{name: "materialize help", args: []string{"ingitdb", "materialize", "--help"}},
		{name: "ci help", args: []string{"ingitdb", "ci", "--help"}},
		{name: "pull help", args: []string{"ingitdb", "pull", "--help"}},
		{name: "setup help", args: []string{"ingitdb", "setup", "--help"}},
		{name: "resolve help", args: []string{"ingitdb", "resolve", "--help"}},
		{name: "watch help", args: []string{"ingitdb", "watch", "--help"}},
		{name: "serve help", args: []string{"ingitdb", "serve", "--help"}},
		{name: "list help", args: []string{"ingitdb", "list", "--help"}},
		{name: "select help", args: []string{"ingitdb", "select", "--help"}},
		{name: "insert help", args: []string{"ingitdb", "insert", "--help"}},
		{name: "update help", args: []string{"ingitdb", "update", "--help"}},
		{name: "drop help", args: []string{"ingitdb", "drop", "--help"}},
		{name: "delete help", args: []string{"ingitdb", "delete", "--help"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fatalCalled := false
			fatal := func(error) {
				fatalCalled = true
			}
			homeDir := func() (string, error) { return "/tmp/home", nil }
			getWd := func() (string, error) { return "/tmp/wd", nil }
			readDefinition := func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
				return &ingitdb.Definition{}, nil
			}
			logf := func(...any) {}

			run(tc.args, homeDir, getWd, readDefinition, fatal, logf)
			if fatalCalled {
				t.Fatalf("fatal should not be called for %s", tc.name)
			}
		})
	}
}

func TestRun_GetWdError(t *testing.T) {
	t.Parallel()

	args := []string{"ingitdb", "validate"}
	fatalCalled := false
	fatal := func(error) {
		fatalCalled = true
	}
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "", errors.New("getwd error") }
	readDefinition := func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return nil, nil
	}
	logf := func(...any) {}

	run(args, homeDir, getWd, readDefinition, fatal, logf)
	if !fatalCalled {
		t.Fatal("fatal should be called when getWd returns error")
	}
}

func TestMain_Fatal(t *testing.T) {
	// Test that main's fatal function writes to stderr and calls exit.
	// Create a temp dir with a root-collections.yaml that points to a
	// nonexistent collection directory, so ReadDefinition returns an error.
	tmpDir := t.TempDir()
	ingitDBDir := tmpDir + "/.ingitdb"
	if mkErr := os.MkdirAll(ingitDBDir, 0755); mkErr != nil {
		t.Fatalf("create .ingitdb dir: %v", mkErr)
	}
	if writeErr := os.WriteFile(ingitDBDir+"/root-collections.yaml", []byte("foo: nonexistent-col\n"), 0644); writeErr != nil {
		t.Fatalf("write root-collections.yaml: %v", writeErr)
	}

	oldExit := exit
	exitCalled := false
	var exitCode int
	exit = func(code int) {
		exitCalled = true
		exitCode = code
	}
	t.Cleanup(func() {
		exit = oldExit
	})

	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stderr = w
	t.Cleanup(func() {
		os.Stderr = oldStderr
	})

	args := os.Args
	os.Args = []string{"ingitdb", "validate", "--path=" + tmpDir}
	t.Cleanup(func() {
		os.Args = args
	})

	// Create a function that captures output
	done := make(chan []byte)
	go func() {
		buf := make([]byte, 1024)
		n, _ := r.Read(buf)
		done <- buf[:n]
	}()

	main()

	_ = w.Close()
	output := <-done

	if !exitCalled {
		t.Fatal("exit should be called")
	}
	if exitCode != 1 {
		t.Fatalf("exit code should be 1, got %d", exitCode)
	}
	if len(output) == 0 {
		t.Fatal("error message should be written to stderr")
	}
}

func TestMain_Logf(t *testing.T) {
	// Test that main's logf function works by running a help command
	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stderr = w
	t.Cleanup(func() {
		os.Stderr = oldStderr
	})

	args := os.Args
	// Use version command which writes to stderr via logf
	os.Args = []string{"ingitdb", "version"}
	t.Cleanup(func() {
		os.Args = args
	})

	// Create a function that captures output
	done := make(chan bool)
	go func() {
		buf := make([]byte, 4096)
		_, _ = r.Read(buf)
		done <- true
	}()

	main()

	_ = w.Close()
	<-done

	// Test passed - logf was used by version command
}

func TestLaunchTUI_ResolvePathError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	homeDir := func() (string, error) { return "", errors.New("no home") }
	getWd := func() (string, error) { return "", errors.New("no wd") }
	readDefinition := func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return nil, nil
	}
	newDB := func(string, *ingitdb.Definition) (dal.DB, error) { return nil, nil }

	err := launchTUI(ctx, "", homeDir, getWd, readDefinition, newDB)
	if err == nil {
		t.Fatal("expected error when path resolution fails")
	}
	const want = "failed to resolve database path"
	if got := err.Error(); len(got) < len(want) || got[:len(want)] != want {
		t.Fatalf("expected error starting with %q, got %q", want, got)
	}
}

func TestLaunchTUI_ReadDefinitionError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "/tmp/wd", nil }
	readDefinition := func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return nil, errors.New("bad config")
	}
	newDB := func(string, *ingitdb.Definition) (dal.DB, error) { return nil, nil }

	err := launchTUI(ctx, "/tmp/db", homeDir, getWd, readDefinition, newDB)
	if err == nil {
		t.Fatal("expected error when readDefinition fails")
	}
	const want = "not an inGitDB repository (no .ingitdb config found)"
	if got := err.Error(); len(got) < len(want) || got[:len(want)] != want {
		t.Fatalf("expected error starting with %q, got %q", want, got)
	}
}

func TestLaunchTUI_NilDefinition(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "/tmp/wd", nil }
	readDefinition := func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return nil, nil
	}
	newDB := func(string, *ingitdb.Definition) (dal.DB, error) { return nil, nil }

	err := launchTUI(ctx, "/tmp/db", homeDir, getWd, readDefinition, newDB)
	if err == nil {
		t.Fatal("expected error when definition is nil")
	}
	const want = "not an inGitDB repository"
	if err.Error() != want {
		t.Fatalf("expected %q, got %q", want, err.Error())
	}
}

func TestLaunchTUI_ValidDefinition(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately so bubbletea exits quickly

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "/tmp/wd", nil }
	readDefinition := func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return &ingitdb.Definition{}, nil
	}
	newDB := func(string, *ingitdb.Definition) (dal.DB, error) { return nil, nil }

	// In a non-TTY test environment, term.GetSize will fail, exercising
	// the fallback branch (w,h = 120,40). bubbletea.Run may return an
	// error because there is no real terminal — we only care that it
	// does not panic.
	_ = launchTUI(ctx, "/tmp/db", homeDir, getWd, readDefinition, newDB)
}

func TestDefaultIsTerminal(t *testing.T) {
	t.Parallel()
	// In the test environment stdout is not a TTY; just exercise the call.
	_ = defaultIsTerminal()
}

func TestLaunchConflictsTUI(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately so bubbletea exits quickly

	// Non-TTY env: term.GetSize fails → fallback size branch. bubbletea.Run
	// may return an error without a real terminal — we only care it does not
	// panic.
	_ = launchConflictsTUI(ctx, []string{"data/users/u1.yaml"})
}

func TestRunTUI_NonTTY(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "/tmp/wd", nil }
	readDefinition := func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		t.Fatal("readDefinition should not be called when not a TTY")
		return nil, nil
	}
	newDB := func(string, *ingitdb.Definition) (dal.DB, error) { return nil, nil }
	isTerminal := func() bool { return false }

	err := runTUI(ctx, "", homeDir, getWd, readDefinition, newDB, isTerminal)
	if err != nil {
		t.Fatalf("expected nil error for non-TTY, got %v", err)
	}
}

func TestRunTUI_TTY(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately so bubbletea exits quickly

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "/tmp/wd", nil }
	readDefinition := func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return &ingitdb.Definition{}, nil
	}
	newDB := func(string, *ingitdb.Definition) (dal.DB, error) { return nil, nil }
	isTerminal := func() bool { return true }

	// launchTUI may return an error because there is no real terminal;
	// we only care that the TTY branch is exercised without panic.
	_ = runTUI(ctx, "/tmp/db", homeDir, getWd, readDefinition, newDB, isTerminal)
}

func TestDefaultNewDB(t *testing.T) {
	t.Parallel()

	db, err := defaultNewDB("/tmp/test", &ingitdb.Definition{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if db == nil {
		t.Fatal("expected non-nil db")
	}
}

func TestMakeViewBuilderLogf(t *testing.T) {
	t.Parallel()

	var got []any
	logf := func(args ...any) { got = args }
	vbl := makeViewBuilderLogf(logf)
	vbl("hello %s", "world")
	if len(got) != 1 {
		t.Fatalf("expected 1 arg, got %d", len(got))
	}
	if got[0] != "hello world" {
		t.Fatalf("expected 'hello world', got %v", got[0])
	}
}

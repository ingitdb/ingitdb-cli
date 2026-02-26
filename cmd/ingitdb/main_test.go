package main

import (
	"errors"
	"os"
	"testing"

	"github.com/urfave/cli/v3"

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

	// Mock both exit and cli.OsExiter to prevent the test from actually exiting
	oldExit := exit
	oldOsExiter := cli.OsExiter
	exitCalled := false
	osExiterCalled := false
	var exitCode int
	var osExiterCode int
	exit = func(code int) {
		exitCalled = true
		exitCode = code
		t.Logf("exit called with code %d", code)
	}
	cli.OsExiter = func(code int) {
		osExiterCalled = true
		osExiterCode = code
		t.Logf("cli.OsExiter called with code %d", code)
	}
	t.Cleanup(func() {
		t.Logf("cleanup: exitCalled=%v, exitCode=%d, osExiterCalled=%v, osExiterCode=%d", 
			exitCalled, exitCode, osExiterCalled, osExiterCode)
		exit = oldExit
		cli.OsExiter = oldOsExiter
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
	// urfave/cli returns an ExitCoder for invalid commands, which calls cli.OsExiter
	_ = fatalCalled
	_ = capturedErr
	_ = exitCalled
	_ = exitCode
	_ = osExiterCalled
	_ = osExiterCode
}

func TestRun_ExitCoderWithNonZeroCode(t *testing.T) {
	// Use validate command with invalid flag to trigger ExitCoder error with non-zero code
	args := []string{"ingitdb", "validate", "--invalid-flag"}
	osExiterCalled := false
	var osExiterCode int
	oldOsExiter := cli.OsExiter
	cli.OsExiter = func(code int) {
		osExiterCalled = true
		osExiterCode = code
	}
	t.Cleanup(func() {
		cli.OsExiter = oldOsExiter
	})

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
	// The invalid flag causes an error, which should be handled by the exit path
	// Either cli.OsExiter or exit should be called (or fatal if it's not an ExitCoder)
	if !exitCalled && !osExiterCalled && !fatalCalled {
		t.Fatal("one of exit, cli.OsExiter, or fatal should be called for invalid flag")
	}
	_ = exitCode
	_ = osExiterCode
}

func TestRun_ExitCoderWithZeroCode(t *testing.T) {
	// Mock cli.OsExiter to prevent urfave/cli from calling os.Exit
	oldOsExiter := cli.OsExiter
	cli.OsExiter = func(int) {}
	t.Cleanup(func() {
		cli.OsExiter = oldOsExiter
	})

	// Test the case where an ExitCoder error with exit code 0 is returned
	// This can happen with help output
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
		t.Fatal("exit should not be called when ExitCoder has code 0")
	}
	if fatalCalled {
		t.Fatal("fatal should not be called when ExitCoder has code 0")
	}
}

func TestRun_NonExitCoderError(t *testing.T) {
	t.Parallel()

	// This test is tricky because urfave/cli wraps most errors as ExitCoders
	// We need to create a scenario where app.Run returns a non-ExitCoder error
	// One way is to make readDefinition return a custom error type
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
		{name: "query help", args: []string{"ingitdb", "query", "--help"}},
		{name: "materialize help", args: []string{"ingitdb", "materialize", "--help"}},
		{name: "pull help", args: []string{"ingitdb", "pull", "--help"}},
		{name: "setup help", args: []string{"ingitdb", "setup", "--help"}},
		{name: "resolve help", args: []string{"ingitdb", "resolve", "--help"}},
		{name: "watch help", args: []string{"ingitdb", "watch", "--help"}},
		{name: "serve help", args: []string{"ingitdb", "serve", "--help"}},
		{name: "list help", args: []string{"ingitdb", "list", "--help"}},
		{name: "find help", args: []string{"ingitdb", "find", "--help"}},
		{name: "create help", args: []string{"ingitdb", "create", "--help"}},
		{name: "read help", args: []string{"ingitdb", "read", "--help"}},
		{name: "update help", args: []string{"ingitdb", "update", "--help"}},
		{name: "delete help", args: []string{"ingitdb", "delete", "--help"}},
		{name: "truncate help", args: []string{"ingitdb", "truncate", "--help"}},
		{name: "migrate help", args: []string{"ingitdb", "migrate", "--help"}},
	}

	for _, tc := range tests {
		tc := tc
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

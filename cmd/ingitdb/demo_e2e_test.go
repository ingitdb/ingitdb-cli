package main

// specscore: feature/cli/demo

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// demoExecEnv makes the test binary run the CLI instead of the tests, so the
// demo tests can run `ingitdb` as a real executable without building it.
const demoExecEnv = "INGITDB_TEST_RUN_MAIN"

func TestMain(m *testing.M) {
	if os.Getenv(demoExecEnv) == "1" {
		main()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// demoCLI is an `ingitdb` executable on a PATH of its own, with Git isolated
// from the machine's configuration and empty HOME and OpenVaultDB folders.
type demoCLI struct {
	env               []string
	home, ovdbHome    string
	ovdbDataHome, bin string
}

func newDemoCLI(t *testing.T) demoCLI {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	binDir := t.TempDir()
	name := "ingitdb"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	bin := filepath.Join(binDir, name)
	if linkErr := os.Link(exe, bin); linkErr != nil {
		copyFile(t, exe, bin)
	}
	c := demoCLI{home: t.TempDir(), ovdbHome: t.TempDir(), ovdbDataHome: t.TempDir(), bin: bin}
	gitConfig := filepath.Join(t.TempDir(), "gitconfig")
	if err = os.WriteFile(gitConfig, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, kv := range os.Environ() {
		key, value, _ := strings.Cut(kv, "=")
		upper := strings.ToUpper(key)
		switch {
		case upper == "PATH":
			c.env = append(c.env, key+"="+binDir+string(os.PathListSeparator)+value)
		case strings.HasPrefix(upper, "GIT_"), strings.HasPrefix(upper, "OVDB_"), upper == "HOME", upper == "XDG_CONFIG_HOME":
		default:
			c.env = append(c.env, kv)
		}
	}
	c.env = append(c.env,
		demoExecEnv+"=1",
		"GIT_CONFIG_GLOBAL="+gitConfig, "GIT_CONFIG_NOSYSTEM=1",
		"HOME="+c.home, "OVDB_HOME="+c.ovdbHome, "OVDB_DATA_HOME="+c.ovdbDataHome,
	)
	return c
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	in, err := os.Open(src)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = io.Copy(out, in); err != nil {
		t.Fatal(err)
	}
	if err = out.Close(); err != nil {
		t.Fatal(err)
	}
}

// run runs cmd with the CLI's environment in dir, stdin not a terminal, and
// returns stdout.
func (c demoCLI) run(t *testing.T, cmd *exec.Cmd, dir string) string {
	t.Helper()
	cmd.Dir = dir
	cmd.Env = c.env
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("%v failed: %v\nstdout:\n%s\nstderr:\n%s", cmd.Args, err, stdout.String(), stderr.String())
	}
	return stdout.String()
}

// shellCommand is one printed command and the shell it is printed for, empty
// for every shell of the platform.
type shellCommand struct{ shell, command string }

// nextSteps parses the What next? list of the human output into labels and
// commands.
func nextSteps(t *testing.T, stdout string) (labels []string, commands []shellCommand) {
	t.Helper()
	_, list, found := strings.Cut(stdout, "\nWhat next?\n")
	if !found {
		t.Fatalf("no What next? list:\n%s", stdout)
	}
	for _, line := range strings.Split(strings.TrimRight(list, "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "       "):
			_, label, _ := strings.Cut(trimmed, ". ")
			labels = append(labels, label)
		default:
			shell := ""
			for _, prefix := range []string{"cmd.exe:", "PowerShell:"} {
				if rest, ok := strings.CutPrefix(trimmed, prefix); ok {
					shell, trimmed = strings.TrimSuffix(prefix, ":"), strings.TrimSpace(rest)
				}
			}
			if strings.HasPrefix(trimmed, "ingitdb ") || strings.HasPrefix(trimmed, "ovdb ") {
				commands = append(commands, shellCommand{shell, trimmed})
			}
		}
	}
	return labels, commands
}

// TestDemoInstall_EndToEnd installs the demo with the real executable into a
// folder whose path contains a space, runs every printed ingitdb command
// through the platform's shell, validates the folder and checks it is a
// plain inGitDB folder with no OpenVaultDB coupling.
func TestDemoInstall_EndToEnd(t *testing.T) {
	t.Parallel()
	c := newDemoCLI(t)
	// A space, a cmd.exe variable (%OS%), a PowerShell variable ($HOME) and a
	// backtick: each must reach ingitdb literally through every shell.
	wd := filepath.Join(t.TempDir(), "my demo %OS% $HOME `x")
	if err := os.Mkdir(wd, 0o755); err != nil {
		t.Fatal(err)
	}
	stdout := c.run(t, exec.Command(c.bin, "demo", "install"), wd)
	target := filepath.Join(wd, "todo-demo")
	if !strings.HasPrefix(stdout, "The TODO demo is ready\n") {
		t.Fatalf("stdout:\n%s", stdout)
	}
	_, afterStored, _ := strings.Cut(stdout, "Stored in: ")
	printedPath, _, _ := strings.Cut(afterStored, "\n")
	if !sameResolvedDir(t, printedPath, target) {
		t.Errorf("printed path %q is not %q", printedPath, target)
	}

	labels, commands := nextSteps(t, stdout)
	wantLabels := []string{
		"Browse the lists in the terminal UI", "Query the lists", "Query what to buy",
		"Use the lists in a web TODO app (OpenVaultDB)",
	}
	if !reflect.DeepEqual(labels, wantLabels) {
		t.Errorf("labels = %q", labels)
	}
	ovdb := commands[len(commands)-2:]
	if ovdb[0].command != "ovdb demo install --yes" || ovdb[1].command != "ovdb demo open" {
		t.Fatalf("commands = %q", commands)
	}
	ingitdbCommands := commands[:len(commands)-2]
	wantPerStep := 1
	if runtime.GOOS == "windows" {
		wantPerStep = 2 // one for cmd.exe, one for PowerShell
	}
	if len(ingitdbCommands) != 3*wantPerStep {
		t.Fatalf("ingitdb commands = %q", ingitdbCommands)
	}
	quote := `'`
	if runtime.GOOS == "windows" {
		quote = `"`
	}
	for _, sc := range ingitdbCommands {
		if !strings.Contains(sc.command, "--path="+quote) {
			t.Errorf("path not quoted for %s: %s", runtime.GOOS, sc.command)
		}
		for _, shell := range demoShells(sc.command, sc.shell) {
			out := c.run(t, shell, t.TempDir())
			if strings.Contains(sc.command, "/items") && !strings.Contains(out, "Milk") {
				t.Errorf("%s via %s: no Milk in\n%s", sc.command, shell.Path, out)
			}
		}
	}

	c.run(t, exec.Command(c.bin, "validate", "--path="+target), wd)

	if got := dirNames(t, target); !reflect.DeepEqual(got, []string{".git", ".ingitdb", "lists"}) {
		t.Errorf("demo folder holds %v", got)
	}
	if got := dirNames(t, filepath.Join(target, ".ingitdb")); !reflect.DeepEqual(got, []string{"demo.yaml", "root-collections.yaml", "settings.yaml"}) {
		t.Errorf(".ingitdb holds %v", got)
	}
	for _, d := range []string{c.home, c.ovdbHome, c.ovdbDataHome} {
		if got := dirNames(t, d); len(got) != 0 {
			t.Errorf("%s should stay empty, holds %v", d, got)
		}
	}

	again := c.run(t, exec.Command(c.bin, "demo", "install"), wd)
	if !strings.HasPrefix(again, "The TODO demo is already installed in ") {
		t.Errorf("reinstall stdout:\n%s", again)
	}
}

// TestDemoInstall_NoOpenVaultDBDependency checks no OpenVaultDB package is
// in the CLI's dependency list.
func TestDemoInstall_NoOpenVaultDBDependency(t *testing.T) {
	t.Parallel()
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go tool not on PATH")
	}
	out, err := exec.Command(goTool, "list", "-deps", ".").Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			t.Fatalf("go list: %v\n%s", err, exitErr.Stderr)
		}
		t.Fatalf("go list: %v", err)
	}
	if !strings.Contains(string(out), "github.com/ingitdb/ingitdb-go/ingitdb/demos/todo") {
		t.Errorf("dependency list lacks the shared demo package")
	}
	if strings.Contains(string(out), "github.com/openvaultdb/") {
		t.Errorf("an OpenVaultDB package is a dependency:\n%s", out)
	}
}

func dirNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names
}

// sameResolvedDir compares two paths after resolving symbolic links (macOS
// temporary folders are under /var, a link to /private/var).
func sameResolvedDir(t *testing.T, a, b string) bool {
	t.Helper()
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	if errA != nil || errB != nil {
		t.Fatalf("resolve %q, %q: %v, %v", a, b, errA, errB)
	}
	return ra == rb
}

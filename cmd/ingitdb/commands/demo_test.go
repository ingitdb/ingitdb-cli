package commands

// specscore: feature/cli/demo

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/dal-go/dalgo/dal"
	"github.com/dal-go/record"
	"github.com/ingitdb/dalgo2ingitdb"
	"github.com/ingitdb/dalgo2ingitdb4local"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/ingitdb/ingitdb-go/ingitdb"
	"github.com/ingitdb/ingitdb-go/ingitdb/datavalidator"
	"github.com/ingitdb/ingitdb-go/ingitdb/demos/todo"
	"github.com/ingitdb/ingitdb-go/ingitdb/validator"
)

// demoTestNow is the install time every demo test uses.
var demoTestNow = time.Date(2026, 9, 17, 10, 0, 0, 0, time.UTC)

// isolatedGitEnv returns the process environment with Git isolated from the
// machine's configuration: GIT_CONFIG_GLOBAL points at a file holding
// globalConfig (empty for no identity), GIT_CONFIG_NOSYSTEM=1 and a temporary
// HOME. The VM and CI runners have a global identity that would otherwise
// hide the identity fallback.
func isolatedGitEnv(t *testing.T, globalConfig string) []string {
	t.Helper()
	home := t.TempDir()
	cfg := filepath.Join(home, "gitconfig")
	if err := os.WriteFile(cfg, []byte(globalConfig), 0o644); err != nil {
		t.Fatalf("write git config: %v", err)
	}
	env := make([]string, 0, len(os.Environ())+3)
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		upper := strings.ToUpper(name)
		if strings.HasPrefix(upper, "GIT_") || upper == "HOME" || upper == "XDG_CONFIG_HOME" {
			continue
		}
		env = append(env, kv)
	}
	return append(env, "GIT_CONFIG_GLOBAL="+cfg, "GIT_CONFIG_NOSYSTEM=1", "HOME="+home)
}

func requireGit(t *testing.T) string {
	t.Helper()
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not installed")
	}
	return git
}

// gitOut runs git in dir with env and fails the test on error.
func gitOut(t *testing.T, env []string, dir string, args ...string) string {
	t.Helper()
	out, err := runDemoGit(context.Background(), requireGit(t), dir, env, args...)
	if err != nil {
		t.Fatalf("%v", err)
	}
	return out
}

func localNewDB(root string, def *ingitdb.Definition) (dal.DB, error) {
	return dalgo2fsingitdb.NewLocalDBWithDef(root, def)
}

// testDemoInstaller is the real installer with a fixed clock and isolated Git.
func testDemoInstaller(t *testing.T) demoInstaller {
	t.Helper()
	i := newDemoInstaller(validator.ReadDefinition, localNewDB)
	i.now = func() time.Time { return demoTestNow }
	i.env = isolatedGitEnv(t, "")
	return i
}

// runDemoInstall runs `demo install` with args in working directory wd.
func runDemoInstall(t *testing.T, i demoInstaller, wd string, args ...string) (string, error) {
	t.Helper()
	homeDir := func() (string, error) { return filepath.Join(wd, "home"), nil }
	getWd := func() (string, error) { return wd, nil }
	cmd := demoInstallCommand(i, homeDir, getWd, runtime.GOOS)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	err := runCobraCommand(cmd, args...)
	return stdout.String(), err
}

// sameDir reports whether two absolute paths name the same folder after
// resolving symbolic links (macOS temporary folders are under /var, a link
// to /private/var).
func sameDir(t *testing.T, a, b string) bool {
	t.Helper()
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	if errA != nil || errB != nil {
		t.Fatalf("resolve %s, %s: %v, %v", a, b, errA, errB)
	}
	return ra == rb
}

// demoRecordFile is the on-disk path of a shared-package record key in the
// dalgo2ingitdb layout, e.g. lists/to-buy/items/$records/milk.yaml.
func demoRecordFile(dir, key string) string {
	segments := strings.Split(key, "/")
	parts := append([]string{dir}, segments[:len(segments)-1]...)
	parts = append(parts, "$records", segments[len(segments)-1]+".yaml")
	return filepath.Join(parts...)
}

// listTree returns every file and folder under dir, relative with forward
// slashes, skipping the inside of .git.
func listTree(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		if rel == "." {
			return nil
		}
		out = append(out, filepath.ToSlash(rel))
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}
	sort.Strings(out)
	return out
}

func TestDemoInstall_FreshInstall(t *testing.T) {
	t.Parallel()
	requireGit(t)
	wd := t.TempDir()
	stdout, err := runDemoInstall(t, testDemoInstaller(t), wd)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	dir := filepath.Join(wd, demoDefaultFolder)
	if !strings.HasPrefix(stdout, "The TODO demo is ready\n") {
		t.Errorf("stdout should start with the ready title:\n%s", stdout)
	}
	if !strings.Contains(stdout, "Stored in: "+dir+"\n") {
		t.Errorf("stdout should name the absolute folder %s:\n%s", dir, stdout)
	}
	marker, err := os.ReadFile(filepath.Join(dir, ".ingitdb", "demo.yaml"))
	if err != nil {
		t.Fatalf("marker: %v", err)
	}
	if string(marker) != "app: todo\nversion: 1\n" {
		t.Errorf("marker = %q", marker)
	}
	wantTree := []string{
		".git",
		".ingitdb", ".ingitdb/demo.yaml", ".ingitdb/root-collections.yaml", ".ingitdb/settings.yaml",
		"lists", "lists/$records", "lists/$records/to-buy.yaml", "lists/$records/to-watch.yaml",
		"lists/.collection", "lists/.collection/definition.yaml",
		"lists/.collection/subcollections", "lists/.collection/subcollections/items",
		"lists/.collection/subcollections/items/definition.yaml",
		"lists/to-buy", "lists/to-buy/items", "lists/to-buy/items/$records",
		"lists/to-buy/items/$records/bananas.yaml", "lists/to-buy/items/$records/coffee.yaml",
		"lists/to-buy/items/$records/milk.yaml",
		"lists/to-watch", "lists/to-watch/items", "lists/to-watch/items/$records",
		"lists/to-watch/items/$records/interstellar.yaml", "lists/to-watch/items/$records/the-matrix.yaml",
	}
	if got := listTree(t, dir); !reflect.DeepEqual(got, wantTree) {
		t.Errorf("demo folder tree =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(wantTree, "\n"))
	}
	def, err := validator.ReadDefinition(dir)
	if err != nil {
		t.Fatalf("read definition: %v", err)
	}
	result, err := datavalidator.NewValidator().Validate(context.Background(), dir, def)
	if err != nil || result.HasErrors() {
		t.Fatalf("validate: %v %v", err, result)
	}
}

// TestDemoInstall_EveryRecordValidAgainstDefinitions validates every root and
// item record file with datavalidator against the definitions the command
// wrote: `ingitdb validate` does not check subcollection records today.
func TestDemoInstall_EveryRecordValidAgainstDefinitions(t *testing.T) {
	t.Parallel()
	wd := t.TempDir()
	i := testDemoInstaller(t)
	i.lookPath = func(string) (string, error) { return "", exec.ErrNotFound }
	if _, err := runDemoInstall(t, i, wd); err != nil {
		t.Fatalf("install: %v", err)
	}
	dir := filepath.Join(wd, demoDefaultFolder)
	def, err := validator.ReadDefinition(dir)
	if err != nil {
		t.Fatalf("read definition: %v", err)
	}
	lists := def.Collections[demoListsCollection]
	items := lists.SubCollections[demoItemsCollection]
	if items == nil {
		t.Fatal("items subcollection not declared")
	}
	for _, r := range todo.Records(demoTestNow) {
		content, readErr := os.ReadFile(demoRecordFile(dir, r.Key))
		if readErr != nil {
			t.Fatalf("record %s: %v", r.Key, readErr)
		}
		var data map[string]any
		if yamlErr := yaml.Unmarshal(content, &data); yamlErr != nil {
			t.Fatalf("parse %s: %v", r.Key, yamlErr)
		}
		colDef := lists
		if strings.Count(r.Key, "/") > 1 {
			colDef = items
		}
		if errs := datavalidator.ValidateRecordData(colDef, r.Key, data); len(errs) > 0 {
			t.Errorf("record %s invalid: %v", r.Key, errs)
		}
	}
	// The check is not vacuous: an item without a title, with a string done
	// or a numeric added_at fails against the same definition.
	bad := map[string]any{"done": "no", "added_at": 12345}
	if errs := datavalidator.ValidateRecordData(items, "lists/to-buy/items/bad", bad); len(errs) < 3 {
		t.Errorf("invalid item should fail validation 3 times, got %v", errs)
	}
}

func TestDemoInstall_RecordsReadThroughDalgo2ingitdb(t *testing.T) {
	t.Parallel()
	requireGit(t)
	wd := t.TempDir()
	if _, err := runDemoInstall(t, testDemoInstaller(t), wd); err != nil {
		t.Fatalf("install: %v", err)
	}
	dir := filepath.Join(wd, demoDefaultFolder)
	db, err := dalgo2ingitdb.NewDatabase(dir, validator.NewCollectionsReader())
	if err != nil {
		t.Fatalf("open dalgo2ingitdb: %v", err)
	}
	for _, r := range todo.Records(demoTestNow) {
		data := map[string]any{}
		rec := record.NewRecordWithData(demoRecordKey(r.Key), data)
		if getErr := db.Get(context.Background(), rec); getErr != nil {
			t.Fatalf("get %s: %v", r.Key, getErr)
		}
		if !reflect.DeepEqual(data, r.Data) {
			t.Errorf("record %s = %#v, want %#v", r.Key, data, r.Data)
		}
	}
}

func TestDemoInstall_AtPath(t *testing.T) {
	t.Parallel()
	wd := t.TempDir()
	i := testDemoInstaller(t)
	stdout, err := runDemoInstall(t, i, wd, "--path=lists-demo")
	if err != nil {
		t.Fatalf("relative install: %v", err)
	}
	if !strings.Contains(stdout, filepath.Join(wd, "lists-demo")) {
		t.Errorf("stdout should name lists-demo:\n%s", stdout)
	}
	absolute := filepath.Join(t.TempDir(), "empty")
	if mkErr := os.Mkdir(absolute, 0o755); mkErr != nil {
		t.Fatal(mkErr)
	}
	if _, err = runDemoInstall(t, i, wd, "--path="+absolute); err != nil {
		t.Fatalf("absolute install: %v", err)
	}
	for _, d := range []string{filepath.Join(wd, "lists-demo"), absolute} {
		if _, statErr := os.Stat(filepath.Join(d, ".ingitdb", "demo.yaml")); statErr != nil {
			t.Errorf("no demo in %s: %v", d, statErr)
		}
	}
	if _, statErr := os.Stat(filepath.Join(wd, demoDefaultFolder)); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("todo-demo should not be created, stat err = %v", statErr)
	}
}

func TestDemoInstall_HomePath(t *testing.T) {
	t.Parallel()
	wd := t.TempDir()
	i := testDemoInstaller(t)
	i.lookPath = func(string) (string, error) { return "", exec.ErrNotFound }
	if _, err := runDemoInstall(t, i, wd, "--path=~/lists"); err != nil {
		t.Fatalf("install: %v", err)
	}
	if _, err := os.Stat(filepath.Join(wd, "home", "lists", ".ingitdb", "demo.yaml")); err != nil {
		t.Errorf("~ should expand to the home folder: %v", err)
	}
}

func TestDemoInstall_ReinstallKeepsEdits(t *testing.T) {
	t.Parallel()
	requireGit(t)
	wd := t.TempDir()
	i := testDemoInstaller(t)
	if _, err := runDemoInstall(t, i, wd); err != nil {
		t.Fatalf("install: %v", err)
	}
	dir := filepath.Join(wd, demoDefaultFolder)
	def, err := validator.ReadDefinition(dir)
	if err != nil {
		t.Fatal(err)
	}
	db, _ := localNewDB(dir, def)
	toBuy := record.NewKeyWithID("lists", "to-buy")
	err = db.RunReadwriteTransaction(context.Background(), func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		tea := record.NewRecordWithData(record.NewKeyWithParentAndID(toBuy, "items", "tea"),
			map[string]any{"title": "Tea", "done": false, "added_at": "2026-09-17T10:00:01Z"})
		if setErr := tx.Set(ctx, tea); setErr != nil {
			return setErr
		}
		return tx.Delete(ctx, record.NewKeyWithParentAndID(toBuy, "items", "coffee"))
	})
	if err != nil {
		t.Fatalf("edit demo: %v", err)
	}
	statusBefore := gitOut(t, i.env, dir, "status", "--porcelain")
	countBefore := gitOut(t, i.env, dir, "rev-list", "--count", "HEAD")

	stdout, err := runDemoInstall(t, i, wd)
	if err != nil {
		t.Fatalf("reinstall: %v", err)
	}
	if !strings.HasPrefix(stdout, "The TODO demo is already installed in "+dir+"\n") {
		t.Errorf("reinstall title missing:\n%s", stdout)
	}
	if !strings.Contains(stdout, "What next?") || !strings.Contains(stdout, "--from=lists/to-buy/items") {
		t.Errorf("reinstall should print the next steps:\n%s", stdout)
	}
	if !strings.Contains(stdout, "Git:       a Git repository, commit ") {
		t.Errorf("reinstall should report the repository:\n%s", stdout)
	}
	if _, statErr := os.Stat(demoRecordFile(dir, "lists/to-buy/items/tea")); statErr != nil {
		t.Errorf("tea should still exist: %v", statErr)
	}
	if _, statErr := os.Stat(demoRecordFile(dir, "lists/to-buy/items/coffee")); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("coffee should still be absent: %v", statErr)
	}
	if got := gitOut(t, i.env, dir, "status", "--porcelain"); got != statusBefore {
		t.Errorf("git status changed:\n%s\nwas\n%s", got, statusBefore)
	}
	if got := gitOut(t, i.env, dir, "rev-list", "--count", "HEAD"); got != countBefore {
		t.Errorf("commit count changed: %s, was %s", got, countBefore)
	}
}

func TestDemoInstall_NonEmptyFolderRefused(t *testing.T) {
	t.Parallel()
	wd := t.TempDir()
	dir := filepath.Join(wd, demoDefaultFolder)
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, err := runDemoInstall(t, testDemoInstaller(t), wd)
	if err == nil {
		t.Fatal("expected refusal")
	}
	for _, want := range []string{dir, "ingitdb demo install --path=<another folder>"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should contain %q: %v", want, err)
		}
	}
	if stdout != "" {
		t.Errorf("stdout should be empty on refusal: %q", stdout)
	}
	if got := listTree(t, dir); !reflect.DeepEqual(got, []string{"notes.txt"}) {
		t.Errorf("folder changed: %v", got)
	}
}

func TestDemoInstall_MarkerRefusals(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"other demo":     "app: notes\nversion: 1\n",
		"no app":         "version: 1\n",
		"invalid marker": "app: [\n",
	}
	for name, marker := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			wd := t.TempDir()
			markerPath := filepath.Join(wd, demoDefaultFolder, ".ingitdb", "demo.yaml")
			if err := writeDemoFile(markerPath, []byte(marker)); err != nil {
				t.Fatal(err)
			}
			_, err := runDemoInstall(t, testDemoInstaller(t), wd)
			if err == nil || !strings.Contains(err.Error(), "--path=<another folder>") {
				t.Fatalf("expected refusal, got %v", err)
			}
			if name == "other demo" && !strings.Contains(err.Error(), `"notes" demo`) {
				t.Errorf("error should name the other demo: %v", err)
			}
			if got := listTree(t, filepath.Join(wd, demoDefaultFolder)); len(got) != 2 {
				t.Errorf("folder changed: %v", got)
			}
		})
	}
}

func TestDemoInstall_FileTargetRefused(t *testing.T) {
	t.Parallel()
	wd := t.TempDir()
	file := filepath.Join(wd, demoDefaultFolder)
	if err := os.WriteFile(file, []byte("a file"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := runDemoInstall(t, testDemoInstaller(t), wd)
	if err == nil || !strings.Contains(err.Error(), "is a file") || !strings.Contains(err.Error(), file) {
		t.Fatalf("expected file refusal naming %s, got %v", file, err)
	}
	if content, _ := os.ReadFile(file); string(content) != "a file" {
		t.Errorf("file changed: %q", content)
	}
}

func TestDemoInstall_EmptyFolderUsed(t *testing.T) {
	t.Parallel()
	wd := t.TempDir()
	dir := filepath.Join(wd, demoDefaultFolder)
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	stdout, err := runDemoInstall(t, testDemoInstaller(t), wd)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if !strings.HasPrefix(stdout, "The TODO demo is ready") {
		t.Errorf("stdout:\n%s", stdout)
	}
	if _, statErr := os.Stat(filepath.Join(dir, ".ingitdb", "demo.yaml")); statErr != nil {
		t.Errorf("demo not installed into the empty folder: %v", statErr)
	}
}

// failOnFourthRecord makes the installer's record writer fail on the fourth
// record.
func failOnFourthRecord(i *demoInstaller) {
	count := 0
	i.setRecord = func(ctx context.Context, tx dal.ReadwriteTransaction, r record.Record) error {
		count++
		if count == 4 {
			return errors.New("disk full")
		}
		return tx.Set(ctx, r)
	}
}

func TestDemoInstall_FailureLeavesNothing(t *testing.T) {
	t.Parallel()
	for _, existing := range []bool{false, true} {
		name := "created folder"
		if existing {
			name = "existing empty folder"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			wd := t.TempDir()
			path := filepath.Join(wd, "nested", "todo-demo")
			if existing {
				if err := os.MkdirAll(path, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			i := testDemoInstaller(t)
			failOnFourthRecord(&i)
			stdout, err := runDemoInstall(t, i, wd, "--path="+path)
			if err == nil {
				t.Fatal("expected failure")
			}
			for _, want := range []string{"write record lists/to-buy/items/coffee", "disk full", path} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error should contain %q: %v", want, err)
				}
			}
			if stdout != "" {
				t.Errorf("stdout should be empty: %q", stdout)
			}
			if existing {
				if got := listTree(t, path); len(got) != 0 {
					t.Errorf("existing folder should be empty again: %v", got)
				}
				return
			}
			if got := listTree(t, wd); len(got) != 0 {
				t.Errorf("created folders should be removed: %v", got)
			}
		})
	}
}

// TestDemoInstall_NoTerminalDetection checks cli/demo#REQ:no-confirmation-prompt
// structurally: the command reads no input and never asks whether it runs on
// a terminal, so its output cannot differ between a terminal and a pipe.
func TestDemoInstall_NoTerminalDetection(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"demo.go", "demo_install.go", "demo_output.go", "demo_schema.go"} {
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{"IsTerminal", "x/term", "os.Stdin", "InOrStdin", "bufio"} {
			if bytes.Contains(src, []byte(forbidden)) {
				t.Errorf("%s references %s", name, forbidden)
			}
		}
	}
	// And an install with stdin at EOF completes without waiting.
	wd := t.TempDir()
	i := testDemoInstaller(t)
	cmd := demoInstallCommand(i, os.UserHomeDir, func() (string, error) { return wd, nil }, runtime.GOOS)
	cmd.SetIn(strings.NewReader(""))
	cmd.SetOut(&bytes.Buffer{})
	if err := runCobraCommand(cmd); err != nil {
		t.Fatalf("install: %v", err)
	}
}

func TestDemoInstall_OneCommitOwnRepository(t *testing.T) {
	t.Parallel()
	requireGit(t)
	cases := []struct {
		name, config, author string
	}{
		{name: "no identity", author: "inGitDB <ingitdb@localhost>"},
		{name: "only name", config: "[user]\n\tname = Ada Lovelace\n", author: "Ada Lovelace <ingitdb@localhost>"},
		{name: "only email", config: "[user]\n\temail = ada@example.com\n", author: "inGitDB <ada@example.com>"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			env := isolatedGitEnv(t, tc.config)
			outer := t.TempDir()
			gitOut(t, env, outer, "init", "-q")
			if err := os.WriteFile(filepath.Join(outer, "readme.txt"), []byte("outer"), 0o644); err != nil {
				t.Fatal(err)
			}
			gitOut(t, env, outer, "add", "readme.txt")
			gitOut(t, env, outer, "-c", "user.name=Outer", "-c", "user.email=outer@example.com", "commit", "-q", "-m", "outer")
			outerHead := gitOut(t, env, outer, "rev-parse", "HEAD")
			outerIndex := fileSHA(t, filepath.Join(outer, ".git", "index"))

			wd := filepath.Join(outer, "work")
			if err := os.Mkdir(wd, 0o755); err != nil {
				t.Fatal(err)
			}
			i := testDemoInstaller(t)
			i.env = env
			if _, err := runDemoInstall(t, i, wd, "--format=json"); err != nil {
				t.Fatalf("install: %v", err)
			}
			dir := filepath.Join(wd, demoDefaultFolder)
			top := gitOut(t, env, dir, "rev-parse", "--show-toplevel")
			if !sameDir(t, top, dir) {
				t.Errorf("demo should be its own repository, top level = %s", top)
			}
			if got := gitOut(t, env, dir, "rev-list", "--count", "HEAD"); got != "1" {
				t.Errorf("commit count = %s, want 1", got)
			}
			log := gitOut(t, env, dir, "log", "-1", "--format=%an <%ae>|%cn <%ce>|%s")
			if want := tc.author + "|" + tc.author + "|Install the TODO demo"; log != want {
				t.Errorf("commit = %q, want %q", log, want)
			}
			if status := gitOut(t, env, dir, "status", "--porcelain"); status != "" {
				t.Errorf("working tree not clean:\n%s", status)
			}
			localConfig, err := os.ReadFile(filepath.Join(dir, ".git", "config"))
			if err != nil || bytes.Contains(localConfig, []byte("[user")) {
				t.Errorf("demo .git/config must not gain a user section (err %v):\n%s", err, localConfig)
			}
			if got := gitOut(t, env, outer, "rev-parse", "HEAD"); got != outerHead {
				t.Errorf("enclosing HEAD changed: %s, was %s", got, outerHead)
			}
			if got := fileSHA(t, filepath.Join(outer, ".git", "index")); got != outerIndex {
				t.Error("enclosing index changed")
			}
		})
	}
}

func fileSHA(t *testing.T, name string) [32]byte {
	t.Helper()
	content, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return sha256.Sum256(content)
}

func TestDemoInstall_WithoutGit(t *testing.T) {
	t.Parallel()
	wd := t.TempDir()
	i := testDemoInstaller(t)
	i.lookPath = func(string) (string, error) { return "", exec.ErrNotFound }
	stdout, err := runDemoInstall(t, i, wd)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	dir := filepath.Join(wd, demoDefaultFolder)
	if _, statErr := os.Stat(filepath.Join(dir, ".git")); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf(".git should not exist: %v", statErr)
	}
	if _, statErr := os.Stat(demoRecordFile(dir, "lists/to-buy/items/milk")); statErr != nil {
		t.Errorf("records should be written: %v", statErr)
	}
	if !strings.Contains(stdout, "not a Git repository") || !strings.Contains(stdout, "git init") {
		t.Errorf("stdout should explain git init:\n%s", stdout)
	}
	// Reinstalling a folder without .git reports it the same way.
	stdout, err = runDemoInstall(t, i, wd, "--format=json")
	if err != nil {
		t.Fatalf("reinstall: %v", err)
	}
	doc := decodeDemoJSON(t, stdout)
	if doc.Git.Repository || doc.Git.Commit != "" || !doc.AlreadyInstalled {
		t.Errorf("reinstall without git = %+v", doc)
	}
}

func TestDemoInstall_ReinstallGitEdgeCases(t *testing.T) {
	t.Parallel()
	requireGit(t)
	t.Run("git removed after install", func(t *testing.T) {
		t.Parallel()
		wd := t.TempDir()
		i := testDemoInstaller(t)
		if _, err := runDemoInstall(t, i, wd); err != nil {
			t.Fatal(err)
		}
		i.lookPath = func(string) (string, error) { return "", exec.ErrNotFound }
		stdout, err := runDemoInstall(t, i, wd)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(stdout, "Git:       a Git repository\n") {
			t.Errorf("stdout:\n%s", stdout)
		}
	})
	t.Run("repository without commits", func(t *testing.T) {
		t.Parallel()
		wd := t.TempDir()
		i := testDemoInstaller(t)
		i.lookPath = func(string) (string, error) { return "", exec.ErrNotFound }
		if _, err := runDemoInstall(t, i, wd); err != nil {
			t.Fatal(err)
		}
		dir := filepath.Join(wd, demoDefaultFolder)
		i.lookPath = exec.LookPath
		gitOut(t, i.env, dir, "init", "-q")
		stdout, err := runDemoInstall(t, i, wd, "--format=json")
		if err != nil {
			t.Fatal(err)
		}
		if doc := decodeDemoJSON(t, stdout); !doc.Git.Repository || doc.Git.Commit != "" {
			t.Errorf("git = %+v, want repository without commit", doc.Git)
		}
	})
}

func decodeDemoJSON(t *testing.T, stdout string) demoResult {
	t.Helper()
	var doc demoResult
	dec := json.NewDecoder(strings.NewReader(stdout))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&doc); err != nil {
		t.Fatalf("stdout is not one JSON object: %v\n%s", err, stdout)
	}
	if dec.More() {
		t.Fatalf("stdout has more than one JSON document:\n%s", stdout)
	}
	return doc
}

func TestDemoInstall_JSONAndYAMLOutput(t *testing.T) {
	t.Parallel()
	requireGit(t)
	wd := t.TempDir()
	i := testDemoInstaller(t)
	for run, wantAlready := range []bool{false, true} {
		stdout, err := runDemoInstall(t, i, wd, "--format=json")
		if err != nil {
			t.Fatalf("run %d: %v", run, err)
		}
		doc := decodeDemoJSON(t, stdout)
		if doc.App != "todo" || !doc.Installed || doc.AlreadyInstalled != wantAlready {
			t.Errorf("run %d: %+v", run, doc)
		}
		if !filepath.IsAbs(doc.Path) || !sameDir(t, doc.Path, filepath.Join(wd, demoDefaultFolder)) {
			t.Errorf("run %d: path = %s", run, doc.Path)
		}
		if !doc.Git.Repository || len(doc.Git.Commit) != 40 {
			t.Errorf("run %d: git = %+v", run, doc.Git)
		}
		if !reflect.DeepEqual(doc.Lists, todo.Lists) || len(doc.Next) == 0 {
			t.Errorf("run %d: lists %v next %v", run, doc.Lists, doc.Next)
		}
		var generic map[string]any
		_ = json.Unmarshal([]byte(stdout), &generic)
		for _, key := range []string{"app", "installed", "already_installed", "path", "git", "lists", "next"} {
			if _, ok := generic[key]; !ok {
				t.Errorf("run %d: JSON lacks %q", run, key)
			}
		}
	}
	yamlOut, err := runDemoInstall(t, i, wd, "--format=yaml")
	if err != nil {
		t.Fatal(err)
	}
	jsonOut, _ := runDemoInstall(t, i, wd, "--format=json")
	var fromYAML, fromJSON map[string]any
	if yamlErr := yaml.Unmarshal([]byte(yamlOut), &fromYAML); yamlErr != nil {
		t.Fatalf("yaml: %v\n%s", yamlErr, yamlOut)
	}
	_ = json.Unmarshal([]byte(jsonOut), &fromJSON)
	yamlAsJSON, _ := json.Marshal(fromYAML)
	var normalized map[string]any
	_ = json.Unmarshal(yamlAsJSON, &normalized)
	if !reflect.DeepEqual(normalized, fromJSON) {
		t.Errorf("yaml document differs from json:\n%s\n%s", yamlOut, jsonOut)
	}
}

func TestDemoInstall_SelectListsAndItems(t *testing.T) {
	t.Parallel()
	wd := t.TempDir()
	if _, err := runDemoInstall(t, testDemoInstaller(t), wd); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(wd, demoDefaultFolder)
	stdout, err := runNestedSelect(t, dir, "--from=lists", "--format=json")
	if err != nil {
		t.Fatal(err)
	}
	rows := decodeJSONRows(t, stdout)
	if got := strings.Join(rowIDs(rows), ","); got != "to-buy,to-watch" {
		t.Errorf("lists = %s", got)
	}
	for _, row := range rows {
		if row["title"] == nil {
			t.Errorf("list without title: %v", row)
		}
	}
	stdout, err = runNestedSelect(t, dir, "--from=lists/to-buy/items", "--order-by=added_at", "--format=json")
	if err != nil {
		t.Fatal(err)
	}
	var titles []string
	for _, row := range decodeJSONRows(t, stdout) {
		title, _ := row["title"].(string)
		titles = append(titles, title)
	}
	if got := strings.Join(titles, ","); got != "Milk,Bananas,Coffee" {
		t.Errorf("to-buy items = %s", got)
	}
}

func TestDemoNextSteps_OrderAndQuoting(t *testing.T) {
	t.Parallel()
	steps := demoNextSteps("/tmp/my lists/todo-demo", "linux")
	var labels []string
	for _, s := range steps {
		if len(labels) == 0 || labels[len(labels)-1] != s.Label {
			labels = append(labels, s.Label)
		}
	}
	wantLabels := []string{
		"Browse the lists in the terminal UI", "Query the lists", "Query what to buy",
		"Use the lists in a web TODO app (OpenVaultDB)",
	}
	if !reflect.DeepEqual(labels, wantLabels) {
		t.Errorf("labels = %v", labels)
	}
	wantCommands := []string{
		"ingitdb --path='/tmp/my lists/todo-demo'",
		"ingitdb select --from=lists --path='/tmp/my lists/todo-demo'",
		"ingitdb select --from=lists/to-buy/items --path='/tmp/my lists/todo-demo'",
		"ovdb demo install --yes",
		"ovdb demo open",
	}
	for idx, s := range steps {
		if s.Command != wantCommands[idx] {
			t.Errorf("step %d = %q, want %q", idx, s.Command, wantCommands[idx])
		}
		if strings.Contains(s.Command, "&&") {
			t.Errorf("step %d chains commands", idx)
		}
	}
	if !strings.Contains(steps[4].Note, "https://github.com/openvaultdb/ovdb") {
		t.Errorf("OpenVaultDB note = %q", steps[4].Note)
	}
}

func TestDemoShellQuote(t *testing.T) {
	t.Parallel()
	cases := []struct{ in, goos, want string }{
		{`/home/ada/todo-demo`, "linux", `/home/ada/todo-demo`},
		{`/home/ada/my lists`, "darwin", `'/home/ada/my lists'`},
		{`/home/ada/it's`, "linux", `'/home/ada/it'\''s'`},
		{`/home/a\b`, "linux", `'/home/a\b'`},
		{`/tmp/$HOME`, "linux", `'/tmp/$HOME'`},
		{`C:\Users\ada\todo-demo`, "windows", `C:\Users\ada\todo-demo`},
		{`C:\Users\Ada Lovelace\todo-demo`, "windows", `"C:\Users\Ada Lovelace\todo-demo"`},
		{`C:\a&b`, "windows", `"C:\a&b"`},
		{``, "linux", `''`},
		{``, "windows", `""`},
	}
	for _, tc := range cases {
		if got := demoShellQuote(tc.in, tc.goos); got != tc.want {
			t.Errorf("demoShellQuote(%q, %s) = %s, want %s", tc.in, tc.goos, got, tc.want)
		}
	}
}

func TestDemoText_Variants(t *testing.T) {
	t.Parallel()
	result := newDemoResult("/x/todo-demo")
	result.Git = demoGitResult{Repository: true, Commit: "abc"}
	var b bytes.Buffer
	if err := writeDemoResult(&b, result, "", "linux"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Git:       a Git repository, commit abc\n",
		"  1. Browse the lists in the terminal UI\n       ingitdb --path=/x/todo-demo\n",
		"  4. Use the lists in a web TODO app (OpenVaultDB)\n       ovdb demo install --yes\n       ovdb demo open\n       OpenVaultDB keeps",
	} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("text lacks %q:\n%s", want, b.String())
		}
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("broken pipe") }

func TestWriteDemoResult_WriteError(t *testing.T) {
	t.Parallel()
	err := writeDemoResult(failingWriter{}, newDemoResult("/x"), "json", "linux")
	if err == nil || !strings.Contains(err.Error(), "broken pipe") {
		t.Errorf("err = %v", err)
	}
}

func TestDemoInstall_FlagAndPathErrors(t *testing.T) {
	t.Parallel()
	i := testDemoInstaller(t)
	wd := t.TempDir()
	if _, err := runDemoInstall(t, i, wd, "--format=csv"); err == nil || !strings.Contains(err.Error(), "yaml, json") {
		t.Errorf("unsupported format: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(wd, demoDefaultFolder)); !errors.Is(statErr, os.ErrNotExist) {
		t.Error("an unsupported format must not install")
	}
	failing := func() (string, error) { return "", errors.New("no wd") }
	cmd := demoInstallCommand(i, failing, failing, runtime.GOOS)
	if err := runCobraCommand(cmd); err == nil || !strings.Contains(err.Error(), "no wd") {
		t.Errorf("getWd error: %v", err)
	}
	cmd = demoInstallCommand(i, failing, failing, runtime.GOOS)
	if err := runCobraCommand(cmd, "--path=~/x"); err == nil || !strings.Contains(err.Error(), "no wd") {
		t.Errorf("homeDir error: %v", err)
	}
	cmd = demoInstallCommand(i, failing, failing, runtime.GOOS)
	cmd.SetOut(failingWriter{})
	if err := runCobraCommand(cmd, "--path="+filepath.Join(wd, "out")); err == nil || !strings.Contains(err.Error(), "broken pipe") {
		t.Errorf("output error: %v", err)
	}
}

func TestDemoGroup_HelpListsInstall(t *testing.T) {
	t.Parallel()
	cmd := Demo(os.UserHomeDir, os.Getwd, validator.ReadDefinition, localNewDB)
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := runCobraCommand(cmd); err != nil {
		t.Fatalf("demo without subcommand: %v", err)
	}
	if !strings.Contains(out.String(), "install") {
		t.Errorf("help should list install:\n%s", out.String())
	}
	if !slices.ContainsFunc(cmd.Commands(), func(c *cobra.Command) bool { return c.Name() == "install" }) {
		t.Error("install subcommand missing")
	}
}

// failingInstaller returns an installer (without git unless withGit) whose
// dependency named by fail returns an error.
func failingInstaller(t *testing.T, fail string) demoInstaller {
	t.Helper()
	i := testDemoInstaller(t)
	boom := errors.New("boom " + fail)
	realGit := i.runGit
	i.runGit = func(ctx context.Context, git, dir string, env []string, args ...string) (string, error) {
		if args[0] == fail {
			return "", boom
		}
		return realGit(ctx, git, dir, env, args...)
	}
	switch fail {
	case "writeFile":
		i.writeFile = func(string, []byte) error { return boom }
	case "readDefinition":
		i.readDefinition = func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return nil, boom }
	case "newDB":
		i.newDB = func(string, *ingitdb.Definition) (dal.DB, error) { return nil, boom }
	}
	return i
}

func TestDemoInstall_StepFailuresCleanUp(t *testing.T) {
	t.Parallel()
	requireGit(t)
	for _, fail := range []string{"init", "writeFile", "readDefinition", "newDB", "add", "commit", "rev-parse"} {
		t.Run(fail, func(t *testing.T) {
			t.Parallel()
			wd := t.TempDir()
			_, err := runDemoInstall(t, failingInstaller(t, fail), wd)
			if err == nil || !strings.Contains(err.Error(), "boom "+fail) || !strings.Contains(err.Error(), "failed to install the TODO demo") {
				t.Fatalf("err = %v", err)
			}
			if got := listTree(t, wd); len(got) != 0 {
				t.Errorf("leftovers after %s failure: %v", fail, got)
			}
		})
	}
}

func TestRunDemoGit_ReportsStderr(t *testing.T) {
	t.Parallel()
	git := requireGit(t)
	_, err := runDemoGit(context.Background(), git, t.TempDir(), isolatedGitEnv(t, ""), "no-such-subcommand")
	if err == nil || !strings.Contains(err.Error(), "no-such-subcommand") {
		t.Errorf("err = %v", err)
	}
}

func TestWriteDemoFile_ParentIsFile(t *testing.T) {
	t.Parallel()
	parent := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(parent, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeDemoFile(filepath.Join(parent, "child", "x.yaml"), nil); err == nil {
		t.Error("expected error when a parent is a file")
	}
}

func TestMissingDirs(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	want := []string{filepath.Join(base, "a"), filepath.Join(base, "a", "b"), filepath.Join(base, "a", "b", "c")}
	if got := missingDirs(filepath.Join(base, "a", "b", "c")); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if got := missingDirs(base); len(got) != 0 {
		t.Errorf("existing dir: got %v", got)
	}
}

// skipWithoutPermissionChecks skips tests that rely on POSIX permission
// denials, which Windows and root do not enforce.
func skipWithoutPermissionChecks(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs POSIX permission checks")
	}
}

// chmodForTest changes mode and restores 0o755 when the test ends.
func chmodForTest(t *testing.T, name string, mode os.FileMode) {
	t.Helper()
	if err := os.Chmod(name, mode); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(name, 0o755) })
}

func TestDemoInstall_FileSystemErrors(t *testing.T) {
	t.Parallel()
	skipWithoutPermissionChecks(t)
	t.Run("parent is a file", func(t *testing.T) {
		t.Parallel()
		wd := t.TempDir()
		file := filepath.Join(wd, "file")
		if err := os.WriteFile(file, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := runDemoInstall(t, testDemoInstaller(t), wd, "--path=file/todo-demo")
		if err == nil || !strings.Contains(err.Error(), "cannot use") {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("unreadable folder", func(t *testing.T) {
		t.Parallel()
		wd := t.TempDir()
		dir := filepath.Join(wd, demoDefaultFolder)
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		chmodForTest(t, dir, 0o000)
		_, err := runDemoInstall(t, testDemoInstaller(t), wd)
		if err == nil || !strings.Contains(err.Error(), "cannot read") {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("read-only parent", func(t *testing.T) {
		t.Parallel()
		wd := t.TempDir()
		chmodForTest(t, wd, 0o555)
		_, err := runDemoInstall(t, testDemoInstaller(t), wd, "--path=new/todo-demo")
		if err == nil || !strings.Contains(err.Error(), "create the folder") {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("created folder cannot be removed", func(t *testing.T) {
		t.Parallel()
		wd := t.TempDir()
		i := testDemoInstaller(t)
		i.setRecord = func(context.Context, dal.ReadwriteTransaction, record.Record) error {
			chmodForTest(t, wd, 0o555)
			return errors.New("disk full")
		}
		_, err := runDemoInstall(t, i, wd)
		if err == nil || !strings.Contains(err.Error(), "disk full") || !strings.Contains(err.Error(), "failed to remove what was written") {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("emptied folder cannot be read", func(t *testing.T) {
		t.Parallel()
		wd := t.TempDir()
		dir := filepath.Join(wd, demoDefaultFolder)
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		i := testDemoInstaller(t)
		i.setRecord = func(context.Context, dal.ReadwriteTransaction, record.Record) error {
			chmodForTest(t, dir, 0o000)
			return errors.New("disk full")
		}
		_, err := runDemoInstall(t, i, wd)
		if err == nil || !strings.Contains(err.Error(), "failed to remove what was written") {
			t.Errorf("err = %v", err)
		}
	})
}

// TestDemoInstall_IgnoresInheritedRepositoryVariables runs the install with
// GIT_DIR and GIT_INDEX_FILE pointing at an enclosing repository, as they are
// inside Git hooks, `rebase --exec` or dotfile managers. The install must
// still create and commit to its own repository and leave the enclosing one
// untouched (cli/demo#REQ:own-git-repository).
func TestDemoInstall_IgnoresInheritedRepositoryVariables(t *testing.T) {
	t.Parallel()
	requireGit(t)
	env := isolatedGitEnv(t, "")
	outer := t.TempDir()
	gitOut(t, env, outer, "init", "-q")
	if err := os.WriteFile(filepath.Join(outer, "x"), []byte("outer"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitOut(t, env, outer, "add", "x")
	gitOut(t, env, outer, "-c", "user.name=Outer", "-c", "user.email=outer@example.com", "commit", "-q", "-m", "outer")
	outerHead := gitOut(t, env, outer, "rev-parse", "HEAD")
	outerIndex := fileSHA(t, filepath.Join(outer, ".git", "index"))

	i := testDemoInstaller(t)
	i.env = append(slices.Clone(env),
		"GIT_DIR="+filepath.Join(outer, ".git"),
		"GIT_INDEX_FILE="+filepath.Join(outer, ".git", "index"),
		"GIT_WORK_TREE="+outer,
	)
	stdout, err := runDemoInstall(t, i, outer, "--path=inner", "--format=json")
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	inner := filepath.Join(outer, "inner")
	if got := gitOut(t, env, outer, "rev-parse", "HEAD"); got != outerHead {
		t.Errorf("enclosing HEAD changed: %s, was %s", got, outerHead)
	}
	if got := fileSHA(t, filepath.Join(outer, ".git", "index")); got != outerIndex {
		t.Error("enclosing index changed")
	}
	if got := gitOut(t, env, outer, "ls-tree", "--name-only", "HEAD"); got != "x" {
		t.Errorf("enclosing tree = %q, want x", got)
	}
	if got := gitOut(t, env, inner, "rev-list", "--count", "HEAD"); got != "1" {
		t.Errorf("inner commit count = %s, want 1", got)
	}
	if top := gitOut(t, env, inner, "rev-parse", "--show-toplevel"); !sameDir(t, top, inner) {
		t.Errorf("inner top level = %s", top)
	}
	if doc := decodeDemoJSON(t, stdout); doc.Git.Commit != gitOut(t, env, inner, "rev-parse", "HEAD") {
		t.Errorf("reported commit %s is not the inner HEAD", doc.Git.Commit)
	}
}

// TestDemoRepositoryEnvVars_CoverGit checks the stripped variables include
// every repository-local variable the installed git reports.
func TestDemoRepositoryEnvVars_CoverGit(t *testing.T) {
	t.Parallel()
	vars := gitOut(t, isolatedGitEnv(t, ""), t.TempDir(), "rev-parse", "--local-env-vars")
	for _, name := range strings.Fields(vars) {
		if !slices.Contains(demoRepositoryEnvVars, name) {
			t.Errorf("git reports %s, which the install does not strip", name)
		}
	}
	got := withoutRepositoryEnv([]string{"git_dir=/x", "GIT_INDEX_FILE=/i", "GIT_CONFIG_GLOBAL=/g", "PATH=/bin"})
	if !reflect.DeepEqual(got, []string{"GIT_CONFIG_GLOBAL=/g", "PATH=/bin"}) {
		t.Errorf("withoutRepositoryEnv = %v", got)
	}
}

// TestDemoInstall_ConcurrentInstalls runs several installs into one path at
// once. Exactly one installs; every other one either reports the finished
// demo as already installed or fails saying an install is in progress, and
// the folder ends up a complete demo with one commit.
func TestDemoInstall_ConcurrentInstalls(t *testing.T) {
	t.Parallel()
	requireGit(t)
	const rounds, racers = 4, 4
	for round := 0; round < rounds; round++ {
		wd := t.TempDir()
		installers := make([]demoInstaller, racers)
		for r := range installers {
			installers[r] = testDemoInstaller(t)
		}
		type outcome struct {
			stdout string
			err    error
		}
		outcomes := make([]outcome, racers)
		done := make(chan int)
		for r := range installers {
			go func(r int) {
				stdout, err := runDemoInstall(t, installers[r], wd, "--path=shared/todo-demo", "--format=json")
				outcomes[r] = outcome{stdout, err}
				done <- r
			}(r)
		}
		for range installers {
			<-done
		}
		dir := filepath.Join(wd, "shared", "todo-demo")
		head := gitOut(t, installers[0].env, dir, "rev-parse", "HEAD")
		fresh := 0
		for r, o := range outcomes {
			if o.err != nil {
				if !strings.Contains(o.err.Error(), "in progress") {
					t.Errorf("round %d racer %d: unexpected error %v", round, r, o.err)
				}
				continue
			}
			doc := decodeDemoJSON(t, o.stdout)
			if !doc.AlreadyInstalled {
				fresh++
			}
			if doc.Git.Commit != head {
				t.Errorf("round %d racer %d: reported commit %q, final HEAD %s", round, r, doc.Git.Commit, head)
			}
		}
		if fresh != 1 {
			t.Errorf("round %d: %d fresh installs, want 1", round, fresh)
		}
		if got := gitOut(t, installers[0].env, dir, "rev-list", "--count", "HEAD"); got != "1" {
			t.Errorf("round %d: %s commits", round, got)
		}
		if status := gitOut(t, installers[0].env, dir, "status", "--porcelain"); status != "" {
			t.Errorf("round %d: working tree not clean:\n%s", round, status)
		}
		if _, err := os.Stat(demoRecordFile(dir, "lists/to-watch/items/interstellar")); err != nil {
			t.Errorf("round %d: demo incomplete: %v", round, err)
		}
	}
}

// TestDemoInstall_SecondRunDuringInstall starts a second install into the
// same folder while the first is writing records: the second must neither
// report the unfinished demo as installed nor remove anything.
func TestDemoInstall_SecondRunDuringInstall(t *testing.T) {
	t.Parallel()
	requireGit(t)
	wd := t.TempDir()
	first := testDemoInstaller(t)
	second := testDemoInstaller(t)
	var secondErr error
	var secondOut string
	count := 0
	first.setRecord = func(ctx context.Context, tx dal.ReadwriteTransaction, r record.Record) error {
		count++
		if count == 7 {
			secondOut, secondErr = runDemoInstall(t, second, wd)
		}
		return tx.Set(ctx, r)
	}
	if _, err := runDemoInstall(t, first, wd); err != nil {
		t.Fatalf("first install: %v", err)
	}
	if secondErr == nil || !strings.Contains(secondErr.Error(), "in progress") {
		t.Errorf("second install during the first: stdout %q, err %v; want an in-progress error", secondOut, secondErr)
	}
	dir := filepath.Join(wd, demoDefaultFolder)
	if got := gitOut(t, first.env, dir, "rev-list", "--count", "HEAD"); got != "1" {
		t.Errorf("%s commits", got)
	}
	if status := gitOut(t, first.env, dir, "status", "--porcelain"); status != "" {
		t.Errorf("working tree not clean:\n%s", status)
	}
	// Once the first install finished, a second run reports it installed.
	stdout, err := runDemoInstall(t, second, wd)
	if err != nil || !strings.Contains(stdout, "already installed") {
		t.Errorf("after the first install: %q, %v", stdout, err)
	}
}

// TestDemoInstall_FailureKeepsOthersFiles makes an install fail after another
// process added a sibling next to the parent folder it created: the failed
// run removes only what it created.
func TestDemoInstall_FailureKeepsOthersFiles(t *testing.T) {
	t.Parallel()
	wd := t.TempDir()
	sibling := filepath.Join(wd, "newparent", "sibling")
	i := testDemoInstaller(t)
	i.setRecord = func(context.Context, dal.ReadwriteTransaction, record.Record) error {
		if err := os.Mkdir(sibling, 0o755); err != nil {
			return err
		}
		return errors.New("disk full")
	}
	if _, err := runDemoInstall(t, i, wd, "--path=newparent/todo-demo"); err == nil {
		t.Fatal("expected failure")
	}
	if got := listTree(t, wd); !reflect.DeepEqual(got, []string{"newparent", "newparent/sibling"}) {
		t.Errorf("after failure wd holds %v, want only newparent/sibling", got)
	}
}

// TestDemoInstall_InterruptedInstallRefused leaves a folder the way a killed
// install does, holding the install lock: a new run refuses it as in
// progress and removes nothing.
func TestDemoInstall_InterruptedInstallRefused(t *testing.T) {
	t.Parallel()
	wd := t.TempDir()
	lock := filepath.Join(wd, demoDefaultFolder, demoLockFileName)
	if err := writeDemoFile(lock, nil); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(wd, demoDefaultFolder, ".ingitdb", "demo.yaml")
	if err := writeDemoFile(marker, []byte("app: todo\nversion: 1\n")); err != nil {
		t.Fatal(err)
	}
	_, err := runDemoInstall(t, testDemoInstaller(t), wd)
	if err == nil || !strings.Contains(err.Error(), "in progress") || !strings.Contains(err.Error(), filepath.Join(wd, demoDefaultFolder)) {
		t.Fatalf("err = %v, want in progress naming the folder", err)
	}
	if got := listTree(t, filepath.Join(wd, demoDefaultFolder)); len(got) != 3 {
		t.Errorf("folder changed: %v", got)
	}
}

// TestDemoInstall_ClaimRaces drives the branches a lost race takes, through
// the folder and lock seams.
func TestDemoInstall_ClaimRaces(t *testing.T) {
	t.Parallel()
	folder := func(wd string) string { return filepath.Join(wd, demoDefaultFolder) }
	exist := &os.PathError{Op: "open", Err: os.ErrExist}
	t.Run("folder created by another run", func(t *testing.T) {
		t.Parallel()
		wd := t.TempDir()
		i := testDemoInstaller(t)
		i.mkdir = func(name string, perm os.FileMode) error {
			if err := os.Mkdir(name, perm); err != nil {
				return err
			}
			return &os.PathError{Op: "mkdir", Path: name, Err: os.ErrExist}
		}
		if _, err := runDemoInstall(t, i, wd); err != nil {
			t.Fatal(err)
		}
		if _, err := runDemoInstall(t, i, wd); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("lock taken, demo completed meanwhile", func(t *testing.T) {
		t.Parallel()
		wd := t.TempDir()
		i := testDemoInstaller(t)
		i.lookPath = func(string) (string, error) { return "", exec.ErrNotFound }
		i.createLock = func(name string) error {
			if err := writeDemoFile(filepath.Join(folder(wd), ".ingitdb", "demo.yaml"), []byte("app: todo\n")); err != nil {
				return err
			}
			return exist
		}
		stdout, err := runDemoInstall(t, i, wd)
		if err != nil || !strings.Contains(stdout, "already installed") {
			t.Errorf("stdout %q, err %v", stdout, err)
		}
	})
	t.Run("lock taken by a running install", func(t *testing.T) {
		t.Parallel()
		wd := t.TempDir()
		i := testDemoInstaller(t)
		i.createLock = func(name string) error {
			if err := createDemoLock(name); err != nil {
				return err
			}
			return exist
		}
		if _, err := runDemoInstall(t, i, wd); err == nil || !strings.Contains(err.Error(), "in progress") {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("lock taken and released without a demo", func(t *testing.T) {
		t.Parallel()
		wd := t.TempDir()
		i := testDemoInstaller(t)
		i.createLock = func(string) error { return exist }
		if _, err := runDemoInstall(t, i, wd); err == nil || !strings.Contains(err.Error(), "in progress") {
			t.Errorf("err = %v", err)
		}
		if got := listTree(t, wd); len(got) != 0 {
			t.Errorf("created folder left behind: %v", got)
		}
	})
	t.Run("lock cannot be created", func(t *testing.T) {
		t.Parallel()
		wd := t.TempDir()
		i := testDemoInstaller(t)
		i.createLock = func(string) error { return errors.New("read-only file system") }
		if _, err := runDemoInstall(t, i, wd); err == nil || !strings.Contains(err.Error(), "claim the folder") {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("lock cannot be released", func(t *testing.T) {
		t.Parallel()
		wd := t.TempDir()
		i := testDemoInstaller(t)
		i.removeLock = func(string) error { return errors.New("busy") }
		if _, err := runDemoInstall(t, i, wd); err == nil || !strings.Contains(err.Error(), "busy") {
			t.Errorf("err = %v", err)
		}
		if got := listTree(t, wd); len(got) != 0 {
			t.Errorf("leftovers: %v", got)
		}
	})
}

func TestCreateDemoLock_Exclusive(t *testing.T) {
	t.Parallel()
	lock := filepath.Join(t.TempDir(), demoLockFileName)
	if err := createDemoLock(lock); err != nil {
		t.Fatal(err)
	}
	if err := createDemoLock(lock); !errors.Is(err, os.ErrExist) {
		t.Errorf("second claim err = %v, want ErrExist", err)
	}
}

func TestDemoInstall_MarkerWriteFails(t *testing.T) {
	t.Parallel()
	wd := t.TempDir()
	i := testDemoInstaller(t)
	i.writeFile = func(name string, data []byte) error {
		if filepath.Base(name) == demoMarkerFileName {
			return errors.New("boom marker")
		}
		return writeDemoFile(name, data)
	}
	if _, err := runDemoInstall(t, i, wd); err == nil || !strings.Contains(err.Error(), "boom marker") {
		t.Errorf("err = %v", err)
	}
	if got := listTree(t, wd); len(got) != 0 {
		t.Errorf("leftovers: %v", got)
	}
}

// TestDemoInstall_DanglingSymlinkRefused points --path at a symbolic link to
// a missing folder: it is refused before anything is written and the link is
// kept.
func TestDemoInstall_DanglingSymlinkRefused(t *testing.T) {
	t.Parallel()
	wd := t.TempDir()
	link := filepath.Join(wd, "dang")
	if err := os.Symlink(filepath.Join(wd, "nowhere"), link); err != nil {
		t.Skipf("cannot create symbolic links here: %v", err)
	}
	_, err := runDemoInstall(t, testDemoInstaller(t), wd, "--path=dang")
	if err == nil || !strings.Contains(err.Error(), link) || !strings.Contains(err.Error(), "--path=<another folder>") {
		t.Fatalf("err = %v, want a refusal naming %s", err, link)
	}
	if _, lstatErr := os.Lstat(link); lstatErr != nil {
		t.Errorf("the link was removed: %v", lstatErr)
	}
	if got := listTree(t, wd); !reflect.DeepEqual(got, []string{"dang"}) {
		t.Errorf("wd holds %v", got)
	}
}

// TestDemoInstall_CommitRefusedHint makes the user's Git configuration refuse
// the install commit, with a failing commit hook and with commit signing
// that cannot sign: the error says why and how to install anyway, and the
// hooks are not bypassed.
func TestDemoInstall_CommitRefusedHint(t *testing.T) {
	t.Parallel()
	requireGit(t)
	hooks := t.TempDir()
	hook := "#!/bin/sh\necho blocked by corp hook >&2\nexit 1\n"
	if err := os.WriteFile(filepath.Join(hooks, "pre-commit"), []byte(hook), 0o755); err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"hook":    "[core]\n\thooksPath = " + filepath.ToSlash(hooks) + "\n",
		"signing": "[commit]\n\tgpgsign = true\n[gpg]\n\tprogram = " + filepath.ToSlash(filepath.Join(hooks, "no-such-gpg")) + "\n",
	}
	for name, config := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			wd := t.TempDir()
			i := testDemoInstaller(t)
			i.env = isolatedGitEnv(t, config)
			_, err := runDemoInstall(t, i, wd)
			if err == nil {
				t.Fatal("expected the commit to be refused")
			}
			for _, want := range []string{"commit hook", "commit.gpgsign", "GIT_CONFIG_GLOBAL"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error lacks %q: %v", want, err)
				}
			}
			if name == "hook" && !strings.Contains(err.Error(), "blocked by corp hook") {
				t.Errorf("the hook did not run: %v", err)
			}
			if got := listTree(t, wd); len(got) != 0 {
				t.Errorf("leftovers: %v", got)
			}
		})
	}
}

// TestDemoInstall_JSONNextSteps checks each `next` entry names its numbered
// step, so the two OpenVaultDB commands read as one step, as in the text.
func TestDemoInstall_JSONNextSteps(t *testing.T) {
	t.Parallel()
	var b bytes.Buffer
	if err := writeDemoResult(&b, newDemoResult("/x/todo-demo"), "json", "linux"); err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Next []map[string]any `json:"next"`
	}
	if err := json.Unmarshal(b.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	var steps []float64
	for _, n := range doc.Next {
		step, _ := n["step"].(float64)
		steps = append(steps, step)
	}
	if !reflect.DeepEqual(steps, []float64{1, 2, 3, 4, 4}) {
		t.Errorf("steps = %v, want [1 2 3 4 4]\n%s", steps, b.String())
	}
}

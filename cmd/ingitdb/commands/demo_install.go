package commands

// specscore: feature/cli/demo

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/dal-go/dalgo/dal"
	"github.com/dal-go/record"
	"gopkg.in/yaml.v3"

	"github.com/ingitdb/ingitdb-go/ingitdb"
	"github.com/ingitdb/ingitdb-go/ingitdb/config"
	"github.com/ingitdb/ingitdb-go/ingitdb/demos/todo"
)

// demoCommitRefusedHint follows a failed install commit. The install honours
// the user's hooks and signing policy rather than bypassing them.
const demoCommitRefusedHint = "Git refused the install commit. A commit hook (core.hooksPath) or commit signing " +
	"(commit.gpgsign) in your Git configuration may have blocked it. Fix that and install again, " +
	"or install once without your global and system Git configuration: " +
	"GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 ingitdb demo install " +
	"(PowerShell: $env:GIT_CONFIG_GLOBAL='NUL'; $env:GIT_CONFIG_NOSYSTEM='1'; ingitdb demo install)"

const (
	demoDefaultGitName  = "inGitDB"
	demoDefaultGitEmail = "ingitdb@localhost"
)

// demoInstaller installs the TODO demo into a folder. Its fields are the
// command's dependencies, so tests replace them without package state.
type demoInstaller struct {
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error)
	newDB          func(string, *ingitdb.Definition) (dal.DB, error)
	now            func() time.Time
	// lookPath finds the git executable; an error means git is unavailable.
	lookPath func(string) (string, error)
	// env is the environment git runs with.
	env    []string
	runGit func(ctx context.Context, git, dir string, env []string, args ...string) (string, error)
	// writeFile writes one file, creating its parent folders.
	writeFile func(name string, data []byte) error
	// mkdir creates one folder.
	mkdir func(name string, perm os.FileMode) error
	// createLock claims the demo folder; removeLock releases it.
	createLock func(name string) error
	removeLock func(name string) error
	// setRecord writes one record in the install transaction.
	setRecord func(ctx context.Context, tx dal.ReadwriteTransaction, r record.Record) error
}

// newDemoInstaller returns an installer wired to the real file system, git
// and the local inGitDB driver.
func newDemoInstaller(
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
) demoInstaller {
	return demoInstaller{
		readDefinition: readDefinition,
		newDB:          newDB,
		now:            time.Now,
		lookPath:       exec.LookPath,
		env:            os.Environ(),
		runGit:         runDemoGit,
		writeFile:      writeDemoFile,
		mkdir:          os.Mkdir,
		createLock:     createDemoLock,
		removeLock:     os.Remove,
		setRecord: func(ctx context.Context, tx dal.ReadwriteTransaction, r record.Record) error {
			return tx.Set(ctx, r)
		},
	}
}

// writeDemoFile writes name, creating its parent folders.
func writeDemoFile(name string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		return err
	}
	return os.WriteFile(name, data, 0o644)
}

// demoRepositoryEnvVars are the repository-local variables Git lists with
// `git rev-parse --local-env-vars`. Inherited from a hook, `rebase --exec` or
// a dotfile manager, they would point the install at another repository, so
// git runs without them (cli/demo#REQ:own-git-repository).
var demoRepositoryEnvVars = []string{
	"GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_CONFIG", "GIT_CONFIG_PARAMETERS", "GIT_CONFIG_COUNT",
	"GIT_OBJECT_DIRECTORY", "GIT_DIR", "GIT_WORK_TREE", "GIT_IMPLICIT_WORK_TREE", "GIT_GRAFT_FILE",
	"GIT_INDEX_FILE", "GIT_NO_REPLACE_OBJECTS", "GIT_REPLACE_REF_BASE", "GIT_PREFIX",
	"GIT_SHALLOW_FILE", "GIT_COMMON_DIR",
}

// withoutRepositoryEnv returns env without demoRepositoryEnvVars. Names are
// compared case-insensitively, as Windows does.
func withoutRepositoryEnv(env []string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		if slices.ContainsFunc(demoRepositoryEnvVars, func(v string) bool { return strings.EqualFold(v, name) }) {
			continue
		}
		out = append(out, kv)
	}
	return out
}

// runDemoGit runs git on the repository in dir, without inherited
// repository-local variables, and returns its trimmed stdout. A failure
// carries git's stderr.
func runDemoGit(ctx context.Context, git, dir string, env []string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, git, append([]string{"-C", dir}, args...)...)
	cmd.Dir = dir
	cmd.Env = withoutRepositoryEnv(env)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	out := strings.TrimSpace(stdout.String())
	if err != nil {
		return out, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

// demoTargetState is what the install found at the target folder.
type demoTargetState int

const (
	demoTargetMissing demoTargetState = iota
	demoTargetEmpty
	demoTargetInstalled
)

// demoAnotherFolderHint is the suggestion every refusal ends with.
const demoAnotherFolderHint = "choose another folder: ingitdb demo install --path=<another folder>"

// demoInProgressError reports a folder another install has claimed.
func demoInProgressError(dir string) error {
	return fmt.Errorf("another ingitdb demo install is in progress in folder %s; wait for it to finish, "+
		"or, if it was interrupted, delete the folder and install again", dir)
}

// inspectDemoTarget checks the target folder (cli/demo#REQ:refuses-conflicting-target).
// A folder holding the install lock is being installed: it is not reported as
// installed until the lock is gone, even when the marker is already there.
func inspectDemoTarget(dir string) (demoTargetState, error) {
	info, err := os.Stat(dir)
	if errors.Is(err, os.ErrNotExist) {
		if _, lstatErr := os.Lstat(dir); lstatErr == nil {
			return 0, fmt.Errorf("the path %s is a symbolic link to a folder that does not exist; %s", dir, demoAnotherFolderHint)
		}
		return demoTargetMissing, nil
	}
	if err != nil {
		return 0, fmt.Errorf("cannot use %s: %w", dir, err)
	}
	if !info.IsDir() {
		return 0, fmt.Errorf("the path %s is a file, not a folder; %s", dir, demoAnotherFolderHint)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, fmt.Errorf("cannot read %s: %w", dir, err)
	}
	if len(entries) == 0 {
		return demoTargetEmpty, nil
	}
	if _, lockErr := os.Lstat(filepath.Join(dir, demoLockFileName)); lockErr == nil {
		return 0, demoInProgressError(dir)
	}
	markerPath := filepath.Join(dir, config.IngitDBDirName, demoMarkerFileName)
	data, readErr := os.ReadFile(markerPath)
	var marker demoMarker
	if readErr != nil || yaml.Unmarshal(data, &marker) != nil || marker.App == "" {
		return 0, fmt.Errorf("folder %s is not empty and does not hold the TODO demo; %s", dir, demoAnotherFolderHint)
	}
	if marker.App != demoApp {
		return 0, fmt.Errorf("folder %s holds the %q demo, not the TODO demo; %s", dir, marker.App, demoAnotherFolderHint)
	}
	return demoTargetInstalled, nil
}

// install installs the TODO demo into dir, an absolute path. When dir
// already holds the TODO demo it writes nothing and reports that.
//
// The run claims the folder by creating the install lock exclusively inside
// it, so concurrent installs into one folder cannot interleave: exactly one
// installs, the others report it in progress or, once it is complete,
// installed. A failed run removes only what it created.
func (i demoInstaller) install(ctx context.Context, dir string) (demoResult, error) {
	state, err := inspectDemoTarget(dir)
	if err != nil {
		return demoResult{}, err
	}
	if state == demoTargetInstalled {
		return i.installedResult(ctx, dir), nil
	}
	created, err := i.createFolders(dir)
	if err != nil {
		removeCreatedDirs(created)
		return demoResult{}, fmt.Errorf("failed to install the TODO demo in %s: create the folder: %w", dir, err)
	}
	lock := filepath.Join(dir, demoLockFileName)
	if lockErr := i.createLock(lock); lockErr != nil {
		removeCreatedDirs(created)
		if !errors.Is(lockErr, os.ErrExist) {
			return demoResult{}, fmt.Errorf("failed to install the TODO demo in %s: claim the folder: %w", dir, lockErr)
		}
		// Another run claimed the folder first.
		state, err = inspectDemoTarget(dir)
		if err != nil {
			return demoResult{}, err
		}
		if state == demoTargetInstalled {
			return i.installedResult(ctx, dir), nil
		}
		return demoResult{}, demoInProgressError(dir)
	}
	result, err := i.write(ctx, dir)
	if err == nil {
		err = i.removeLock(lock)
	}
	if err != nil {
		return demoResult{}, i.failed(dir, err, created)
	}
	return result, nil
}

// createDemoLock creates the install lock, failing with os.ErrExist when another
// run holds it.
func createDemoLock(name string) error {
	f, err := os.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	return f.Close()
}

// createFolders creates the missing folders of dir, top down, and returns the
// ones this run created. A folder another process creates meanwhile is used,
// not claimed.
func (i demoInstaller) createFolders(dir string) ([]string, error) {
	var created []string
	for _, d := range missingDirs(dir) {
		err := i.mkdir(d, 0o755)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return created, err
		}
		created = append(created, d)
	}
	return created, nil
}

// missingDirs returns dir and its ancestors that do not exist, top down.
func missingDirs(dir string) []string {
	var missing []string
	for d := dir; !demoPathExists(d) && !slices.Contains(missing, d); d = filepath.Dir(d) {
		missing = append([]string{d}, missing...)
	}
	return missing
}

func demoPathExists(name string) bool {
	_, err := os.Lstat(name)
	return err == nil
}

// removeCreatedDirs removes, bottom up, the folders this run created that are
// empty; a folder holding anything, such as another process's files, stays.
func removeCreatedDirs(created []string) []error {
	var errs []error
	for idx := len(created) - 1; idx >= 0; idx-- {
		if entries, err := os.ReadDir(created[idx]); err != nil || len(entries) > 0 {
			continue
		}
		errs = append(errs, os.Remove(created[idx]))
	}
	return errs
}

// failed removes what the install wrote (cli/demo#REQ:no-partial-install):
// the contents of the claimed folder, then the lock, then the folders this
// run created. It returns the error to report.
func (i demoInstaller) failed(dir string, cause error, created []string) error {
	err := fmt.Errorf("failed to install the TODO demo in %s: %w", dir, cause)
	cleanupErrs := []error{removeDirContents(dir)}
	cleanupErrs = append(cleanupErrs, removeCreatedDirs(created)...)
	if cleanupErr := errors.Join(cleanupErrs...); cleanupErr != nil {
		return errors.Join(err, fmt.Errorf("failed to remove what was written: %w", cleanupErr))
	}
	return err
}

// removeDirContents empties the claimed folder dir, removing the install
// lock last so no other run claims the folder while it is being emptied.
func removeDirContents(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var errs []error
	for _, e := range entries {
		if e.Name() != demoLockFileName {
			errs = append(errs, os.RemoveAll(filepath.Join(dir, e.Name())))
		}
	}
	errs = append(errs, os.Remove(filepath.Join(dir, demoLockFileName)))
	return errors.Join(errs...)
}

// write fills the (new or empty) folder dir: git init, configuration files,
// records, and the install commit.
func (i demoInstaller) write(ctx context.Context, dir string) (demoResult, error) {
	result := newDemoResult(dir)
	git, lookErr := i.lookPath("git")
	hasGit := lookErr == nil
	// git init comes before any write so no write lands in an enclosing
	// repository (cli/demo#REQ:own-git-repository).
	if hasGit {
		if _, err := i.runGit(ctx, git, dir, i.env, "init", "-q"); err != nil {
			return result, err
		}
	}
	for _, f := range demoConfigFiles() {
		if err := i.writeYAMLFile(dir, f); err != nil {
			return result, err
		}
	}
	if err := i.writeRecords(ctx, dir); err != nil {
		return result, err
	}
	// The marker is written last, after every record.
	if err := i.writeYAMLFile(dir, demoMarkerFile()); err != nil {
		return result, err
	}
	if !hasGit {
		return result, nil
	}
	commit, err := i.commit(ctx, git, dir)
	if err != nil {
		return result, err
	}
	result.Git = demoGitResult{Repository: true, Commit: commit}
	return result, nil
}

func (i demoInstaller) writeYAMLFile(dir string, f demoFile) error {
	name := filepath.Join(dir, filepath.FromSlash(f.path))
	data, _ := yaml.Marshal(f.value)
	if err := i.writeFile(name, data); err != nil {
		return fmt.Errorf("write %s: %w", f.path, err)
	}
	return nil
}

// writeRecords writes the shared package's records through the local driver
// at the driver's own on-disk paths.
func (i demoInstaller) writeRecords(ctx context.Context, dir string) error {
	def, err := i.readDefinition(dir)
	if err != nil {
		return fmt.Errorf("read the demo definitions: %w", err)
	}
	db, err := i.newDB(dir, def)
	if err != nil {
		return fmt.Errorf("open the demo database: %w", err)
	}
	records := todo.Records(i.now())
	return db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error {
		for _, r := range records {
			rec := record.NewRecordWithData(demoRecordKey(r.Key), r.Data)
			if err := i.setRecord(ctx, tx, rec); err != nil {
				return fmt.Errorf("write record %s: %w", r.Key, err)
			}
		}
		return nil
	})
}

// demoRecordKey turns a shared-package key such as lists/to-buy/items/milk
// into a record key with its parent chain.
func demoRecordKey(path string) *record.Key {
	segments := strings.Split(path, "/")
	var key *record.Key
	for j := 0; j+1 < len(segments); j += 2 {
		key = record.NewKeyWithParentAndID(key, segments[j], segments[j+1])
	}
	return key
}

// commit stages everything and makes the install commit, filling a missing
// user.name or user.email from inGitDB <ingitdb@localhost> for this commit
// only (cli/demo#REQ:git-identity). It returns the commit id.
func (i demoInstaller) commit(ctx context.Context, git, dir string) (string, error) {
	env := slices.Clone(i.env)
	if name, _ := i.runGit(ctx, git, dir, i.env, "config", "user.name"); name == "" {
		env = append(env, "GIT_AUTHOR_NAME="+demoDefaultGitName, "GIT_COMMITTER_NAME="+demoDefaultGitName)
	}
	if email, _ := i.runGit(ctx, git, dir, i.env, "config", "user.email"); email == "" {
		env = append(env, "GIT_AUTHOR_EMAIL="+demoDefaultGitEmail, "GIT_COMMITTER_EMAIL="+demoDefaultGitEmail)
	}
	if _, err := i.runGit(ctx, git, dir, env, "add", "-A", "--", ".", ":(exclude)"+demoLockFileName); err != nil {
		return "", err
	}
	if _, err := i.runGit(ctx, git, dir, env, "commit", "-q", "-m", demoCommitMessage); err != nil {
		return "", fmt.Errorf("%w\n%s", err, demoCommitRefusedHint)
	}
	return i.runGit(ctx, git, dir, env, "rev-parse", "HEAD")
}

// installedResult reports an existing TODO demo without writing anything
// (cli/demo#REQ:idempotent-reinstall).
func (i demoInstaller) installedResult(ctx context.Context, dir string) demoResult {
	result := newDemoResult(dir)
	result.AlreadyInstalled = true
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		return result
	}
	result.Git.Repository = true
	git, lookErr := i.lookPath("git")
	if lookErr != nil {
		return result
	}
	if commit, err := i.runGit(ctx, git, dir, i.env, "rev-parse", "HEAD"); err == nil {
		result.Git.Commit = commit
	}
	return result
}

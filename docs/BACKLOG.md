# 📝 Development Backlog

The backlog is organized by phase. See [ROADMAP.md](ROADMAP.md) for the big picture.

Tasks within each phase are ordered by dependency — implement them top to bottom.

---

## 🧩 Phase 1: Validator + Materialized Views

### 🖥️ P1-1: Migrate CLI to subcommand-based interface

**What:** Replace the positional argument in `cmd/ingitdb/main.go` with a subcommand dispatcher using `github.com/urfave/cli/v3`. For this phase, implement the `validate` subcommand with a `--path` flag.

**Why:** The current `args[1]` positional approach is a temporary placeholder. The intended interface is subcommand-based (`ingitdb validate --path=PATH`), which is the standard Go CLI pattern and matches `docs/cli/README.md`.

**Acceptance criteria:**

- `ingitdb validate --path=/path/to/db` validates and exits 0 on success, 1 on failure
- `ingitdb validate` with no `--path` validates the current working directory
- `ingitdb --version` prints version string (unchanged behavior)
- `ingitdb` with no subcommand prints usage to stderr and exits 1
- Unknown subcommand prints usage to stderr and exits 1
- `~` in `--path` values is still expanded to the home directory
- All existing tests updated; new tests cover subcommand routing and flag parsing

**Implementation notes:**

- Use `github.com/urfave/cli/v3` for subcommand and flag parsing
- Keep `run()` dependency-injected (current signature pattern must be preserved for testability)
- Usage text must go to `os.Stderr`

---

### 🔹 P1-2: Implement data validation

**What:** After loading collection schemas, walk each collection's `data_dir` and validate every record file against its schema.

**Why:** The validator currently only validates schema files (`.definition.yaml`). Actual record data is never checked.

**Acceptance criteria:**

- Detects and reports all of the following violations:
  - Missing required fields
  - Type mismatches (e.g. string field contains a number)
  - String values exceeding `max_length`
  - `map[locale]string` fields missing values for required languages (from `.ingitdb.yaml`)
  - `foreign_key` values that do not match any record ID in the referenced collection
- Each error includes: collection ID, record file path, field name, violation description
- All violations collected before exiting — does not stop at the first error
- Exit 0 if no violations; exit 1 if any violations found

**Implementation notes:**

- Add a `DataValidator` in `pkg/ingitdb/validator/` (separate file from `def_validator.go`)
- Use `record_file.format` to choose the parser (JSON or YAML) and `record_file.type` to handle single-record (`map[string]any`) vs. multi-record (`[]map[string]any`) files
- Language validation requires the root config's `languages` list — pass it through from `ReadDefinition`
- Foreign key validation requires all collection definitions to be loaded first; this is already satisfied since `ReadDefinition` loads all collections before returning
- No nested calls: assign intermediate results to named variables

---

### 🧾 P1-3: Implement materialized views builder

**What:** After successful validation, read each collection's view definitions (`.collection/views/<name>.yaml`) and generate the corresponding output files under `$views/`.

**Why:** Materialized views are precomputed outputs derived from the same records the validator has just read. Rebuilding them in the same pass avoids a second full scan.

**Acceptance criteria:**

- For each `.collection/views/<name>.yaml` in a collection directory, the corresponding `$views/<name>/` output is created or updated
- Output files respect `order_by`, `columns`, and `formats` defined in the view definition
- Views are only rebuilt for collections that passed validation — invalid collections are skipped
- `ingitdb validate` rebuilds all views by default; a future `--no-materialize` flag may opt out (not required in this task)
- Materialized view output files for records no longer in the collection are removed

**Implementation notes:**

- Add a `ViewsBuilder` in a new package `pkg/ingitdb/views/` (separate from the validator)
- The validator passes its loaded record data to the views builder — the builder must not re-read files from disk
- View partitioning: the view name pattern (e.g. `status_{.status}`) determines the output filename per partition; each unique value of the partitioned field produces one output file

---

### 🔹 P1-4: Implement change validation mode

**What:** Add `--from-commit` and `--to-commit` flags to the `validate` subcommand. When provided, validate only the record files changed between those two commits and rematerialize only the affected views.

**Why:** Full validation of a large database on every CI push is too slow. Change validation makes inGitDB-backed CI pipelines practical at scale.

**Acceptance criteria:**

- `ingitdb validate --path=PATH --from-commit=SHA1 --to-commit=SHA2` validates only changed records and rebuilds only affected views
- Schema config files (`.ingitdb.yaml`, `.definition.yaml`) are always fully re-validated regardless of the commit range
- `--from-commit` without `--to-commit` defaults to HEAD as the "to" commit
- If `git diff` fails (not a git repo, bad SHA, git not installed), error is reported clearly and process exits with code 2 (infrastructure error, distinct from validation failure)

**Implementation notes:**

- Use `os/exec` to run `git diff --name-only FROM TO` and capture the output
- Filter paths to only those under the DB root path
- Pass the filtered file set into both the data validator and the views builder

---

### 🔹 P1-5: Structured error reporting and exit codes

**What:** Standardize error output format and exit codes across all validation paths.

**Why:** The validator is used in CI pipelines. Consistent exit codes and output format are essential for scripting and readable CI logs.

**Acceptance criteria:**

- Exit code 0: validation passed and views rebuilt successfully
- Exit code 1: one or more validation errors in the data or schema
- Exit code 2: infrastructure/runtime error (config file unreadable, git failure, bad flag, etc.)
- All validation errors printed to `os.Stderr`, one per line:
  `<collection-id>/<record-file>: <field>: <message>`
- Summary line printed after all errors:
  - `Validation passed.` on success
  - `Validation failed: N error(s) found.` on failure
- Runtime errors clearly distinguished from validation failures in output

**Implementation notes:**

- Introduce an error collector (e.g. `[]error` slice or a dedicated type) in the validator to accumulate all violations before returning
- Separate validator errors (data/schema issues) from infrastructure errors (I/O, git, parsing) so the CLI can assign the correct exit code

---

## 📂 Phase 3: Git Merge Conflict Resolution

### 🖥️ P3-1: Implement pull command

**What:** Add `ingitdb pull` — a single command that pulls the latest git changes, resolves conflicts, rebuilds views, and prints a change summary.

**Why:** Running `git pull` followed by manual conflict resolution, `ingitdb resolve`, and `ingitdb materialize` is error-prone and tedious. `ingitdb pull` automates the full cycle.

**Acceptance criteria:**

- `ingitdb pull [--path=PATH] [--strategy=rebase|merge] [--remote=REMOTE] [--branch=BRANCH]` executes the pull cycle end-to-end
- Default strategy is `rebase`; `--strategy=merge` switches to `git pull --no-rebase`
- `--remote` defaults to `origin`; `--branch` defaults to the current branch's configured tracking branch
- Generated file conflicts (`$views/**`, `README.md`) are resolved silently by regeneration
- Data file conflicts open the TUI resolver one file at a time (reuses the resolver from `ingitdb resolve`)
- Materialized views and `README.md` are rebuilt after all conflicts are resolved
- A human-readable change summary is printed to stdout listing records added, updated, and deleted by the pull
- Exit codes: `0` success, `1` unresolved conflicts remain, `2` infrastructure error (git failure, network, bad flags)

**Change summary format:**

```
Pulled 3 commits from origin/main (rebase)

  Records added:   2
    + /countries/de/cities/berlin
    + /countries/fr/cities/paris

  Records updated: 1
    ~ /countries/gb/cities/london  (2 fields: population, area)

  Records deleted: 0
```

**Implementation notes:**

- Run `git pull [--rebase|--no-rebase] <remote> <branch>` via `os/exec`; capture stderr for error messages
- After pull, run `git status --porcelain` to detect conflicted files; delegate to the conflict resolver (Phase 3 `resolve` implementation)
- Collect the set of changed files from `git diff --name-only ORIG_HEAD HEAD` after a successful pull to build the summary
- Reuse the Views Builder (Phase 1) for regenerating views post-pull
- Summary goes to stdout; all diagnostic messages (progress, errors) go to stderr

---

## 🧩 Phase 4: Watcher

### 🖥️ P4-1: Implement watch command

**What:** Implement `ingitdb watch` to monitor an inGitDB directory for file-system changes and stream structured record events to stdout.

**Why:** Developers and tooling need real-time visibility into which records change and how, without polling or running `validate` repeatedly.

**Acceptance criteria:**

- `ingitdb watch [--path=PATH] [--format=text|json]` runs in the foreground and exits cleanly on SIGINT/SIGTERM
- One event per line written to stdout as it occurs
- Text format:
  ```
  Record /countries/gb/cities/london: added
  Record /countries/gb/cities/london: 2 fields updated: {population: 9000000, area: 1572}
  Record /countries/gb/cities/london: deleted
  ```
- JSON format (`--format=json`):
  ```json
  {"type":"added","record":"/countries/gb/cities/london"}
  {"type":"updated","record":"/countries/gb/cities/london","fields":{"population":9000000,"area":1572}}
  {"type":"deleted","record":"/countries/gb/cities/london"}
  ```
- `--path` defaults to current working directory; `~` is expanded
- Non-record file changes (schema files, view definitions) are ignored
- Startup errors (invalid path, unreadable config) exit with code 2

**Implementation notes:**

- Use `fsnotify` (or equivalent) for OS-level file-system events
- Load `Definition` on startup via `validator.ReadDefinition`; use it to map changed paths back to collection + record ID
- Compare old and new file content to produce the `fields` map for `updated` events (read file before and after the write event)
- Implement the `Watcher` interface defined in [component doc](components/watcher.md)
- `ingitdb serve --watcher` should reuse the same `Watcher` implementation

---

## 📂 Phase 2: Query

### 🖥️ P2-1: Implement query subcommand

**What:** Add a `query` subcommand that reads and returns records from a specified collection.

**Acceptance criteria:**

- `ingitdb query --collection=<key> [--path=PATH] [--format=CSV|JSON|YAML]` returns records to stdout
- Default format is JSON
- Unknown collection key prints an error to stderr and exits 1
- `--path` defaults to current working directory and supports `~` expansion

**Implementation notes:**

- Output goes to `os.Stdout` (this is the one case where stdout is used — query results are data, not diagnostics)
- Reuse `validator.ReadDefinition` for schema loading; add a separate reader for record data

---

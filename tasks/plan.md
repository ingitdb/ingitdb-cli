# Implementation Plan: dalgo2ingitdb-dbschema-ddl-coverage

## Overview

Add `dbschema.SchemaReader`, `ddl.SchemaModifier`, `ddl.TransactionalDDL`, and `dal.ConcurrencyAware`
support to the `pkg/dalgo2ingitdb` package by introducing a `Database` struct returned by a new
`NewDatabase` constructor. Schema reads walk `definition.yaml` files on disk; DDL writes/rewrites
them, plus record files for field-level alter ops.

## Architecture Decisions

- **Cross-platform file locking via `github.com/gofrs/flock`** — single `filelock.go` file
  works on macOS, Linux, and Windows. The library wraps `syscall.Flock` on Unix and
  `LockFileEx` on Windows behind one API. `Database` embeds `dal.ConcurrencyAvailable` so
  `SupportsConcurrentConnections()` returns `true` on all platforms.
- **File locking helper** — `filelock.go` exposes:
  - `withSharedLock(path string, fn func() error) error` — `flock.New(path).RLock()` → run → unlock
  - `withExclusiveLock(path string, fn func() error) error` — `flock.New(path).Lock()` → run → unlock
  Lock granularity is per-file: `definition.yaml` and each record file are locked independently,
  so a read of one collection's schema proceeds while another collection's records are rewritten.
- **Stub record-access methods** — `RunReadonlyTransaction`, `RunReadwriteTransaction`, `Get`,
  `Exists`, `GetMulti`, `ExecuteQueryToRecordsReader`, `ExecuteQueryToRecordsetReader` all return
  `dal.ErrNotSupported`. This feature is about schema management, not record access.
- **`go.mod` replace directive** — Published `github.com/dal-go/dalgo v0.41.15` lacks `dbschema`,
  `ddl`, and `dal.ConcurrencyAware`. A `replace` directive pointing to `../../dal-go/dalgo` is
  required for development. Must be removed when dalgo publishes the new version.
- **Flat walker for `ListCollections`** — Uses `filepath.WalkDir` directly rather than
  `validator.ReadDefinition`, which has config-file dependencies. Simpler; no dependency on
  `.ingitdb.yaml`.
- **`ConstraintDef.Type` is a plain string** — `dbschema.ConstraintDef` has no `Fields` field and
  no exported type constants. Use `Type: "primary-key"` and `Name: "$key-pk"` per the string
  documented in `constraint.go`.
- **`Applier` pattern for `AlterCollection`** — `Database` implements `ddl.Applier`, iterating ops
  with `op.ApplyTo(ctx, applier)` and catching partial-success failures.

## Dependency Graph

```
Task 1: go.mod replace directive   (prerequisite — nothing compiles without this)
    │
Task 2: type_mapping.go            (no deps, pure translation table)
    │
Task 3: filelock.go                (no deps, syscall.Flock wrapper)
    │
Task 4: database.go                (deps: Tasks 2–3, ConcurrencyAvailable embed)
    │
Task 5: schema_reader.go           (deps: Tasks 3–4; reads use shared lock)
    │
Task 6: schema_modifier.go         (deps: Tasks 3–5; writes use exclusive lock)
    │
Task 7: integration test           (deps: Tasks 4–6 all green)
```

## Task List

### Phase 1: Foundation
- [ ] Task 1: Add `replace` directive for local dalgo
- [ ] Task 2: Implement bidirectional type mapping
- [ ] Task 3: File locking helper (`filelock.go`)

### Checkpoint: Foundation
- [ ] `go build ./pkg/dalgo2ingitdb/...` succeeds
- [ ] `go test -run 'TestTypeMapping|TestFileLock' ./pkg/dalgo2ingitdb/` passes

### Phase 2: Core Interfaces
- [ ] Task 4: `Database` struct + `NewDatabase` + `dal.DB` stubs + `SupportsTransactionalDDL`
- [ ] Task 5: `SchemaReader` — all five methods (reads use shared lock on `definition.yaml`)

### Checkpoint: Read Path
- [ ] `go test ./pkg/dalgo2ingitdb/...` Tasks 4–5 pass
- [ ] Compile-time type assertions in `database.go` hold

### Phase 3: Write Path
- [ ] Task 6: `SchemaModifier` — `CreateCollection`, `DropCollection`, `AlterCollection` + `Applier` (writes use exclusive lock)

### Checkpoint: Write Path
- [ ] `go test ./pkg/dalgo2ingitdb/...` all modifier tests pass
- [ ] `go vet ./pkg/dalgo2ingitdb/...` clean

### Phase 4: Integration
- [ ] Task 7: End-to-end lifecycle integration test (includes concurrent-access assertion)

### Checkpoint: Complete
- [ ] `go test -timeout=30s ./...` passes
- [ ] `golangci-lint run` clean
- [ ] All spec ACs verified

---

## Task 1: Add `replace` directive for local dalgo

**Description:** Wire `go.mod` to the local dalgo repo so the new `dbschema`, `ddl`, and
`dal.ConcurrencyAware` symbols resolve.

**Acceptance criteria:**
- [ ] `go.mod` has `replace github.com/dal-go/dalgo => ../../dal-go/dalgo`
- [ ] `go build ./...` succeeds with no import errors
- [ ] Existing tests still pass

**Verification:** `go build ./...` exits 0; `go test -timeout=10s ./...` exits 0

**Dependencies:** None

**Files:** `go.mod`, `go.sum`

**Estimated scope:** XS

---

## Task 2: Bidirectional type mapping

**Description:** `pkg/dalgo2ingitdb/type_mapping.go` with two functions:
`ingitdbTypeToDBSchema(ingitdb.ColumnType) (dbschema.Type, error)` and
`dbschemaTypeToIngitdb(dbschema.Type) (ingitdb.ColumnType, error)`. Covers all 9 ingitdb column
types plus map variants. Table-driven unit tests with round-trip coverage.

**Mapping table:**
| `ingitdb.ColumnType` | `dbschema.Type` |
|---|---|
| `string` | `String` |
| `int` | `Int` |
| `float` | `Float` |
| `bool` | `Bool` |
| `date`, `time`, `datetime` | `Time` |
| `any`, `map[locale]string`, `map[*]string` / other map variants | `String` |

Round-trip note: `dbschema.Time` → `datetime`; `dbschema.Null` → error.

**Acceptance criteria:**
- [ ] All 9 column types map correctly
- [ ] `dbschema.Null` returns error
- [ ] `dbschema.Time` → `ingitdb.ColumnTypeDateTime`
- [ ] map variants → `dbschema.String`
- [ ] unknown `dbschema.Type` returns error naming the value

**Verification:** `go test -run TestTypeMapping ./pkg/dalgo2ingitdb/` passes

**Dependencies:** Task 1

**Files:** `pkg/dalgo2ingitdb/type_mapping.go` (new), `pkg/dalgo2ingitdb/type_mapping_test.go` (new)

**Estimated scope:** S

---

## Task 3: File locking helper (cross-platform via `gofrs/flock`)

**Description:** `pkg/dalgo2ingitdb/filelock.go` with two helpers wrapping
`github.com/gofrs/flock`:
- `withSharedLock(path string, fn func() error) error` — `flock.New(path).RLock()` → call `fn` → `Unlock()`. Uses defer to guarantee unlock on `fn` error/panic.
- `withExclusiveLock(path string, fn func() error) error` — `flock.New(path).Lock()` → call `fn` → `Unlock()`. Same defer pattern.

`gofrs/flock` is a thin wrapper: `syscall.Flock` on Unix, `LockFileEx` on Windows. Lock files
are the actual target files (e.g. `definition.yaml`); no sidecar `.lock` files are created.

Add `github.com/gofrs/flock` to `go.mod` (run `go get github.com/gofrs/flock`).

**Acceptance criteria:**
- [ ] `withExclusiveLock` blocks a second goroutine from acquiring an exclusive lock on the same file (test via channel ordering)
- [ ] `withSharedLock` allows two goroutines to hold simultaneous shared locks
- [ ] `withExclusiveLock` blocks while a shared lock is held (and vice versa)
- [ ] Lock is always released even when `fn` returns an error
- [ ] Tests pass on the developer's macOS box (cross-platform tested in CI later if Windows CI exists)

**Verification:** `go test -run TestFileLock ./pkg/dalgo2ingitdb/` — goroutine-ordering tests via channels

**Dependencies:** Task 1

**Files:** `pkg/dalgo2ingitdb/filelock.go` (new), `pkg/dalgo2ingitdb/filelock_test.go` (new), `go.mod`/`go.sum` (add `github.com/gofrs/flock`)

**Estimated scope:** S

---

## Task 4: `Database` struct + `NewDatabase` + `dal.DB` stubs

**Description:** `pkg/dalgo2ingitdb/database.go` with the `Database` struct embedding
`dal.ConcurrencyAvailable` (returns `true` on all platforms — safe because all reads/writes use
`gofrs/flock`-based file locking), the `NewDatabase(projectPath string, reader ingitdb.CollectionsReader) (dal.DB, error)`
constructor, all `dal.DB` interface method stubs, and `SupportsTransactionalDDL() bool`.

Compile-time interface checks (`var _ dbschema.SchemaReader = (*Database)(nil)` etc.) go in the
same file.

**Acceptance criteria:**
- [ ] `NewDatabase("./valid-path", reader)` returns `(db, nil)` when path exists
- [ ] `NewDatabase("", reader)` returns `(nil, err)` — empty path rejected before `os.Stat`
- [ ] `NewDatabase("./missing", reader)` returns `(nil, err)` — `os.Stat` error propagated
- [ ] `db.(dbschema.SchemaReader)` holds at compile time
- [ ] `db.(ddl.SchemaModifier)` holds at compile time
- [ ] `db.(ddl.TransactionalDDL)` holds at compile time
- [ ] `db.SupportsConcurrentConnections()` returns `true`
- [ ] `ddl.SupportsTransactionalDDL(db)` returns `false`

**Verification:** `go build ./pkg/dalgo2ingitdb/` and `go test -run TestDatabase ./pkg/dalgo2ingitdb/`

**Dependencies:** Tasks 2–3

**Files:** `pkg/dalgo2ingitdb/database.go` (new), `pkg/dalgo2ingitdb/database_test.go` (new)

**Estimated scope:** M

---

## Task 5: `SchemaReader` — all five methods

**Description:** `pkg/dalgo2ingitdb/schema_reader.go` implementing the five `dbschema.SchemaReader`
methods on `*Database`.

- `ListCollections`: `filepath.WalkDir` looking for `<dir>/.collection/definition.yaml`; skips
  `.collection/`, `$records/`, `subcollections/`, `views/` entries; returns sorted
  `[]dal.CollectionRef` using relative paths with `/` separator. No lock needed (directory listing
  is inherently safe; definition.yaml existence is checked, not read).
- `DescribeCollection`: acquires shared lock on `definition.yaml`, reads and parses it, maps
  columns via type mapping, builds `*dbschema.CollectionDef` with `PrimaryKey: ["$key"]`.
  Respects `ColumnsOrder`; falls back to sorted column names when absent.
- `ListIndexes`: returns `([]dbschema.IndexDef{}, nil)`.
- `ListConstraints`: returns `([]dbschema.ConstraintDef{{Name: "$key-pk", Type: "primary-key"}}, nil)`.
- `ListReferrers`: returns `(nil, &dbschema.NotSupportedError{Op: "ListReferrers", Backend: "ingitdb", …})`.

**Acceptance criteria:**
- [ ] `ListCollections` finds dirs with `.collection/definition.yaml`, ignores dirs without
- [ ] `ListCollections` skips reserved sub-entries (`.collection`, `$records`, etc.)
- [ ] `ListCollections` result is alphabetically sorted
- [ ] `ListCollections(ctx, nonNilKey)` behaves same as `ListCollections(ctx, nil)`
- [ ] `DescribeCollection` returns `Name`, `PrimaryKey == ["$key"]`, correct `Fields`
- [ ] `DescribeCollection` `Required: true` → `Nullable: false`; absent Required → `Nullable: true`
- [ ] `DescribeCollection` field `"$key"` NOT included in `Fields`
- [ ] `DescribeCollection` missing collection returns err containing "not found" and collection name
- [ ] `DescribeCollection` holds shared lock while reading (verified by concurrent test)
- [ ] `ListIndexes` returns non-nil empty slice, nil error
- [ ] `ListConstraints` returns single `{Name: "$key-pk", Type: "primary-key"}` and nil error
- [ ] `ListReferrers` returns `(nil, *dbschema.NotSupportedError{Op: "ListReferrers"})`

**Verification:** `go test -run TestSchemaReader ./pkg/dalgo2ingitdb/` — tests use `t.TempDir()` + YAML fixtures

**Dependencies:** Tasks 3–4

**Files:** `pkg/dalgo2ingitdb/schema_reader.go` (new), `pkg/dalgo2ingitdb/schema_reader_test.go` (new)

**Estimated scope:** M

---

## Task 6: `SchemaModifier` + `Applier`

**Description:** `pkg/dalgo2ingitdb/schema_modifier.go` implementing `ddl.SchemaModifier` on
`*Database` plus `ddl.Applier` for `AlterCollection` dispatch. All writes to `definition.yaml`
and record files go through `withExclusiveLock`.

`CreateCollection`:
1. Validate name (non-empty, no `..` path traversal, no leading/trailing whitespace)
2. Validate all field types via type mapping — error before any write if any field uses `dbschema.Null`
3. `withExclusiveLock(definition.yaml path, func)`:
   - Check for existing `definition.yaml` — if exists + `IfNotExists` set → return nil; if exists + not set → error
   - `os.MkdirAll` the `.collection/` dir
   - Marshal `ingitdb.CollectionDef` and write `definition.yaml` with default `RecordFile: {name: "{key}.yaml", format: yaml, type: "map[string]any"}`
4. Log warning + skip if `c.Indexes` non-empty

`DropCollection`:
1. Validate name
2. `withExclusiveLock(definition.yaml path, func)`:
   - Check if `definition.yaml` exists — absent + `IfExists` set → nil; absent + not set → error
   - `os.RemoveAll(<projectPath>/<name>)` (lock file is inside; lock released after RemoveAll)

`AlterCollection` + `Applier`:
1. `withExclusiveLock(definition.yaml)` wraps the entire alter sequence
2. Load `definition.yaml` into `ingitdb.CollectionDef`
3. Iterate ops via `op.ApplyTo(ctx, applier)`
4. After each op, write updated `definition.yaml` back to disk
5. On op failure, return `*ddl.PartialSuccessError` with applied/failed/not-attempted

Applier methods:
- `ApplyAddField`: append column; if `f.Default` set, rewrite each record file under `withExclusiveLock(recordFile)`; IfNotExists → skip if field exists
- `ApplyDropField`: remove from definition; rewrite each record file under `withExclusiveLock(recordFile)`; IfExists → skip if absent
- `ApplyRenameField`: rename in definition; rewrite each record file under `withExclusiveLock(recordFile)`
- `ApplyModifyField`: update type in definition; no record file rewrite
- `ApplyAddIndex`/`ApplyDropIndex`: `log.Printf` warning, return nil

**Acceptance criteria:**
- [ ] `CreateCollection` writes correct `definition.yaml` under exclusive lock
- [ ] `CreateCollection` with `IfNotExists()` is a no-op when collection exists
- [ ] `CreateCollection` rejects `dbschema.Null` field — no filesystem writes
- [ ] `CreateCollection` rejects empty name and `..` path traversal
- [ ] `DropCollection` removes entire collection dir tree
- [ ] `DropCollection` with `IfExists()` returns nil for absent collection
- [ ] `DropCollection` returns error (no removal) when target has no `definition.yaml`
- [ ] `AddField` adds to definition and `columns_order`; with Default, record files updated
- [ ] `DropField` removes from definition; record files updated
- [ ] `RenameField` renames in definition + record files
- [ ] `ModifyField` updates definition type; does NOT rewrite record files for pure type change
- [ ] `AddIndex`/`DropIndex` log warning, return nil (no error)
- [ ] Partial failure returns `*ddl.PartialSuccessError` with correct applied/failed counts

**Verification:** `go test -run TestSchemaModifier ./pkg/dalgo2ingitdb/` — uses `t.TempDir()` with real YAML files

**Dependencies:** Tasks 4–5

**Files:** `pkg/dalgo2ingitdb/schema_modifier.go` (new), `pkg/dalgo2ingitdb/schema_modifier_test.go` (new)

**Estimated scope:** L

---

## Task 7: End-to-end integration test

**Description:** External test package `dalgo2ingitdb_test` in `pkg/dalgo2ingitdb/integration_test.go`.
Exercises the full lifecycle against a real `t.TempDir()`. Includes a concurrent-access sub-test
that runs two goroutines performing simultaneous schema reads to confirm no deadlock and correct
results. No git daemon required.

Covers: `NewDatabase → CreateCollection(events, 4 fields) → ListCollections → DescribeCollection →
AlterCollection(AddField color) → DescribeCollection → AlterCollection(DropField color) →
AlterCollection(two-op partial-failure) → DropCollection → ListCollections`.

Concurrent sub-test: two goroutines each call `DescribeCollection` on the same collection
simultaneously; both must return correct results.

**Acceptance criteria:**
- [ ] `events` collection created with correct fields; round-trip via `DescribeCollection` matches
- [ ] `AlterCollection` add then describe shows new field
- [ ] `AlterCollection` drop then describe shows field removed
- [ ] `DropCollection` removes collection; `ListCollections` no longer includes it
- [ ] Two-op `AlterCollection` (valid then invalid) returns `*ddl.PartialSuccessError`; first op applied
- [ ] Concurrent `DescribeCollection` from two goroutines produces correct results with no deadlock
- [ ] `db.SupportsConcurrentConnections()` returns `true`

**Verification:** `go test -run TestIntegration ./pkg/dalgo2ingitdb/` and `go test -timeout=30s ./...`

**Dependencies:** Tasks 4–6

**Files:** `pkg/dalgo2ingitdb/integration_test.go` (new)

**Estimated scope:** M

---

## Risks and Mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| `replace` directive left in go.mod after dalgo publishes | Medium | Add comment; track as follow-up |
| `dal.DB` interface has changed methods in local dalgo | High — compile failure | Check `dal/db_database.go` before implementing stubs |
| New dependency `github.com/gofrs/flock` | Low — well-maintained, single-purpose, zero transitive deps | Pin via `go.sum`; vetted library used in many Go projects |
| `ColumnsOrder` absent in some `definition.yaml` | Medium — wrong ordering | Fall back to sorted `maps.Keys(colDef.Columns)` |
| `DropCollection` removes lock file mid-lock | Low — flock held on open fd, not path | `syscall.Flock` operates on the fd; removing the file path does not release the fd lock |
| Record file rewrite slow for large collections | Low for MVP | Documented; streaming rewrite deferred |

## Outstanding Questions

1. **`ConstraintDef` shape**: `dbschema.ConstraintDef` has only `Name` and `Type` — no
   `Fields []dal.FieldName`. Spec AC says `Fields == ["$key"]` but the type doesn't support it.
   Plan: use `Name: "$key-pk", Type: "primary-key"`. Gap between spec and actual type.
2. **Full `dal.DB` record access**: Record-access methods return `ErrNotSupported` for MVP.
   Assumption confirmed — feature scope is schema management; record access is a follow-up.

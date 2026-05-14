# Feature: dbschema + ddl + ConcurrencyAware Coverage for inGitDB

**Status:** Draft
**Source Idea:** —
**Date:** 2026-05-13
**Owner:** alex

## Summary

`dalgo2ingitdb` is the DALgo driver for the inGitDB project format — a git repository where each collection is a directory of files (yaml/json/markdown/toml/ingr/csv) with schema declared in `.collection/definition.yaml`. Today the package provides `dal.DB` read/write for records (Get, Set, Delete, queries) via `CollectionForKey` and format-aware parsers, but does NOT implement the schema-management capability interfaces shipped in `dal-go/dalgo`.

This Feature adds three new capabilities to the existing `dalgo2ingitdb` package:

- **`dbschema.SchemaReader`** — schema introspection by reading `.collection/definition.yaml` files on disk and walking the project directory tree
- **`ddl.SchemaModifier`** — filesystem-level DDL: writing new `definition.yaml` files, removing collection directories, rewriting field definitions
- **`dal.ConcurrencyAware`** — advertises `SupportsConcurrentConnections() = false` (inGitDB writes to a git working tree, which is single-writer; concurrent writes would corrupt the working tree)

A new `dalgo2ingitdb.NewDatabase(projectPath string, reader ingitdb.CollectionsReader) (dal.DB, error)` constructor (matching the directory-driven model the package already uses) returns a `dal.DB` that satisfies all four capability interfaces. The `ingitdb.CollectionsReader` argument follows the pattern the existing code uses for loading `Definition` from disk.

This Feature is the filesystem half of the cross-engine dbschema/ddl coverage. It is the inGitDB analogue of the `dalgo2sqlite` dbschema-ddl-coverage Feature but differs fundamentally: there is no SQL, no type-affinity mapping, no transactions — schema changes are directory and file operations on a git working tree.

## Synopsis

```go
import (
    "github.com/dal-go/dalgo/dal"
    "github.com/dal-go/dalgo/dbschema"
    "github.com/dal-go/dalgo/ddl"
    "github.com/ingitdb/ingitdb-cli/pkg/dalgo2ingitdb"
    "github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

db, err := dalgo2ingitdb.NewDatabase("./my-project", validator.NewReader())
// err handling…

// dal.DB surface (existing — read/write records)
_ = db.RunReadwriteTransaction(ctx, func(ctx context.Context, tx dal.ReadwriteTransaction) error { /*…*/ })

// dbschema introspection
reader := db.(dbschema.SchemaReader)
collections, _ := reader.ListCollections(ctx, nil)
def, _ := reader.DescribeCollection(ctx, &dal.CollectionRef{Name: "countries"})

// ddl
_ = ddl.CreateCollection(ctx, db, dbschema.CollectionDef{
    Name:   "tags",
    Fields: []dbschema.FieldDef{{Name: "label", Type: dbschema.String}},
})
_ = ddl.AlterCollection(ctx, db, "tags", ddl.AddField(dbschema.FieldDef{Name: "color", Type: dbschema.String, Nullable: true}))
_ = ddl.DropCollection(ctx, db, "tags")

// concurrency hint
parallel := db.(dal.ConcurrencyAware).SupportsConcurrentConnections() // false
```

## Problem

The `dal-go/dalgo` package ships `dbschema.SchemaReader`, `ddl.SchemaModifier`, and `dal.ConcurrencyAware` as engine-agnostic interfaces. Consumers (e.g. `datatug-cli`'s `db copy`) need drivers to implement these interfaces so cross-engine workflows — listing collections from one engine, recreating them in another — are testable end-to-end. The inGitDB driver `dalgo2ingitdb` currently only covers `dal.DB` read/write; it cannot answer basic questions like "what collections exist?" or "what fields does this collection have?" via the standard dalgo interfaces. This Feature closes that gap.

## Behavior

### Construction

#### REQ: new-database-constructor

The package MUST export `dalgo2ingitdb.NewDatabase(projectPath string, reader ingitdb.CollectionsReader) (dal.DB, error)`. The constructor records `projectPath` and `reader` for later use and returns a `dal.DB` that satisfies (in addition to the base `dal.DB` contract): `dbschema.SchemaReader`, `ddl.SchemaModifier`, `ddl.TransactionalDDL`, and `dal.ConcurrencyAware`.

`NewDatabase` MUST return an error if `projectPath` is empty or if `os.Stat(projectPath)` reports the path does not exist. It MUST NOT read any collection definitions at construction time — those are lazy, loaded on first schema call. It MUST NOT execute any DDL or write any data.

### ConcurrencyAware

#### REQ: concurrency-aware-false

`Database.SupportsConcurrentConnections() bool` MUST return `false`. inGitDB writes to a git working tree. Concurrent writers against the same working-tree directory produce data races on both the record files and the `.collection/definition.yaml` metadata. The DALgo `ConcurrencyAware` contract collapses to a single boolean; the MVP returns `false` unconditionally.

### TransactionalDDL

#### REQ: transactional-ddl-false

`Database.SupportsTransactionalDDL() bool` MUST return `false`. The filesystem has no native multi-operation transaction. Multi-step DDL operations (e.g. `AlterCollection` with several `AlterOp` values) are applied operation-by-operation in order; a failure mid-sequence leaves the collection in a partially-altered state and returns a `*ddl.PartialSuccessError` listing applied, failed, and not-attempted ops. A git-backed atomic commit approach (write to temp, then git-commit) is explicitly deferred to a follow-up Feature. Callers wanting atomicity SHOULD check `ddl.SupportsTransactionalDDL(db)` before calling `AlterCollection` with multiple ops.

### dbschema.SchemaReader

#### REQ: list-collections

`Database.ListCollections(ctx context.Context, parent *dal.Key) ([]dal.CollectionRef, error)` MUST walk `projectPath` looking for directories that contain `.collection/definition.yaml`. The `parent *dal.Key` argument is ignored (inGitDB has no catalog hierarchy) — when non-nil it is treated the same as nil. Each such directory yields one `dal.CollectionRef` whose `Name` is the directory's path relative to `projectPath`, using `/` as the separator (matching the inGitDB collection ID convention). Subdirectories under `.collection/`, `$records/`, `subcollections/`, and `views/` MUST NOT be treated as top-level collections. The result MUST be sorted alphabetically by `Name`. A directory scan error MUST be returned as-is (wrapped with `fmt.Errorf`/`%w`).

#### REQ: describe-collection

`Database.DescribeCollection(ctx context.Context, ref *dal.CollectionRef) (*dbschema.CollectionDef, error)` MUST locate the YAML file at `<projectPath>/<ref.Name>/.collection/definition.yaml`, parse it into `ingitdb.CollectionDef`, and map the result to `*dbschema.CollectionDef` as follows:

- `Name`: echoed from `ref.Name`
- `Fields`: one `dbschema.FieldDef` per entry in `CollectionDef.ColumnsOrder` (or alphabetical order when `ColumnsOrder` is absent), mapped via REQ:type-mapping; `Nullable` is `true` unless the column has `Required: true`
- `PrimaryKey`: when `definition.yaml` carries a `primary_key` field (a list of column names), echoed back as `[]dal.FieldName`. Otherwise (legacy projects that predate PK persistence), a single-element slice `["$key"]` synthesized from the record-key convention. Persisting `primary_key` is REQUIRED by REQ:create-collection step 5 — newly-created collections always round-trip the source PK column names losslessly. Drivers MUST NOT include `$key` in `Fields`; when used, it is a synthetic PK only.
- `Indexes`: an empty slice (inGitDB collections have no per-collection index declarations today; see REQ:list-indexes and Outstanding Questions)

If the collection directory does not exist or its `definition.yaml` is absent, `DescribeCollection` MUST return `(nil, err)` where `err.Error()` contains the substring `"not found"` and the collection name. The exact error type is plan-time (see Outstanding Questions).

#### REQ: list-indexes

`Database.ListIndexes(ctx context.Context, ref *dal.CollectionRef) ([]dbschema.IndexDef, error)` MUST return an empty (non-nil) slice and a nil error. inGitDB collections have no per-collection index declarations in `definition.yaml` today. Full-text search indexes are declared at the project subscriber level (not collection-level YAML), so they are out of scope. When inGitDB adds per-collection index declarations, this REQ will be revisited.

#### REQ: list-constraints

`Database.ListConstraints(ctx context.Context, ref *dal.CollectionRef) ([]dbschema.ConstraintDef, error)` MUST return a one-element slice containing a synthesized primary-key constraint, and a nil error. The element MUST have `Type == dbschema.PrimaryKeyConstraint`. The richer PK column information lives on `DescribeCollection.PrimaryKey`; `dbschema.ConstraintDef` is intentionally minimal (Name + Type only). inGitDB has no other declared constraints (NOT NULL, CHECK, FK are not stored in `definition.yaml`).

#### REQ: list-referrers

`Database.ListReferrers(ctx context.Context, ref *dal.CollectionRef) ([]dbschema.Referrer, error)` MUST return `(nil, &dbschema.NotSupportedError{Op: "ListReferrers", Backend: "ingitdb", Reason: "inGitDB has no native foreign-key declarations"})`. inGitDB `ColumnDef.ForeignKey` is a free-text hint for tooling; it is not a structural FK constraint. A `ListReferrers` implementation that scans all columns for matching `ForeignKey` strings is a follow-up Feature.

### Type Mapping

#### REQ: type-mapping

The driver MUST implement a bidirectional mapping between `ingitdb.ColumnType` and `dbschema.Type`:

| `ingitdb.ColumnType` | `dbschema.Type` |
|---|---|
| `string` | `String` |
| `int` | `Int` |
| `float` | `Float` |
| `bool` | `Bool` |
| `date` | `Time` (inGitDB stores dates as ISO 8601 strings; `dbschema.Time` is the nearest portable type) |
| `time` | `Time` |
| `datetime` | `Time` |
| `any` | `String` (best-effort; the column may hold structured data — stored verbatim) |
| `map[locale]string` | `String` (the locale-map column is mapped as a string column; locale expansion is a runtime concern, not a schema type) |
| `map[*]string` / other map variants | `String` (same rationale) |

Round-trip note: mapping is **lossy** in one direction. `dbschema.Time` maps back to `datetime` when writing `definition.yaml` (the most general time column type). `date`, `time`, and `datetime` all round-trip to `dbschema.Time` on read; callers cannot distinguish them from the `dbschema` layer alone.

### ddl.SchemaModifier

#### REQ: create-collection

`Database.CreateCollection(ctx context.Context, c dbschema.CollectionDef, opts ...ddl.Option) error` MUST:

1. Validate that `c.Name` is a valid collection name (non-empty, no path traversal, no leading/trailing whitespace).
2. Validate that all fields map to a supported `ingitdb.ColumnType` via REQ:type-mapping. A field with `Type == dbschema.Null` or any unrepresentable type MUST return an error naming the field before any filesystem write.
3. Check whether `<projectPath>/<c.Name>/.collection/` already exists. If it exists and `ddl.WithIfNotExists()` is NOT set, return an error. If it exists and `IfNotExists` IS set, return nil without writing.
4. Create the directory `<projectPath>/<c.Name>/.collection/` (including parent `<projectPath>/<c.Name>/`).
5. Write `<projectPath>/<c.Name>/.collection/definition.yaml` from a canonical `ingitdb.CollectionDef` built from `c.Fields` (in declared order), with `RecordFile` set to a default of `{name: "{key}.yaml", format: yaml, type: "map[string]any"}`. When `c.PrimaryKey` is non-empty, persist the PK column names as `primary_key: [...]` in the YAML so `DescribeCollection` can later round-trip them losslessly (see REQ:describe-collection). When `c.PrimaryKey` is empty, omit the field — `DescribeCollection` will synthesize `["$key"]` for backward compatibility.
6. If `c.Indexes` is non-empty, log a warning and silently skip index creation (no per-collection index concept today; see Outstanding Questions).
7. Register the collection in `<projectPath>/.ingitdb/root-collections.yaml` so the validator-backed `CollectionsReader` (used by `loadDefinition` for record transactions) sees it. See REQ:auto-register-in-root-collections for the exact contract.

All filesystem errors MUST be returned wrapped with `fmt.Errorf`/`%w`.

#### REQ: auto-register-in-root-collections

After steps 1-6 of REQ:create-collection succeed (i.e. the on-disk `definition.yaml` has been written), the driver MUST update `<projectPath>/.ingitdb/root-collections.yaml` to map `c.Name → c.Name`. Specifically:

1. Ensure the directory `<projectPath>/.ingitdb/` exists (`os.MkdirAll` with `0o755`).
2. Read the existing flat YAML map at `<projectPath>/.ingitdb/root-collections.yaml` (treat a missing file as an empty map).
3. If the map already contains `c.Name` and the stored value equals `c.Name`, leave it alone (idempotent — re-running CreateCollection with `WithIfNotExists` MUST NOT churn the file).
4. If the map already contains `c.Name` with a DIFFERENT path, leave the existing entry alone and return a non-nil error wrapping `ErrCollectionPathConflict` (defined in this package). This protects user-authored entries from being silently overwritten by an auto-registration with a path-equals-name convention.
5. Otherwise add the entry `c.Name: c.Name` to the map and re-write the file in sorted-by-key order.

The `definition.yaml` write in step 5 of REQ:create-collection is the durable record of the collection. The root-collections.yaml update in this REQ is a derived index. If the index update fails after the definition.yaml write, the driver MUST return a wrapped error naming the registry path; the collection is still on disk and the user can recover by re-running with `WithIfNotExists` (which will retry the registry step idempotently per (3) above).

#### REQ: drop-collection

`Database.DropCollection(ctx context.Context, name string, opts ...ddl.Option) error` MUST:

1. Validate that `name` is a valid collection name (non-empty, no path traversal).
2. Check whether `<projectPath>/<name>/.collection/definition.yaml` exists. If it does NOT exist and `ddl.WithIfExists()` is set, return nil. If it does NOT exist and `IfExists` is NOT set, return an error.
3. As a safety check, verify that `<projectPath>/<name>/.collection/definition.yaml` is present before removing anything. This guards against accidentally deleting a directory that is not an inGitDB collection.
4. Remove the entire `<projectPath>/<name>/` directory tree via `os.RemoveAll`.
5. Remove the entry from `<projectPath>/.ingitdb/root-collections.yaml` per REQ:auto-deregister-from-root-collections.

`DropCollection` MUST NOT check whether the collection directory contains record files before removing — that is the caller's responsibility. If callers want to guard against data loss, they SHOULD call `ListCollections` and count records first.

#### REQ: auto-deregister-from-root-collections

After step 4 of REQ:drop-collection succeeds (the on-disk collection directory has been removed), the driver MUST update `<projectPath>/.ingitdb/root-collections.yaml`:

1. If the registry file does not exist, do nothing (the collection was never registered; not an error).
2. If the registry file exists, read it, remove the entry whose key is `name` (regardless of its stored path value — DropCollection's safety check already confirmed the on-disk presence, so the registry entry is fair game), and re-write the file in sorted-by-key order.
3. If the map becomes empty after removal, the registry file MUST be left as an empty file (the file's presence is a project-shape signal even when no collections are registered).

The `os.RemoveAll` in step 4 of REQ:drop-collection is the durable removal. The registry update in this REQ is index maintenance — if it fails after the directory has been removed, the driver MUST return a wrapped error naming the registry path; the directory is gone and a subsequent re-run with `WithIfExists` will be a no-op for the directory but will retry the index cleanup.

#### REQ: alter-collection

`Database.AlterCollection(ctx context.Context, name string, ops ...AlterOp) error` MUST apply each op in order by:

1. Loading `<projectPath>/<name>/.collection/definition.yaml` into `ingitdb.CollectionDef`.
2. For each op, mutating the in-memory `CollectionDef` and — if the op touches record data — rewriting record files.
3. Writing the updated `definition.yaml` back to disk after each op (to minimize data loss on failure).

Because `SupportsTransactionalDDL() = false`, a failure on op N leaves ops 0..N-1 applied. The driver MUST return `*ddl.PartialSuccessError` naming the applied ops, the failing op, and the not-attempted ops. The partial-success state is intentional and documented; callers wanting atomicity MUST check `ddl.SupportsTransactionalDDL(db)` before issuing multi-op `AlterCollection` calls.

Per-op semantics:

- **`AddField(f)`** — append `f` (mapped via REQ:type-mapping) to `definition.yaml#columns` and `columns_order`. If `f.Default` is set, rewrite all existing record files to inject the default value for the new field. If `IfNotExists` is set and the field already exists by name, skip as a no-op.
- **`DropField(n)`** — remove column `n` from `columns` and `columns_order` in `definition.yaml`. Rewrite all existing record files to remove the field. If `IfExists` is set and the field does not exist, skip as a no-op.
- **`RenameField(old, new)`** — rename column in `columns` and `columns_order`. Rewrite all existing record files to rename the field key.
- **`ModifyField(n, newDef)`** — update the column's type in `definition.yaml` (via REQ:type-mapping). If the new type is incompatible with existing values, the driver MUST return an error rather than silently truncating data. Record files are NOT rewritten for a pure type metadata change (the files store raw values that remain valid after a type rename). If `n != newDef.Name`, the field is also renamed (same semantics as `RenameField`).
- **`AddIndex(idx)`** — log a warning and skip with no error. inGitDB has no per-collection index mechanism today. Returns a `*ddl.PartialSuccessError` only when it is mid-batch with other ops that already applied.
- **`DropIndex(n)`** — log a warning and skip with no error (same rationale as `AddIndex`).

## Architecture

### Files (target layout in `pkg/dalgo2ingitdb/`)

| File | Responsibility |
|---|---|
| `database.go` (new) | `Database` struct, `NewDatabase` constructor, `SupportsConcurrentConnections()`, `SupportsTransactionalDDL()`. Satisfies `dal.DB` by embedding or delegating the existing read/write logic. |
| `schema_reader.go` (new) | `ListCollections`, `DescribeCollection`, `ListIndexes`, `ListConstraints`, `ListReferrers` implementations. Filesystem walks and YAML reads. |
| `schema_modifier.go` (new) | `CreateCollection`, `DropCollection`, `AlterCollection` implementations. All filesystem writes, `definition.yaml` marshaling. |
| `type_mapping.go` (new) | Bidirectional `ingitdb.ColumnType` ↔ `dbschema.Type` translation table. |
| `collection.go` (existing) | `CollectionForKey` — unchanged. |
| `parse.go`, `csv.go`, `locale.go`, `batch_parsers.go` (existing) | Record content parsing — unchanged, but used by `AlterCollection`'s record-rewrite path. |

### Dependencies

- `github.com/dal-go/dalgo` — `dbschema`, `ddl`, `dal.ConcurrencyAware`. Must be the version that includes `SchemaReader`, `SchemaModifier`, `TransactionalDDL`, and `ConcurrencyAware` interfaces (already present in the repo's `go.mod` as of this Feature).
- `github.com/ingitdb/ingitdb-cli/pkg/ingitdb` — `CollectionDef`, `ColumnDef`, `RecordFileDef`, `CollectionsReader` (all already depended upon).
- `gopkg.in/yaml.v3` — reading and writing `definition.yaml` (already a transitive dependency).
- Standard library: `os`, `path/filepath`, `fmt`, `io/fs`.

## Testing Strategy

- **Unit tests for `type_mapping.go`** — table-driven round-trip coverage for all `ingitdb.ColumnType` → `dbschema.Type` → back conversions. No I/O.
- **Unit tests for `schema_reader.go`** — use `os.DirFS`-backed in-memory testdata trees (via `testing/fstest.MapFS`) to exercise `ListCollections` path filtering, `DescribeCollection` YAML loading, and the synthesized PK/constraints. No git dependency.
- **Unit tests for `schema_modifier.go`** — use `t.TempDir()` directories; create minimal `definition.yaml` fixtures, apply ops, assert resulting YAML content and directory structure.
- **Integration tests** — write a small end-to-end test in `dalgo2ingitdb_test` (external test package) that exercises the full `NewDatabase → CreateCollection → DescribeCollection → AlterCollection → DropCollection` lifecycle against a real `t.TempDir()`. Confirms REQ:create-describe-round-trip. No git daemon required; plain filesystem is sufficient for MVP.

## Out of Scope

- **Transactional DDL** — writing to a temp directory and atomically renaming / git-committing is a follow-up Feature. MVP is best-effort with `PartialSuccessError`.
- **Remote git URLs** — `NewDatabase` takes a local filesystem path only. The `--remote` access layer is a separate concern (handled by `dalgo2ghingitdb`).
- **Concurrent writes** — `SupportsConcurrentConnections() = false`; concurrent writer behavior is undefined.
- **Per-column index declarations in `definition.yaml`** — inGitDB does not yet have this concept. `AddIndex`/`DropIndex` are no-ops with warnings.
- **`ListReferrers` via `ColumnDef.ForeignKey` scanning** — the free-text `ForeignKey` field is a UX hint; wiring it to `ListReferrers` is a follow-up Feature.
- **SQL, SQLite, PostgreSQL** — entirely separate drivers and Features.
- **Schema migration for incompatible type changes** — `ModifyField` returns an error rather than attempting data coercion.

## Assumption Carryover

No source Idea exists. The implicit assumptions this Feature commits to:

| Tier | Assumption | Status |
|---|---|---|
| Must-be-true | `ingitdb.CollectionsReader.ReadDefinition` loads all collections from the project root recursively, making it usable as the backing implementation for `ListCollections` | Plan-time: audit `validator.NewReader()` and confirm it walks subdirectories. |
| Must-be-true | `ingitdb.CollectionDef.Columns` + `ColumnsOrder` is the complete field descriptor — no additional per-field metadata in sibling files that `CreateCollection` must also write | Confirmed by reading `collection_def.go`. |
| Must-be-true | Writing `definition.yaml` via `yaml.Marshal(ingitdb.CollectionDef{…})` and then re-reading it via `yaml.Unmarshal` round-trips faithfully for the fields this Feature writes | Plan-time: verify YAML tags, especially `yaml:"-"` fields on `DirPath` and `ID` which are runtime-only. |
| Should-be-true | `os.RemoveAll` is safe to use for `DropCollection` on the collection directory (no symlinks or mounts inside typical inGitDB repos) | Assumed; documented in REQ:drop-collection. |
| Should-be-true | The `dbschema.ConstraintDef` type supports a `PrimaryKeyConstraint` kind variant | Plan-time: check `dbschema/constraint.go` for the enum value. |

## Implementation

Source files implementing this feature (annotated with
`// specscore: feature/dalgo2ingitdb-dbschema-ddl-coverage`):

- [`pkg/dalgo2ingitdb/schema_modifier.go`](../../pkg/dalgo2ingitdb/schema_modifier.go)
- [`pkg/dalgo2ingitdb/schema_reader.go`](../../pkg/dalgo2ingitdb/schema_reader.go)
- [`pkg/dalgo2ingitdb/type_mapping.go`](../../pkg/dalgo2ingitdb/type_mapping.go)

## Acceptance Criteria

### AC: new-database-opens-existing-path

**Requirements:** dalgo2ingitdb-dbschema-ddl-coverage#req:new-database-constructor

**Given** a valid local directory at `./testdata/my-project` containing at least one collection subdirectory
**When** the caller invokes `dalgo2ingitdb.NewDatabase("./testdata/my-project", reader)`
**Then** the call returns `(db, nil)`; `db.(dbschema.SchemaReader) != nil`; `db.(ddl.SchemaModifier) != nil`; `db.(dal.ConcurrencyAware) != nil`; `db.(ddl.TransactionalDDL) != nil`.

### AC: new-database-rejects-missing-path

**Requirements:** dalgo2ingitdb-dbschema-ddl-coverage#req:new-database-constructor

**Given** a path `./testdata/does-not-exist` that does not exist on the filesystem
**When** the caller invokes `dalgo2ingitdb.NewDatabase("./testdata/does-not-exist", reader)`
**Then** the call returns `(nil, err)` where `err` is non-nil; no filesystem writes occur.

### AC: supports-concurrent-connections-false

**Requirements:** dalgo2ingitdb-dbschema-ddl-coverage#req:concurrency-aware-false

**Given** any `dalgo2ingitdb.Database` value
**When** the caller invokes `db.(dal.ConcurrencyAware).SupportsConcurrentConnections()`
**Then** the return value is `false`.

### AC: supports-transactional-ddl-false

**Requirements:** dalgo2ingitdb-dbschema-ddl-coverage#req:transactional-ddl-false

**Given** any `dalgo2ingitdb.Database` value
**When** the caller invokes `ddl.SupportsTransactionalDDL(db)`
**Then** the return value is `false`.

### AC: list-collections-walks-project-root

**Requirements:** dalgo2ingitdb-dbschema-ddl-coverage#req:list-collections

**Given** a project directory with three collection subdirectories (`countries`, `tags`, `todo/tasks`), each containing `.collection/definition.yaml`, plus one plain directory (`docs`) without `.collection/definition.yaml`
**When** the caller invokes `reader.ListCollections(ctx, nil)`
**Then** the result is a `[]dal.CollectionRef` of length 3 whose `Name` fields are `"countries"`, `"tags"`, `"todo/tasks"` in alphabetical order; `"docs"` is absent; the second return is `nil`.

### AC: describe-collection-maps-columns

**Requirements:** dalgo2ingitdb-dbschema-ddl-coverage#req:describe-collection, dalgo2ingitdb-dbschema-ddl-coverage#req:type-mapping

**Given** a `countries/.collection/definition.yaml` with columns `name` (type: string, required: true) and `population` (type: int) in that `columns_order`
**When** the caller invokes `reader.DescribeCollection(ctx, &dal.CollectionRef{Name: "countries"})`
**Then** the result is a `*dbschema.CollectionDef` with `Name == "countries"`, `PrimaryKey == ["$key"]`, and `Fields` in order: `{Name: "name", Type: String, Nullable: false}`, `{Name: "population", Type: Int, Nullable: true}`. `Indexes` is empty.

### AC: describe-collection-not-found

**Requirements:** dalgo2ingitdb-dbschema-ddl-coverage#req:describe-collection

**Given** a project directory with no `nonexistent` collection subdirectory
**When** the caller invokes `reader.DescribeCollection(ctx, &dal.CollectionRef{Name: "nonexistent"})`
**Then** the result is `(nil, err)` where `err.Error()` contains both `"not found"` and `"nonexistent"`.

### AC: list-indexes-returns-empty

**Requirements:** dalgo2ingitdb-dbschema-ddl-coverage#req:list-indexes

**Given** any existing collection
**When** the caller invokes `reader.ListIndexes(ctx, ref)`
**Then** the result is a non-nil empty slice and a nil error.

### AC: list-constraints-returns-pk

**Requirements:** dalgo2ingitdb-dbschema-ddl-coverage#req:list-constraints

**Given** any existing collection
**When** the caller invokes `reader.ListConstraints(ctx, ref)`
**Then** the result is a one-element `[]dbschema.ConstraintDef` with `Type == PrimaryKeyConstraint` and `Fields == ["$key"]`, and a nil error.

### AC: list-referrers-not-supported

**Requirements:** dalgo2ingitdb-dbschema-ddl-coverage#req:list-referrers

**Given** any existing collection
**When** the caller invokes `reader.ListReferrers(ctx, ref)`
**Then** the result is `(nil, *dbschema.NotSupportedError)` where `err.(*dbschema.NotSupportedError).Op == "ListReferrers"`.

### AC: create-collection-writes-definition-yaml

**Requirements:** dalgo2ingitdb-dbschema-ddl-coverage#req:create-collection

**Given** an empty project directory and a `dbschema.CollectionDef` `c` for a `tags` collection with fields `label` (String) and `color` (String, Nullable)
**When** the caller invokes `ddl.CreateCollection(ctx, db, c)`
**Then** the call returns `nil`; the file `<projectPath>/tags/.collection/definition.yaml` exists; parsing it via `yaml.Unmarshal` yields an `ingitdb.CollectionDef` with two columns matching `label` and `color`; a follow-up `reader.DescribeCollection(ctx, &dal.CollectionRef{Name: "tags"})` returns a `CollectionDef` semantically equal to `c` (with PK synthesized as `["$key"]`).

### AC: create-collection-if-not-exists

**Requirements:** dalgo2ingitdb-dbschema-ddl-coverage#req:create-collection

**Given** a project directory that already has a `tags/.collection/definition.yaml`
**When** the caller invokes `ddl.CreateCollection(ctx, db, cTags, ddl.WithIfNotExists())`
**Then** the call returns `nil`; the existing `definition.yaml` is unchanged.

### AC: create-collection-rejects-invalid-type

**Requirements:** dalgo2ingitdb-dbschema-ddl-coverage#req:create-collection, dalgo2ingitdb-dbschema-ddl-coverage#req:type-mapping

**Given** a `dbschema.CollectionDef` whose `Fields` includes one entry with `Type == dbschema.Null`
**When** the caller invokes `ddl.CreateCollection(ctx, db, c)`
**Then** the call returns a non-nil error before any filesystem write; the error message names the offending field; no directories or files are created.

### AC: create-describe-pk-roundtrip-single

**Requirements:** dalgo2ingitdb-dbschema-ddl-coverage#req:create-collection, dalgo2ingitdb-dbschema-ddl-coverage#req:describe-collection

**Given** a `dbschema.CollectionDef` `c` with `PrimaryKey: []dal.FieldName{"AlbumId"}`
**When** the caller invokes `ddl.CreateCollection(ctx, db, c)` followed by `reader.DescribeCollection(ctx, &ref)`
**Then** the returned `*dbschema.CollectionDef.PrimaryKey` equals `[]dal.FieldName{"AlbumId"}` (NOT the synthesized `["$key"]`); the on-disk `definition.yaml` contains a `primary_key: [AlbumId]` field.

### AC: create-describe-pk-roundtrip-composite

**Requirements:** dalgo2ingitdb-dbschema-ddl-coverage#req:create-collection, dalgo2ingitdb-dbschema-ddl-coverage#req:describe-collection

**Given** a `dbschema.CollectionDef` `c` with `PrimaryKey: []dal.FieldName{"PlaylistId", "TrackId"}`
**When** the caller invokes `ddl.CreateCollection(ctx, db, c)` followed by `reader.DescribeCollection(ctx, &ref)`
**Then** the returned `PrimaryKey` equals `["PlaylistId", "TrackId"]` in that order; `Fields` includes both columns.

### AC: describe-collection-legacy-no-pk-field

**Requirements:** dalgo2ingitdb-dbschema-ddl-coverage#req:describe-collection

**Given** a hand-written `definition.yaml` with NO `primary_key` field (an older project that predates PK persistence)
**When** the caller invokes `reader.DescribeCollection(ctx, &ref)`
**Then** the returned `PrimaryKey` equals `[]dal.FieldName{"$key"}` (backward-compatible synthesized PK).

### AC: create-collection-registers-in-root-collections

**Requirements:** dalgo2ingitdb-dbschema-ddl-coverage#req:create-collection, dalgo2ingitdb-dbschema-ddl-coverage#req:auto-register-in-root-collections

**Given** an empty project directory with no `.ingitdb/root-collections.yaml` file and a `dbschema.CollectionDef` for `tags`
**When** the caller invokes `ddl.CreateCollection(ctx, db, c)`
**Then** the call returns `nil`; `<projectPath>/.ingitdb/root-collections.yaml` exists; parsing it via `yaml.Unmarshal` into a `map[string]string` yields exactly one entry `tags: tags`; a subsequent `validator.NewCollectionsReader().ReadDefinition(projectPath)` returns a `*ingitdb.Definition` whose `Collections` map contains `tags`.

### AC: create-collection-appends-to-existing-root-collections

**Requirements:** dalgo2ingitdb-dbschema-ddl-coverage#req:auto-register-in-root-collections

**Given** a project directory with a pre-existing `.ingitdb/root-collections.yaml` containing exactly `tags: tags`
**When** the caller invokes `ddl.CreateCollection(ctx, db, cForLabels)` (a new collection `labels`)
**Then** the registry file now contains both entries `labels: labels` and `tags: tags`, in sorted-by-key order; neither entry has been mutated.

### AC: create-collection-idempotent-registry-with-if-not-exists

**Requirements:** dalgo2ingitdb-dbschema-ddl-coverage#req:auto-register-in-root-collections

**Given** a project directory where `tags` is already created (its `definition.yaml` exists and the registry contains `tags: tags`)
**When** the caller invokes `ddl.CreateCollection(ctx, db, cTags, ddl.WithIfNotExists())`
**Then** the call returns `nil`; the registry file contents (including byte order) are unchanged.

### AC: create-collection-rejects-registry-conflict

**Requirements:** dalgo2ingitdb-dbschema-ddl-coverage#req:auto-register-in-root-collections

**Given** a project directory where `.ingitdb/root-collections.yaml` contains `tags: legacy/tags-path` (a user-authored entry mapping the `tags` id to a non-default path)
**When** the caller invokes `ddl.CreateCollection(ctx, db, cTags)`
**Then** the call returns a non-nil error wrapping `ErrCollectionPathConflict`; the registry file is unchanged; the `<projectPath>/tags/.collection/definition.yaml` may have been written by the earlier steps and SHOULD be cleaned up by the caller, but the driver does NOT roll it back. (The error message names both the registry entry and the conflict so the user can resolve it.)

### AC: drop-collection

**Requirements:** dalgo2ingitdb-dbschema-ddl-coverage#req:drop-collection

**Given** a project directory with a `tags/` collection directory containing `definition.yaml` and some record files
**When** the caller invokes `ddl.DropCollection(ctx, db, "tags")`
**Then** the call returns `nil`; a subsequent `os.Stat("<projectPath>/tags")` returns an `fs.ErrNotExist` error; `reader.ListCollections` no longer includes `"tags"`.

### AC: drop-collection-if-exists

**Requirements:** dalgo2ingitdb-dbschema-ddl-coverage#req:drop-collection

**Given** a project directory with no `tags/` collection
**When** the caller invokes `ddl.DropCollection(ctx, db, "tags", ddl.WithIfExists())`
**Then** the call returns `nil`; the project directory is unchanged.

### AC: drop-collection-safety-check

**Requirements:** dalgo2ingitdb-dbschema-ddl-coverage#req:drop-collection

**Given** a project directory with a plain `docs/` subdirectory that does NOT contain `.collection/definition.yaml`
**When** the caller invokes `ddl.DropCollection(ctx, db, "docs")`
**Then** the call returns a non-nil error; the `docs/` directory is NOT removed.

### AC: drop-collection-deregisters-from-root-collections

**Requirements:** dalgo2ingitdb-dbschema-ddl-coverage#req:drop-collection, dalgo2ingitdb-dbschema-ddl-coverage#req:auto-deregister-from-root-collections

**Given** a project directory where `tags` and `labels` are both registered (`<projectPath>/.ingitdb/root-collections.yaml` contains `labels: labels` and `tags: tags`)
**When** the caller invokes `ddl.DropCollection(ctx, db, "tags")`
**Then** the call returns `nil`; the `tags/` directory is removed; the registry file now contains exactly `labels: labels` (`tags` removed); a subsequent `validator.NewCollectionsReader().ReadDefinition(projectPath)` returns a definition whose `Collections` map contains only `labels`.

### AC: drop-collection-tolerates-missing-registry

**Requirements:** dalgo2ingitdb-dbschema-ddl-coverage#req:auto-deregister-from-root-collections

**Given** a project directory where `tags` exists on disk but `.ingitdb/root-collections.yaml` does NOT exist (e.g. created by an older driver version before auto-registration shipped)
**When** the caller invokes `ddl.DropCollection(ctx, db, "tags")`
**Then** the call returns `nil`; the `tags/` directory is removed; the absence of the registry file is not treated as an error.

### AC: alter-collection-add-field

**Requirements:** dalgo2ingitdb-dbschema-ddl-coverage#req:alter-collection

**Given** a `tags` collection with one field `label` (String) and no records
**When** the caller invokes `ddl.AlterCollection(ctx, db, "tags", ddl.AddField(dbschema.FieldDef{Name: "color", Type: dbschema.String, Nullable: true}))`
**Then** the call returns `nil`; `reader.DescribeCollection` for `tags` now shows two fields: `label` and `color` in that order.

### AC: alter-collection-drop-field

**Requirements:** dalgo2ingitdb-dbschema-ddl-coverage#req:alter-collection

**Given** a `tags` collection with fields `label` and `color`, and 2 record files each containing both fields
**When** the caller invokes `ddl.AlterCollection(ctx, db, "tags", ddl.DropField("color"))`
**Then** the call returns `nil`; `DescribeCollection` shows only `label`; both record files no longer contain the `color` key.

### AC: alter-collection-rename-field

**Requirements:** dalgo2ingitdb-dbschema-ddl-coverage#req:alter-collection

**Given** a `tags` collection with field `label` and 2 record files
**When** the caller invokes `ddl.AlterCollection(ctx, db, "tags", ddl.RenameField("label", "name"))`
**Then** the call returns `nil`; `DescribeCollection` shows field `name` (not `label`); both record files have key `name` where `label` was.

### AC: alter-collection-partial-success-on-failure

**Requirements:** dalgo2ingitdb-dbschema-ddl-coverage#req:alter-collection, dalgo2ingitdb-dbschema-ddl-coverage#req:transactional-ddl-false

**Given** a `tags` collection with field `label` and a two-op `AlterCollection` call whose second op drops a nonexistent field `nonexistent` (without `IfExists`)
**When** the caller invokes `ddl.AlterCollection(ctx, db, "tags", ddl.AddField(dbschema.FieldDef{Name: "color", Type: dbschema.String}), ddl.DropField("nonexistent"))`
**Then** the call returns a `*ddl.PartialSuccessError`; `DescribeCollection` shows `label` and `color` (the first op was applied); the second op is recorded as the failing op in the error.

### AC: create-describe-round-trip

**Requirements:** dalgo2ingitdb-dbschema-ddl-coverage#req:create-collection, dalgo2ingitdb-dbschema-ddl-coverage#req:describe-collection, dalgo2ingitdb-dbschema-ddl-coverage#req:type-mapping

**Given** a fresh empty project directory and a `dbschema.CollectionDef` `c` for an `events` collection with fields: `title` (String, Nullable: false), `starts_at` (Time, Nullable: true), `attendees` (Int, Nullable: true), and `is_public` (Bool, Nullable: false)
**When** the caller invokes `ddl.CreateCollection(ctx, db, c)` then `reader.DescribeCollection(ctx, &dal.CollectionRef{Name: "events"})`
**Then** the second call returns a `*dbschema.CollectionDef` with `Name == "events"`, `PrimaryKey == ["$key"]`, and `Fields` matching `c.Fields` except that `starts_at.Type` is `Time` (round-trips via `datetime` intermediate). `Indexes` is empty.

## Outstanding Questions

- **Per-collection index declarations.** inGitDB's `definition.yaml` does not declare secondary indexes today. `AddIndex`/`DropIndex` are currently no-ops. If inGitDB adds an `indexes` field to `CollectionDef`, this Feature must be revised. Tracked as a follow-up once the inGitDB schema supports it.
- **Collection-not-found error type.** The `dbschema` package exports `*NotSupportedError` but not a `NotFoundError`. `DescribeCollection`'s contract is currently content-based (`err.Error()` contains `"not found"` + collection name). Plan-time options: (a) add `dbschema.NotFoundError` to `dal-go/dalgo` first; (b) define `dalgo2ingitdb.ErrCollectionNotFound` locally. The AC pins only the message-content contract so both options pass.
- **`$key` as a synthetic PK field name.** Using the literal string `"$key"` as the synthesized primary-key field name may conflict if inGitDB ever declares a column literally named `$key`. For MVP this is acceptable. A follow-up could use a named constant exported from `ingitdb`.
- **`AlterCollection` with record backfill and large collections.** The current design rewrites all record files in-memory per op. For collections with thousands of records this may be slow. A streaming rewrite is a follow-up optimization.
- **`dbschema.ConstraintDef` type constant.** The spec assumes `dbschema.PrimaryKeyConstraint` exists as a named variant. Plan-time: verify against `dbschema/constraint.go`; if the constant name differs, update REQ:list-constraints and AC:list-constraints-returns-pk.
- **Default `RecordFile` value in `CreateCollection`.** The spec picks `{name: "{key}.yaml", format: yaml, type: "map[string]any"}` as the default. Plan-time: decide whether to expose a `WithRecordFileDef` option on `CreateCollection` so callers can specify a different format.

---
*This document follows the https://specscore.md/feature-specification*

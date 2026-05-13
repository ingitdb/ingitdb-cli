# Task List: dalgo2ingitdb-dbschema-ddl-coverage

## Phase 1: Foundation

- [ ] **Task 1** — Add `replace github.com/dal-go/dalgo => ../../dal-go/dalgo` to `go.mod`; run `go mod tidy`; verify `go build ./...` and `go test ./...` pass
- [ ] **Task 2** — `type_mapping.go` + `type_mapping_test.go`: bidirectional `ingitdb.ColumnType ↔ dbschema.Type`
- [ ] **Task 3** — `filelock.go` + `filelock_test.go`: `withSharedLock` / `withExclusiveLock` wrapping `github.com/gofrs/flock` (cross-platform: Unix `flock`, Windows `LockFileEx`)

### Checkpoint 1
- [ ] `go build ./pkg/dalgo2ingitdb/...` succeeds
- [ ] `go test -run 'TestTypeMapping|TestFileLock' ./pkg/dalgo2ingitdb/` passes

## Phase 2: Core Interfaces

- [ ] **Task 4** — `database.go` + `database_test.go`: `Database` struct embedding `dal.ConcurrencyAvailable` (true on all platforms), `NewDatabase`, `dal.DB` stubs, `SupportsTransactionalDDL`
- [ ] **Task 5** — `schema_reader.go` + `schema_reader_test.go`: `ListCollections`, `DescribeCollection` (shared lock), `ListIndexes`, `ListConstraints`, `ListReferrers`

### Checkpoint 2
- [ ] `go test -run 'TestDatabase|TestSchemaReader' ./pkg/dalgo2ingitdb/` passes
- [ ] Compile-time assertions in `database.go` hold
- [ ] `db.SupportsConcurrentConnections()` returns `true` (all platforms)

## Phase 3: Write Path

- [ ] **Task 6** — `schema_modifier.go` + `schema_modifier_test.go`: `CreateCollection`, `DropCollection`, `AlterCollection` + `Applier` (all writes under exclusive lock)

### Checkpoint 3
- [ ] `go test -run TestSchemaModifier ./pkg/dalgo2ingitdb/` passes
- [ ] `go vet ./pkg/dalgo2ingitdb/...` clean

## Phase 4: Integration

- [ ] **Task 7** — `integration_test.go`: end-to-end lifecycle + concurrent-read sub-test

### Checkpoint 4 (Final)
- [ ] `go test -timeout=30s ./...` passes
- [ ] `golangci-lint run` clean
- [ ] `specscore spec lint` clean

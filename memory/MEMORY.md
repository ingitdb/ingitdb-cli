# ingitdb-cli memory

## Project
Go CLI for inGitDB (git-backed database). Module: `github.com/ingitdb/ingitdb-cli`

## Key Architecture
- `cmd/ingitdb/commands/` — CLI commands (urfave/cli v3); inject deps via function params
- `pkg/dalgo2fsingitdb/` — local filesystem DALgo implementation
- `pkg/ingitdb/` — schema types (Definition, CollectionDef, RecordFileDef)
- `pkg/dalgo2ingitdb/` — locale helpers (ApplyLocaleToRead/Write)
- demo-dbs/test-db — test fixture database (countries: SingleRecord YAML, companies: SingleRecord JSON)

## Record Types
- `ingitdb.SingleRecord` ("map[string]any") — one file per record; glob with `{key}` template
- `ingitdb.MapOfRecords` ("map[$record_id]map[$field_name]any") — all records in one file
- Files stored under `$records/` subdirectory when name template contains `{key}`

## Command Pattern
All CRUD commands accept: `homeDir, getWd, readDefinition, newDB, logf` deps.
Use `resolveDBPath(cmd, homeDir, getWd)` → `readDefinition(dirPath)` → `newDB(dirPath, def)` → transaction.

## DALgo Query API
- `dal.NewQueryBuilder(dal.From(dal.NewRootCollectionRef(id, "")))`
- `.Where(conditions...).OrderBy(exprs...).Limit(n)`
- `.SelectIntoRecord(func() dal.Record {...})`
- `dal.WhereField(field, operator, value)` — creates Comparison condition
- Operators: `dal.Equal`, `dal.GreaterThen`, `dal.GreaterOrEqual`, `dal.LessThen`, `dal.LessOrEqual`
- `dal.AscendingField(name)`, `dal.DescendingField(name)`

## Implemented: query command (2026-03-10)
Files: `pkg/dalgo2fsingitdb/{slice_records_reader,tx_query}.go`, `cmd/ingitdb/commands/{query,query_parser,query_output}.go`
- WHERE/ORDER BY/LIMIT evaluated in-memory
- `DisableSliceFlagSeparator: true` on query command to allow commas in numeric WHERE values

## YAML Parsing Note
`gopkg.in/yaml.v3` parses integers as `int` not `float64`. The `toFloat64()` helper in tx_query.go handles all numeric types. Tests must not assume `float64` from YAML-deserialized data.

## golangci-lint
Not installed in environment; use `go vet ./...` as substitute.

## Lint/Test Gates
```
go build ./...
go test -timeout=10s ./...
go vet ./...
```

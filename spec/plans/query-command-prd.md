# Plan: Implement `ingitdb query` Command

## Context
The `query` command exists as a stub returning "not yet implemented". The user wants it to work with field selection, filtering, ordering, and multiple output formats. Filtering and ordering must be implemented inside `pkg/dalgo2fsingitdb` (as a proper `ExecuteQueryToRecordsReader` implementation), not as ad-hoc logic in the command layer.

Note: `-f=%id` in the 5th example appears to be a typo for `-f=$id`; we use `$id` throughout.

---

## CLI Specification

```
ingitdb query -c=COLLECTION [-f=FIELDS] [--where=EXPR] [--order-by=FIELDS] [--format=csv|json|yaml] [--path=PATH]
```

| Flag | Alias | Required | Description |
|---|---|---|---|
| `--collection` | `-c` | yes | Collection ID |
| `--fields` | `-f` | no | `*` = all, `$id` = record key, `field1,field2` = specific fields |
| `--where` | `-w` | no | Filter expression: `field>value`, `field==value`, etc. Repeatable for AND. |
| `--order-by` | | no | Comma-separated fields; prefix `-` = descending |
| `--format` | | no | `csv` (default, with header), `json`, `yaml`, `md` (markdown table) |
| `--path` | | no | DB directory (default: current dir) |

**Operators in `--where`:** `>=`, `<=`, `>`, `<`, `==`, `=` (= treated as ==)
**Number formatting:** commas are stripped before parsing (e.g. `1,000,000` → `1000000`)

---

## Files to Modify / Create

### 1. `pkg/dalgo2fsingitdb/tx_query.go` _(new file)_
Implement `executeQueryToRecordsReader(ctx, r readonlyTx, query dal.StructuredQuery)`:
- Extract collection ID from `query.From()` (cast to `dal.CollectionRef`, get `.ID`)
- Look up `colDef` from `r.db.def.Collections`
- Read **all** records from disk using same file-reading logic as `Get()`:
  - `SingleRecord`: glob + read each file
  - `MapOfRecords`: read single file, iterate keys
- **Apply WHERE** filter in-memory: evaluate `query.Where()` condition against each record's field map
  - Handle `dal.Comparison` with `dal.FieldRef` + `dal.Constant`
  - For `$id` field references, compare against the record key
  - Operators: `==`, `>`, `>=`, `<`, `<=`
  - Type coercion: compare numbers as `float64`
- **Apply ORDER BY** in-memory sort on `query.OrderBy()` expressions (stable sort, multi-key)
- **Apply LIMIT** if `query.Limit() > 0`
- Return a simple slice-backed `dal.RecordsReader` (`sliceRecordsReader`)

Also implement `ExecuteQueryToRecordsReader` in `tx_readonly.go` to delegate to this new function (remove the panic).

### 2. `pkg/dalgo2fsingitdb/slice_records_reader.go` _(new file)_
`sliceRecordsReader` implementing `dal.RecordsReader`:
- Wraps `[]dal.Record` slice
- `Next()` returns next record or `dal.ErrNoMoreRecords`
- `Cursor()` returns `("", nil)`
- `Close()` is a no-op

### 3. `cmd/ingitdb/commands/query.go` _(rewrite)_
Change signature: `func Query(homeDir, getWd, readDefinition, newDB, logf)` (matching `Read`, `Create`, etc.)

Add flags:
- `--collection / -c` (required)
- `--fields / -f` (default `*`)
- `--where / -w` (repeatable via `cli.StringSliceFlag`)
- `--order-by` (single string, comma-separated)
- `--format` (default `csv`)
- `--path`

Action:
1. `resolveDBPath` → `dirPath`
2. `readDefinition(dirPath)` → `def`
3. Validate collection exists in `def`
4. Parse `--fields` → `[]string` (empty = all)
5. Parse each `--where` value → `[]dal.Condition` using `parseWhereExpr(s)`
6. Parse `--order-by` → `[]dal.OrderExpression` using `parseOrderBy(s)`
7. Build query:
   ```go
   qb := dal.NewQueryBuilder(dal.From(dal.CollectionRef{ID: colID}))
   qb.Where(conditions...).OrderBy(orderExprs...)
   q := qb.SelectIntoRecord(func() dal.Record { return dal.NewRecordWithData(key, map[string]any{}) })
   ```
8. `newDB(dirPath, def)` → `db`
9. `db.RunReadonlyTransaction(ctx, func(ctx, tx) { reader, _ = tx.ExecuteQueryToRecordsReader(ctx, q) })`
10. Iterate reader, project `--fields`, write output

### 4. `cmd/ingitdb/commands/query_parser.go` _(new file)_
- `parseWhereExpr(s string) (dal.Condition, error)` — parse `field>=value` into `dal.WhereField(...)`
  - Strip commas from numeric literal values before type inference
- `parseOrderBy(s string) ([]dal.OrderExpression, error)` — comma-split, `-field` → `dal.DescendingField`, `field` → `dal.AscendingField`
- `projectRecord(data map[string]any, id string, fields []string) map[string]any` — select specified columns (`$id` → use the record's key value)

### 5. `cmd/ingitdb/commands/query_output.go` _(new file)_
- `writeCSV(w io.Writer, records []map[string]any, columns []string) error` — header + rows
- `writeJSON(w io.Writer, records []map[string]any) error` — JSON array, indented
- `writeYAML(w io.Writer, records []map[string]any) error` — YAML list
- `writeMarkdown(w io.Writer, records []map[string]any, columns []string) error` — GFM table (header row, separator row, data rows)

### 6. `cmd/ingitdb/main.go`
Update: `commands.Query(homeDir, getWd, readDefinition, newDB, logf)` to pass dependencies.

### 7. `docs/cli/commands/query.md` _(rewrite)_
Update to reflect all new flags with examples matching the user's spec.

### 8. `cmd/ingitdb/commands/query_test.go` _(update)_
- Remove `TestQuery_NotYetImplemented` (command now works)
- Add `TestQuery_ReturnsCommand` coverage for new flags (collection, fields, where, order-by, format)

### 9. `cmd/ingitdb/commands/query_parser_test.go` _(new file)_
Comprehensive table-driven tests for all parsers:
- **`TestParseWhereExpr`**: valid expressions (`field>value`, `field>=value`, `field==value`, `field=value`, `field<value`, `field<=value`), number with thousands-separator commas (`population>1,000,000`), unknown operator error, malformed input errors
- **`TestParseOrderBy`**: single ascending, single descending (`-field`), multiple comma-separated, empty string, whitespace handling
- **`TestProjectRecord`**: `*` selects all fields, `$id` selects only key, mixed fields, missing fields omitted gracefully

### 10. `cmd/ingitdb/commands/query_output_test.go` _(new file)_
- **`TestWriteCSV`**: header row correctness, data rows, empty result set
- **`TestWriteMarkdown`**: header row, separator row (`|---|---|`), data rows
- **`TestWriteJSON`**: valid JSON array output
- **`TestWriteYAML`**: valid YAML list output

### 11. `pkg/dalgo2fsingitdb/tx_query_test.go` _(new file)_
Integration-level tests against `demo-dbs/test-db/countries`:
- Query all records (`*` fields) returns expected count
- WHERE filter (`population>100000000`) returns subset
- ORDER BY ascending / descending produces correct order
- LIMIT respected
- Unknown collection returns error

---

## Reuse of Existing Code
- `resolveDBPath()` — `cmd/ingitdb/commands/record_context.go`
- `readDefinition` seam — same pattern as `Create`, `Read`
- `newDB(rootDirPath, def)` — `dalgo2fsingitdb.NewLocalDBWithDef`
- `readRecordFromFile` / `readMapOfRecordsFile` — `pkg/dalgo2fsingitdb/record_file.go` (reuse file reading logic in tx_query.go)
- `dalgo2ingitdb.ApplyLocaleToRead` — apply before yielding records
- `dal.NewQueryBuilder`, `dal.From`, `dal.CollectionRef`, `dal.WhereField`, `dal.AscendingField`, `dal.DescendingField` — from dalgo v0.41.6

---

## Implementation Strategy
Delegate to subagents:
1. **go-engineer** orchestrates overall implementation, breaking into subtasks
2. **go-coder** for isolated pieces: parsers, output writers, slice reader
3. **go-tester** for comprehensive test suites on parsers and output functions
4. **go-reviewer** reviews all produced code before marking done
5. **ci-runner** runs full quality gate at the end

## Verification

```bash
# 1. Build
go build ./...

# 2. Unit + integration tests
go test -timeout=10s ./...

# 3. Lint (must report no errors)
golangci-lint run

# 4. Fix any lint errors, then re-run tests to confirm
go test -timeout=10s ./...

# 5. Manual smoke tests (run by Claude after implementation, from repo root)
go run cmd/ingitdb/ query -c=countries --fields='$id'
go run cmd/ingitdb/ query -c=countries -f='*'
go run cmd/ingitdb/ query -c=countries --fields='$id,currency,flag'
go run cmd/ingitdb/ query -c=countries -f='$id,currency,flag' --format=json
go run cmd/ingitdb/ query -c=countries -f='$id,currency,flag' --format=yaml
go run cmd/ingitdb/ query -c=countries -f='$id,currency,flag' --format=md
go run cmd/ingitdb/ query -c=countries -f='$id' --where='population>1000000'
go run cmd/ingitdb/ query -c=countries -f='$id' --where='population>1,000,000'
go run cmd/ingitdb/ query -c=countries -f='$id,population' --order-by='-population'
```

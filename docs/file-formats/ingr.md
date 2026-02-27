# INGR File Format Specification

**Extension:** `.ingr`  
**Purpose:** Compact, deterministic, Git-friendly fixed-line record format.

---

## 1. Design Goals

- **Compact** — minimal syntax, no structural noise.
- **Deterministic** — fixed structure, zero ambiguity.
- **Diff-friendly** — one field per line, stable ordering.
- **Streamable** — readable line-by-line.
- **JSON-typed** — each value is a single-line JSON expression.
- **Self-describing** — first line carries the recordset name and column list.

---

## 2. Core Concept

An `.ingr` file begins with a **metadata header line** followed by a sequence of records.

Each record:

- Contains a **fixed number of lines (N)**, where N equals the number of columns declared in the header.
- Each line represents **one field value**, encoded as JSON.
- Records follow each other immediately with no delimiters.

**Parser rule:**

1. Read line 1 → parse header to get column list (length = N).
2. Read `N` lines → 1 record.
3. Repeat until EOF.

---

## 3. Structure

### 3.1 Header Line

The first line of every `.ingr` file is a metadata header:

```
#INGR: {recordset_name}: $ID, col2, col3, ...
```

- Starts with `#INGR: ` (literal prefix).
- **Recordset name** — an arbitrary identifier for the dataset (e.g. `people`, `orders/2024`). Its meaning is defined by the producer.
- Followed by `: ` (colon + space).
- **Column list** — comma-separated column names, separated by `, ` (comma + space) for readability. Parsers may trim surrounding whitespace from each name.
- **`$ID`** is the reserved name for the record key (always the first column).

Example:

```
#INGR: people: $ID, name, age
```

### 3.2 Fixed Field Count

The number of fields per record **N** is determined by the number of columns in the header (including `$ID`).

### 3.3 Value Encoding

Each field value is encoded as a **compact single-line JSON expression**:

| Go/source type | INGR line |
|----------------|-----------|
| string         | `"hello world"` |
| integer        | `123` |
| float          | `3.14` |
| boolean        | `true` / `false` |
| null / missing | `null` |
| object         | `{"key1":"value1","key2":2}` |
| array          | `[1,2,3]` |

JSON objects and arrays must be written without embedded newlines (compact form).

### 3.4 Example (fields: `$ID`, `name`, `age`)

```
#INGR: people: $ID, name, age
"john"
"John Doe"
35
"jane"
"Jane Smith"
29
# 2 records
```

Parsed as:

| $ID  | name       | age |
|------|------------|-----|
| john | John Doe   | 35  |
| jane | Jane Smith | 29  |

### 3.5 Footer Line

The last line of every `.ingr` file is a record count footer **with no trailing newline**:

```
# 1 record
```
or
```
# {N} records
```

- Uses `record` (singular) for exactly 1, `records` (plural) for all other counts (including 0).
- No newline character after this line.

---

## 4. Rules

1. Encoding: UTF-8.
2. Line separator: LF (`\n`).
3. Line 1 is the metadata header; it must match the format above.
4. Each field value line must be a valid single-line JSON expression.
5. JSON objects and arrays must not contain embedded newlines.
6. `(total_lines - 2) % N == 0` (subtract 1 for header + 1 for footer).
7. Last line is the footer; it must match `# {N} records` or `# 1 record`.
8. No newline after the footer line.
9. No inline delimiters between records.

---

## 5. Example With Null Field

Header + 2 records + footer, `N = 3`:

```
#INGR: people: $ID, name, age
"john"
"John Doe"
35
"jane"
null
29
# 2 records
```

Second record:

- `$ID` = `"jane"`
- `name` = `null` (missing or explicitly null)
- `age` = `29`

---

## 6. Validation

A valid `.ingr` file must:

- Have line 1 be a well-formed header.
- Have the last line be a well-formed footer matching the actual record count.
- Not contain partial records between header and footer.
- Have every value line be a valid single-line JSON expression.
- Have no trailing newline after the footer.

Validation condition:

```
(total_lines - 2) % N == 0
```

---

## 7. Why `.ingr` Works Well in Git

- One field per line → clean, minimal diffs.
- JSON encoding is compact and unambiguous.
- Strings with special characters (tabs, newlines) are safely JSON-escaped.
- Stable, deterministic structure.
- Easier merge conflict resolution.
- Works naturally with line-based tools (grep, jq, awk).

---

## 8. Suitable Use Cases

Good for:

- Structured flat or nested data with predictable schema.
- Git-tracked datasets.
- CLI-driven workflows.
- Deterministic record storage.

Not ideal for:

- Variable field counts.
- Binary data.

---

## 9. Summary

`.ingr` is a self-describing, deterministic, fixed-line record format:

- Line 1: `#INGR: {recordset_name}: $ID, col2, col3, ...`
- Lines 2…(end-1): `N` JSON-encoded values per record, one value per line
- Last line: `# {N} records` or `# 1 record` — no trailing newline
- No record delimiters
- Optimised for simplicity and Git friendliness

# INGR File Format Specification - v1-RC

- **Extension:**: `.ingr`
- **Version:** `1.0.0-RC`
- **Purpose:** Compact, deterministic, self-describing, Git-friendly fixed-line record format.

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
- Records follow each other immediately with no delimiters (optional delimiter lines are described in §3.6).

**Parser rule:**

1. Read line 1 → parse header to get column list (length = N).
2. Read `N` lines → 1 record. Skip any `#` delimiter line immediately after.
3. Repeat until EOF.

---

## 3. Structure

### 3.1 Header Line

The first line of every `.ingr` file is a metadata header:

```
# INGR.io | {recordset_name}: $ID[:type], col2[:type], col3[:type], ...
```

- Starts with `# INGR.io | ` (spaces after `#` and around `|` are optional for parsers).
- **Recordset name** — an arbitrary identifier for the dataset (e.g. `people`, `orders/2024`). Its meaning is defined by
  the producer.
- Followed by `: ` (colon + space).
- **Column list** — comma-separated column names, separated by `, ` (comma + space) for readability. Parsers may trim
  surrounding whitespace from each name.
- **`$ID`** is the reserved name for the record key (always the first column).
- Each column name may carry an **optional type annotation** using `col:type` syntax (see §3.2).

Example (untyped):

```
# INGR.io | people: $ID, name, age
```

Example (typed):

```
# INGR.io | people: $ID:string, name:string, age:int
```

### 3.2 Column Type Annotations

Column names may include an **optional** type annotation separated by `:`, analogous to TypeScript's optional type
suffixes. When no type is given, the column is untyped (equivalent to `any`).

```
col_name[:type]
```

#### Scalar types

| Annotation  | Meaning                                              | Example value          |
|-------------|------------------------------------------------------|------------------------|
| `string`    | UTF-8 string                                         | `"hello"`              |
| `int`       | Integer                                              | `42`                   |
| `float`     | Floating-point                                       | `3.14`                 |
| `decimal`   | Exact-precision decimal                              | `19.99`                |
| `number`    | Any numeric value                                    | `100` / `3.14`         |
| `bool`      | Boolean                                              | `true` / `false`       |
| `date`      | Date — ISO 8601 string                               | `"2024-01-15"`         |
| `time`      | Time string                                          | `"14:30:00"`           |
| `datetime`  | Date + time — ISO 8601 string                        | `"2024-01-15T14:30:00Z"` |
| `any`       | Untyped; accepts any JSON value (equivalent to `any` in TypeScript or a plain `object` in JavaScript — string, number, boolean, null, array, or object) | `null` / `{}` / `[]`  |

#### Composite types (Go notation)

| Annotation              | Meaning                          | Example value                   |
|-------------------------|----------------------------------|---------------------------------|
| `[]{type}`              | Slice / array                    | `[1,2,3]` / `["a","b"]`        |
| `map[string]{type}`     | Map with string keys             | `{"x":1,"y":2}`                 |
| `map[int]{type}`        | Map with integer keys            | `{"1":10,"2":20}`               |
| `map[float]{type}`      | Map with float keys              | `{"3.14":"pi"}`                 |
| Nested, e.g. `[]map[string]int` | Slice of string→int maps  | `[{"a":1},{"b":2}]`            |

> **Map key encoding:** JSON requires all object keys to be strings. Numeric key types (`int`, `float`, `decimal`, `number`)
> are therefore encoded as JSON strings (e.g. `"42"`, `"3.14"`). Parsers must convert them to the declared key type and
> return an error if conversion fails.

> **Notes**
> - `$ID` may also be typed, e.g. `$ID:int` or `$ID:string`.
> - Untyped columns (no `:type` suffix) default to `any`.
> - Type annotations are advisory metadata for producers, consumers, and validators; they do not change the underlying
>   JSON encoding of values.

Example header with mixed typed and untyped columns:

```
# INGR.io | example: $ID:int, FirstName:string, Age:int, Weight:decimal, N:number, IsConfirmed:bool, UserData:map[string]any
```

### 3.3 Fixed Field Count

The number of fields per record **N** is determined by the number of columns in the header (including `$ID`).

### 3.4 Value Encoding

Each field value is encoded as a **compact single-line JSON expression**:

| Go/source type | INGR line                    |
|----------------|------------------------------|
| string         | `"hello world"`              |
| integer        | `123`                        |
| float          | `3.14`                       |
| boolean        | `true` / `false`             |
| null / missing | `null`                       |
| object         | `{"key1":"value1","key2":2}` |
| array          | `[1,2,3]`                    |

JSON objects and arrays must be written without embedded newlines (compact form).

### 3.5 Example (fields: `$ID:string`, `name:string`, `age:int`)

Without record delimiter:

```
# INGR.io | people: $ID:string, name:string, age:int
"john"
"John Doe"
35
"jane"
"Jane Smith"
29
# 2 records
```

With record delimiter (see §3.6):

```
# INGR.io | people: $ID:string, name:string, age:int
"john"
"John Doe"
35
#
"jane"
"Jane Smith"
29
#
# 2 records
```

Parsed as:

| $ID  | name       | age |
|------|------------|-----|
| john | John Doe   | 35  |
| jane | Jane Smith | 29  |

### 3.6 Record Delimiter (Optional)

Records may be separated by a **delimiter line** — a line containing only `#` with no other content.

Rules:

- The delimiter is **optional**. A file may use it or omit it entirely.
- If used, a delimiter line **must appear after every record**, including the first. It cannot be used only between some records.
- A delimiter line after the **last record** (before the record count line) is permitted.
- Parsers must accept both forms (with and without delimiter).

### 3.7 Footer

The footer starts immediately after the last record (or the last record's delimiter line). It consists of one **required** line followed by any number of **optional** comment lines:

**Required — record count** (always the first footer line; the trailing newline is optional but recommended):

```
# 1 record
```

or

```
# {N} records
```

- Uses `record` (singular) for exactly 1, `records` (plural) for all other counts (including 0).
- Must be the first line after the records (and any trailing delimiter).
- Parsers should accept the count line with or without a trailing newline.

**Optional — additional footer lines** (each starting with `#`):

Any number of `#`-prefixed lines may follow. Their content and meaning are agreed upon between producer and consumer.
Example:

```
# sha256:{hex}
```

- When present, `sha256` names the hash algorithm and `{hex}` is the lowercase hex-encoded SHA-256 digest of all file
  content above this line (header + records + count line including its `\n`).

The last line of the file (whether the count line or the last optional line) has **no trailing newline**. When the count
line is the last line, producers may include a trailing newline — parsers must accept both.

The space after `#` is preserved but optional for parsers.

---

## 4. Rules

1. Encoding: UTF-8.
2. Line separator: LF (`\n`).
3. Line 1 is the metadata header; it must match the format above.
4. Each field value line must be a valid single-line JSON expression.
5. JSON objects and arrays must not contain embedded newlines.
6. `(total_lines - 1 - footer_lines - delimiter_lines) % N == 0` where `footer_lines ≥ 1`.
7. First footer line must match `# {N} records` or `# 1 record`.
8. All subsequent footer lines must start with `#`.
9. No newline after the last line of the file.
10. Record delimiter lines (`#`) are optional, but if used must appear after every record.

---

## 5. Example With Null Field

Header + 2 records + footer, `N = 3`:

```
# INGR.io people: $ID, name, age
"john"
"John Doe"
35
"jane"
null
29
# 2 records
# sha256:3a7bd3e2360a3d80...
```

Second record:

- `$ID` = `"jane"`
- `name` = `null` (missing or explicitly null)
- `age` = `29`

---

## 6. Validation

A valid `.ingr` file must:

- Have line 1 be a well-formed header.
- Have the first footer line match `# {N} records` or `# 1 record` with the actual record count.
- Not contain partial records between header and footer.
- Have every value line be a valid single-line JSON expression.
- Have no trailing newline after the last line.
- Either use record delimiters after every record, or not use them at all.

Validation condition:

```
(total_lines - 1 - footer_lines - delimiter_lines) % N == 0   // footer_lines ≥ 1
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

- Line 1: `# INGR.io | {recordset_name}: $ID[:type], col2[:type], col3[:type], ...`
- Lines 2…(end-N): `N` JSON-encoded values per record, one value per line
- Optional: a bare `#` delimiter line after each record (all or none)
- First footer line: `# {N} records` (required, with `\n` unless last line)
- Additional footer lines: optional `#`-prefixed lines (e.g. `# sha256:{hex}`)
- Optimised for simplicity and Git friendliness

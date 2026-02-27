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

---

## 2. Core Concept

An `.ingr` file is a sequence of records.

Each record:

- Contains a **fixed number of lines (N)**.
- Each line represents **one field value**, encoded as JSON.
- Records follow each other immediately.
- No delimiters are required if N is known.

**Parser rule:**

Read `N` lines → 1 record  
Repeat until EOF

---

## 3. Structure

### 3.1 Fixed Field Count

The number of fields per record **must be defined externally** (schema, CLI flag, metadata, or convention).

### 3.2 Value Encoding

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

### 3.3 Example (`N = 3`, fields: `first_name`, `last_name`, `age`)

```
"John"
"Doe"
35
"Jane"
"Smith"
29
```

Parsed as:

| Record | first_name | last_name | age |
|--------|------------|-----------|-----|
| 1      | John       | Doe       | 35  |
| 2      | Jane       | Smith     | 29  |

---

## 4. Rules

1. Encoding: UTF-8.
2. Line separator: LF (`\n`).
3. Each field occupies exactly one line.
4. Each line must be a valid JSON expression (string, number, boolean, null, object, or array).
5. JSON objects and arrays must not contain embedded newlines.
6. Total number of lines must be divisible by `N`.
7. No header row.
8. No inline delimiters.

---

## 5. Optional Schema Declaration (Recommended)

Schema should be declared outside the file.

Example:

fields = 3
field_order = [first_name, last_name, age]

Or via CLI:

ingitdb import –fields 3 file.ingr

Keeping schema external ensures:

- Stable diffs
- No metadata noise
- Cleaner records

---

## 6. Example With Null Field

`N = 3`

```
"John"
"Doe"
35
"Jane"
null
29
```

Second record:

- Field 1 = `"Jane"`
- Field 2 = `null` (missing or explicitly null)
- Field 3 = `29`

---

## 7. Validation

A valid `.ingr` file must:

- Not contain partial records.
- Not contain trailing extra lines.
- Maintain strict line ordering.
- Have every line be a valid single-line JSON expression.

Validation condition:

(total_lines % N) == 0

---

## 8. Why `.ingr` Works Well in Git

- One field per line → clean diffs.
- JSON encoding is compact and unambiguous.
- Strings with special characters (tabs, newlines) are safely JSON-escaped.
- Stable structure.
- Easier merge conflict resolution.
- Works naturally with line-based tools (grep, jq, awk).

---

## 9. Suitable Use Cases

Good for:

- Structured flat or nested data with predictable schema
- Git-tracked datasets
- CLI-driven workflows
- Deterministic record storage

Not ideal for:

- Variable field counts
- Binary data

---

## 10. Summary

`.ingr` is a deterministic, fixed-line record format:

- `N` lines per record
- Each line is a JSON-encoded value
- No delimiters
- Schema defined externally
- Optimised for simplicity and Git friendliness
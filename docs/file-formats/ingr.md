# INGR File Format Specification

**Extension:** `.ingr`  
**Purpose:** Compact, deterministic, Git-friendly fixed-line record format.

---

## 1. Design Goals

- **Compact** — minimal syntax, no structural noise.
- **Deterministic** — fixed structure, zero ambiguity.
- **Diff-friendly** — one field per line, stable ordering.
- **Streamable** — readable line-by-line.
- **No escaping rules** — raw text only.

---

## 2. Core Concept

An `.ingr` file is a sequence of records.

Each record:

- Contains a **fixed number of lines (N)**.
- Each line represents **one field**.
- Records follow each other immediately.
- No delimiters are required if N is known.

**Parser rule:**

Read `N` lines → 1 record  
Repeat until EOF

---

## 3. Structure

### 3.1 Fixed Field Count

The number of fields per record **must be defined externally** (schema, CLI flag, metadata, or convention).

Example: `N = 3`

John
Doe
35
Jane
Smith
29

Parsed as:

| Record | Field 1 | Field 2 | Field 3 |
|--------|---------|---------|---------|
| 1      | John    | Doe     | 35      |
| 2      | Jane    | Smith   | 29      |

---

## 4. Rules

1. Encoding: UTF-8.
2. Line separator: LF (`\n`).
3. Each field occupies exactly one line.
4. Empty line is a valid field value.
5. Total number of lines must be divisible by `N`.
6. No header row.
7. No inline delimiters.
8. No escaping or quoting rules.

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

## 6. Example With Empty Field

`N = 3`

John
Doe
35
Jane

29

Second record:

- Field 1 = Jane
- Field 2 = "" (empty)
- Field 3 = 29

---

## 7. Validation

A valid `.ingr` file must:

- Not contain partial records.
- Not contain trailing extra lines.
- Maintain strict line ordering.

Validation condition:

(total_lines % N) == 0

---

## 8. Why `.ingr` Works Well in Git

- One field per line → clean diffs.
- No JSON punctuation (`{}`, `,`, quotes).
- No CSV escaping issues.
- Stable structure.
- Easier merge conflict resolution.
- Works naturally with line-based tools (grep, sed, awk).

---

## 9. Suitable Use Cases

Good for:

- Structured flat data with predictable schema
- Git-tracked datasets
- CLI-driven workflows
- Deterministic record storage

Not ideal for:

- Nested structures
- Variable field counts
- Binary data

---

## 10. Summary

`.ingr` is a deterministic, fixed-line record format:

- `N` lines per record
- No delimiters
- No escaping
- Schema defined externally
- Optimised for simplicity and Git friendliness
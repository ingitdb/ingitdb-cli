### 🔹 find` — search records by value _(not yet implemented)_

[Source Code](../../cmd/ingitdb/commands/find.go)


```
ingitdb find [--path=PATH] [--in=REGEXP] [--substr=TEXT] [--re=REGEXP] [--exact=VALUE] [--fields=FIELDS] [--limit=N]
```

Searches record files for fields matching the given pattern. At least one of `--substr`, `--re`, or `--exact` must
be provided. When multiple search flags are given they are combined with OR.

| Flag            | Required     | Description                                                                               |
| --------------- | ------------ | ----------------------------------------------------------------------------------------- |
| `--path=PATH`   | no           | Path to the database directory. Defaults to the current working directory.                |
| `--substr=TEXT` | one of three | Match records where any field contains TEXT as a substring.                               |
| `--re=REGEXP`   | one of three | Match records where any field value matches REGEXP.                                       |
| `--exact=VALUE` | one of three | Match records where any field value equals VALUE exactly.                                 |
| `--in=REGEXP`   | no           | Regular expression scoping the search to a sub-path.                                      |
| `--fields=LIST` | no           | Comma-separated list of field names to search. Without this flag all fields are searched. |
| `--limit=N`     | no           | Maximum number of matching records to return.                                             |

**Examples:**

```shell
# 📘 Search all fields for a substring
ingitdb find --substr=Dublin

# 📘 Regex search with a result cap
ingitdb find --re='pop.*[0-9]{6,}' --limit=10

# 🔁 Search specific fields only
ingitdb find --substr=Dublin --fields=name,capital

# 🔁 Scope search to a sub-path and match a specific field value exactly
ingitdb find --exact=Ireland --in='countries/.*' --fields=country
```

---


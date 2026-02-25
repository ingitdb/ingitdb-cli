# 🐙 Reading Records: Local vs GitHub

This document shows how to read the same record using the ingitdb CLI from two different sources:
1. Local file system (using `dalgo2ingitdb`)
2. GitHub repository (using `dalgo2ghingitdb`)

## 📂 Example: Reading the "active" tag record

Both commands read the same record: `todo/tags/active` from the test-ingitdb repository.

### 🖥️ Command 1: Read from Local Repository

```bash
ingitdb read record --path test-ingitdb --id todo.tags/active
```

Or from current directory (if you're in the repository):

```bash
ingitdb read record --id todo.tags/active
```

**Explanation:**
- `read record` - Read a single record
- `--id todo.tags/active` - Record ID in format `collection.id/record-key`
  - `todo.tags` = collection ID
  - `active` = record key
- `--path test-ingitdb` - (Optional) Path to the database directory (defaults to current directory)

**Output:**
```yaml
title: Active
```

### 🖥️ Command 2: Read from GitHub Repository

```bash
ingitdb read record --github ingitdb/ingitdb-cli@main --id todo.tags/active
```

Or specify without a branch (uses default branch):

```bash
ingitdb read record --github ingitdb/ingitdb-cli --id todo.tags/active
```

**Explanation:**
- `read record` - Read a single record
- `--github ingitdb/ingitdb-cli@main` - GitHub repository as `owner/repo[@ref]`
  - `ingitdb` = GitHub organization
  - `ingitdb-cli` = repository name
  - `@main` = (Optional) Git reference (branch, tag, or commit)
- `--id todo.tags/active` - Same record ID format as local version

**Output:**
```yaml
title: Active
```

## 📂 Additional Options

### 🔹 Change Output Format to JSON

Both commands support JSON output with `--format json`:

**Local:**
```bash
ingitdb read record --id todo.tags/active --format json
```

**GitHub:**
```bash
ingitdb read record --github ingitdb/ingitdb-cli --id todo.tags/active --format json
```

**Output:**
```json
{
  "title": "Active"
}
```

## 📂 Important Notes

1. **Collection ID Format**
   - Use dots for collection hierarchy: `todo.tags`, `todo.tasks.statuses`
   - Can also use slashes: `todo/tags`, `todo/tasks/statuses` (both are equivalent)

2. **GitHub Access**
   - Public repositories: no token required
   - Private repositories: supply `--token=TOKEN` or set the `GITHUB_TOKEN` environment variable
   - Write operations (`create record`, `update record`, `delete record`) always require a token

3. **Record ID Structure**
   - Format: `collection.id/record-key`
   - Examples:
     - `todo.tags/active` - reads `active` from the `todo.tags` collection
     - `countries/ie` - reads `ie` from the `countries` collection

4. **Git References**
   - Can specify `@branch`, `@tag`, or `@commit` for GitHub
   - Examples:
     - `--github owner/repo@main`
     - `--github owner/repo@v1.0.0`
     - `--github owner/repo@abc123def456`
     - `--github owner/repo` (uses default branch)

## 📂 Error Handling

**Record not found:**
```
error: record not found: todo.tags/nonexistent
```

**Invalid record ID:**
```
error: invalid --id: collection not found for ID "invalid.format"
```

**GitHub error (public repo not found):**
```
error: failed to resolve remote definition: github api error status 404
```

**Configuration not found:**
```
error: failed to resolve remote definition: .ingitdb file not found in repository
```

## ⚡ Quick Comparison Table

| Aspect | Local | GitHub |
|--------|-------|--------|
| Command | `--path <dir>` | `--github owner/repo[@ref]` |
| Authentication | None | None for public reads; token required for private reads and all writes |
| File Access | Direct filesystem | GitHub REST API |
| Performance | Fast (local disk) | Network dependent |
| Branch Support | N/A | Yes (`@branch`, `@tag`, `@commit`) |
| Write Support | Yes | Yes (creates one commit per write) |
| Use Case | Development, local testing | Remote inspection, CI/CD, scripted writes |

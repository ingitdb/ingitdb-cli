# Idea: Markdown Record Insert UX

**Status:** Draft
**Date:** 2026-05-12
**Owner:** alexander.trakhimenok@gmail.com
**Promotes To:** —
**Supersedes:** —
**Related Ideas:** —

## Problem Statement

How Might We let users insert markdown records (YAML frontmatter + body) via the CLI in a way that feels natural, composable, and powerful for both humans and scripts?

## Context

`create record` currently requires `--data=YAML` for all record content. For `format: markdown`
collections this is lossy — there is no way to supply the Markdown body. The `markdown.Parse()`
function (pkg/ingitdb/markdown/) already handles `---`-delimited frontmatter + body, so the
wire format is settled; only the CLI input path is missing.

Prior art: Hugo, Jekyll, and Obsidian all treat the `---` frontmatter + body file as the
canonical unit of a markdown record, making stdin/file-redirect the natural insert interface.

## Recommended Direction

Enhance `create record` with a **stdin-or-editor dual mode**. When `--data` is absent, read
from stdin (pipe/heredoc) and parse via `markdown.Parse()` for markdown collections or
`yaml.Unmarshal()` for yaml/json collections. Add `--edit` to open `$EDITOR` with a
schema-generated template for interactive one-off inserts. Keep `--id` required and `--data`
unchanged for backward compatibility.

This requires no new subcommand, no new flag conventions, and reuses existing parsing code.

## Alternatives Considered

- **New `insert` subcommand** — clean discovery surface, but adds CLI surface area for behavior
  that is a natural extension of `create record`. Rejected.
- **Auto-slug ID from `title` frontmatter** — minimal keystrokes, but collision risk and
  non-obvious behavior. Rejected for MVP.
- **`--collection` + stdin (no `--id`)** — matches the user's initial sketch, but breaks
  consistency with every other CRUD command. Rejected.

## MVP Scope

Extend `create record` so that `cat record.md | ingitdb create record --id products/p1`
inserts a markdown record end-to-end, including frontmatter → structured fields and
body → `$content`. The `--edit` flag is in scope for MVP; bulk import is not.

## Not Doing (and Why)

- Auto-slug from title — collision risk, non-obvious behavior; defer until explicit demand
- New insert subcommand — create record already exists; this is a natural extension
- Bulk import (*.md glob) — separate concern, separate ticket

## Key Assumptions to Validate

| Tier | Assumption | How to validate |
|------|------------|-----------------|
| Must-be-true | stdin TTY detection works reliably in GitHub Actions / Docker | integration test against non-TTY env |
| Should-be-true | `$EDITOR` template derived from `columns_order` is enough to author a valid record | user test with a real collection |
| Might-be-true | `--edit` without `--id` (prompt post-edit) would be valuable | defer; gather feedback after MVP ships |


## SpecScore Integration

- **New Features this would create:** none — this is an enhancement to `cli/create-record`
- **Existing Features affected:** [cli/create-record](../features/cli/create-record/README.md)
- **Dependencies:** none

## Outstanding Questions

None at this time.

---
*This document follows the https://specscore.md/idea-specification*

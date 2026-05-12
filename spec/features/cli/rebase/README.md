# Feature: Rebase Command

**Status:** Implementing

## Summary

The `ingitdb rebase` command runs `git rebase` on top of a base reference and automatically resolves common inGitDB-specific conflicts in generated files (collection `README.md` files, materialized views, data indexes). Conflicts in source files (`.ingitdb.yaml`, hand-edited records, Go source) abort the rebase and are reported to the user.

## Problem

Long-lived branches in an inGitDB repository regularly conflict on generated artifacts even when the source data does not. Hand-resolving those conflicts is busywork because the generated content is reproducible from the source records. `ingitdb rebase` automates that reproduction so that the only conflicts a human ever sees are the meaningful ones.

## Behavior

### Invocation

#### REQ: subcommand-name

The command MUST be invoked as `ingitdb rebase`. All flags are optional.

### Flags

#### REQ: base-ref-resolution

The `--base_ref=REF` flag MUST set the git ref to rebase onto. When omitted, the command MUST fall back to the `BASE_REF` environment variable, then to `GITHUB_BASE_REF`. When none of those are set, the command MUST fail with a diagnostic message.

#### REQ: resolve-flag

The `--resolve=FILES` flag MUST accept a comma-separated list of categories of generated files that the command is allowed to auto-resolve. The value `readme` opts in to auto-resolving collection `README.md` conflicts. When the flag is omitted, no categories are auto-resolved.

### Conflict handling

#### REQ: detects-and-resolves-readmes

When `git rebase` halts on a conflict, the command MUST inspect the unmerged files via `git diff --name-only --diff-filter=U`. If every conflicting file is a collection `README.md` AND `readme` was passed to `--resolve`, the command MUST regenerate only those READMEs, stage them with `git add`, and continue the rebase via `git rebase --continue`.

#### REQ: aborts-on-unresolvable

If any conflicting file falls outside the categories listed in `--resolve` (for example a `.ingitdb.yaml` or a Go source file), the command MUST abort the rebase and print the list of unresolved paths so the user can resolve them manually.

## Acceptance Criteria

### AC: auto-resolves-readme-conflicts

**Requirements:** cli/rebase#req:subcommand-name, cli/rebase#req:resolve-flag, cli/rebase#req:detects-and-resolves-readmes

Given a branch whose only conflict against `main` is in collection `README.md` files, `ingitdb rebase --base_ref=main --resolve=readme` regenerates the conflicted READMEs, stages them, and runs the rebase to completion without prompting the user. The resulting tree contains the regenerated README content.

### AC: aborts-on-source-conflict

**Requirements:** cli/rebase#req:aborts-on-unresolvable

When a conflict touches a source file (e.g. a hand-edited record YAML), `ingitdb rebase` does not silently resolve it; instead it aborts the rebase and lists the conflicting paths. The user must resolve them and continue manually.

## Outstanding Questions

- Should `--resolve=views` and `--resolve=indexes` be promoted from "supported categories" to documented and tested behavior on par with `readme`?
- Should the command default `--resolve` to a curated set when run inside CI (detected via `GITHUB_ACTIONS` etc.)?

---
*This document follows the https://specscore.md/feature-specification*

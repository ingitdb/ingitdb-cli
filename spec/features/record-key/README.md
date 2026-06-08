---
format: https://specscore.md/feature-specification
status: Draft
---

# Feature: Record Key

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/record-key?op=explore) | [Edit](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/record-key?op=edit) | [Ask question](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/record-key?op=ask) | [Request change](https://specscore.studio/app/github.com/ingitdb/ingitdb-cli/spec/features/record-key?op=request-change) |
**Status:** Draft
**Source Ideas:** —
**Source Idea:** [`derived-record-keys`](../../ideas/derived-record-keys.md)

## Summary

Record-key Features define how inGitDB resolves, validates, and exposes the
canonical identity of a record across schema validation, reads, writes, and CLI
commands.

## Problem

Record identity is a cross-cutting concern: storage paths use `{key}`, CLI
commands expose `$id`, and write paths need one effective key before they can
check collisions. This umbrella keeps record-key behavior grouped separately
from any one CLI command or record-file format.

## Contents

| Child | Description |
|---|---|
| [derived-keys](derived-keys/README.md) | Allow a single-record collection to derive its canonical key from fields inside the record and validate filename drift. |

## Children

| Feature | Summary |
|---|---|
| [derived-keys/](derived-keys/README.md) | Schema-level derived keys for single-record collections, including validation and single-record insert integration. |

## Open Questions

None at this time.

---
*This document follows the https://specscore.md/feature-specification*

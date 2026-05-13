# Feature: Path Targeting

**Status:** Implementing

## Summary

The `--path=PATH` flag selects the local database directory that a command operates on. When omitted, commands default to the current working directory. `--path` is mutually exclusive with `--remote` on every command that supports both: a single invocation targets either a local directory or a remote repository, never both.

## Problem

Most ingitdb-cli commands operate on a database located somewhere on disk. Without a shared `--path` convention, every command would invent its own way of locating the database, and the CWD-as-default behavior would be inconsistent.

## Behavior

### Flag

#### REQ: path-flag-name

When a command operates on a local database directory, the flag for selecting it MUST be `--path=PATH`. Other flag names (e.g. `--dir`, `--root`) MUST NOT be introduced for the same purpose.

#### REQ: cwd-default

When `--path` is omitted, the command MUST default to the current working directory.

### Mutual exclusion

#### REQ: mutex-with-remote

On commands that also support `--remote`, the `--path` and `--remote` flags MUST be mutually exclusive. Supplying both MUST result in an error before any data is read or written.

### Path resolution

#### REQ: resolves-relative-paths

Relative `--path` values MUST be resolved against the current working directory. Symbolic links MAY be followed.

## Acceptance Criteria

### AC: defaults-to-cwd

**Requirements:** path-targeting#req:path-flag-name, path-targeting#req:cwd-default

A command run without `--path` from a directory containing a valid `.ingitdb.yaml` operates on that directory. Moving to a different directory and running the same command targets the new directory.

### AC: rejects-path-and-remote-together

**Requirements:** path-targeting#req:mutex-with-remote

Any command supporting both flags rejects `--path=. --remote=github.com/owner/repo` with a clear error message and a non-zero exit code, before performing any I/O.

## Outstanding Questions

- Should `--path` accept a URL-style scheme (e.g. `file://`) for symmetry with future remote storage backends?

---
*This document follows the https://specscore.md/feature-specification*

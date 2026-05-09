# Feature: GitHub Direct Access

**Status:** Implementing

## Summary

Several commands accept `--github=OWNER/REPO[@REF]` as a substitute for `--path` to read or write records directly against a GitHub repository over the REST API, without a local clone. Authentication is provided via `--token` or the `GITHUB_TOKEN` environment variable. Each successful write operation MUST result in exactly one commit in the remote repository.

## Problem

Cloning a repository solely to read or update one record is wasteful when the data already lives on GitHub. Direct GitHub access lets ingitdb-cli participate in CI workflows, scripts, and AI agents that operate on remote repositories without ever touching local disk.

## Behavior

### Flag

#### REQ: github-flag-syntax

The `--github=OWNER/REPO[@REF]` flag MUST accept either `owner/repo` (default ref) or `owner/repo@ref` where `ref` is a branch name, tag name, or commit SHA. The flag MUST be mutually exclusive with `--path`.

### Authentication

#### REQ: token-resolution

A token MAY be supplied via `--token=TOKEN` or the `GITHUB_TOKEN` environment variable. `--token`, when set, MUST take precedence over `GITHUB_TOKEN`.

#### REQ: token-required-for-writes

Read operations against public repositories MUST work without a token. All write operations (`create record`, `update record`, `delete record`) MUST require a token, even when the target repository is public.

### Write semantics

#### REQ: one-commit-per-write

Each successful write operation MUST produce exactly one commit in the remote repository, containing the change for that single record. The command MUST NOT batch multiple unrelated writes into a single commit.

### No local clone

#### REQ: no-local-clone

The command MUST NOT clone the remote repository to local disk in order to satisfy a `--github` operation. All reads and writes are performed via the GitHub REST API.

## Acceptance Criteria

### AC: read-public-without-token

**Requirements:** github-direct-access#req:github-flag-syntax, github-direct-access#req:token-resolution

`ingitdb read record --github=owner/public-repo --id=collection/key` succeeds without `--token` or `GITHUB_TOKEN` set when the repository is public.

### AC: write-creates-one-commit

**Requirements:** github-direct-access#req:token-required-for-writes, github-direct-access#req:one-commit-per-write, github-direct-access#req:no-local-clone

With a valid token, every `--github` write produces exactly one commit in the target repository whose diff is limited to that record. No `.git` directory is created on the caller's filesystem.

## Outstanding Questions

- How should the command behave when GitHub rate-limits the caller mid-operation?
- Should the commit message format be configurable per command or fixed by convention?

---
*This document follows the https://specscore.md/feature-specification*

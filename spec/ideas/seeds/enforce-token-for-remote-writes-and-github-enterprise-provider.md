---
type: sidekick-seed
captured_by: claude
status: done
---
# Enforce a local token-required pre-flight for remote writes

REQ:token-required-for-writes is unenforced: no CLI/library pre-flight rejects a write when no token resolves (flag and all *_TOKEN env vars empty). `remoteToken` may return "" and writes proceed unauthenticated, failing only at the GitHub API (401) instead of with the spec-mandated local "token required" error; no test covers "write without token must fail".

RESOLVED (separate change): the `--provider=github-enterprise` sub-gap on `AC:self-hosted-host-token` was fixed by correcting the spec to use the registered `github` provider (a self-hosted GitHub Enterprise host reuses the `github` adapter); there is no separate `github-enterprise` id. Only the token-required-for-writes enforcement above remains before remote-repo-access can move to Stable.

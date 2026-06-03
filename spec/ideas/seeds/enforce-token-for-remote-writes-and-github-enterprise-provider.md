---
type: sidekick-seed
slug: enforce-token-for-remote-writes-and-github-enterprise-provider
captured_at: 2026-06-03T14:25:20Z
captured_by: claude
captured_during: null
trigger: explicit
status: queued
synchestra_task: null
---
# Enforce a local token-required pre-flight for remote writes, and add the github-enterprise provider to the registry

Two gaps found while reconciling remote-repo-access (7/9 ACs). (1) REQ:token-required-for-writes is unenforced: no CLI/library pre-flight rejects a write when no token resolves (flag and all *_TOKEN env vars empty). `remoteToken` may return "" and writes proceed unauthenticated, failing only at the GitHub API (401) instead of with the spec-mandated local "token required" error; no test covers "write without token must fail". (2) `AC:self-hosted-host-token` is written against `--provider=github-enterprise`, but that id is not in `registeredProviders` (only github/gitlab/bitbucket), so the AC's literal invocation fails with "unknown --provider" before token resolution runs. Token resolution for self-hosted hosts (`TestResolveRemoteToken`, `TestHostTokenEnvName`) is otherwise implemented. Either register a `github-enterprise` provider/adapter or fix the spec to use a registered provider, then add the end-to-end test.

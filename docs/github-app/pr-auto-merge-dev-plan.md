# Dev Plan: PR Auto-Merge

**Feature spec:** [pr-auto-merge-feature.md](./pr-auto-merge-feature.md)

## Goals

Implement the PR auto-validate and merge feature of the inGitDB GitHub App. When a PR is opened
against an inGitDB repository, the App validates the changed records, evaluates the repository's
permission model, posts a structured change summary comment, and merges automatically when all
rules are satisfied.

---

## Dependencies

The following CLI capabilities are assumed to be ready before or alongside App development. They
are CLI deliverables tracked in the CLI roadmap, not App deliverables.

| Capability | Notes |
|---|---|
| `ingitdb validate` — incremental/change mode | Validates only files changed between two commits or between PR base and head |
| `ingitdb resolve` — merge conflict resolution | Rebuilds calculated files (views, indexes); merges record files by policy; returns structured conflict details when unresolvable |
| Change diff computation | Produces schema-change and per-collection/view record-count deltas between two repo states |
| `dalgo2ghingitdb` read/write via GitHub API | File read, list directory, write, delete |

---

## Architecture

### New packages

| Package | Responsibility |
|---|---|
| `server/webhook/` | HTTP endpoint, GitHub webhook signature verification, event routing |
| `server/github_app/` | Top-level orchestrator: receives parsed events, drives the full PR flow |
| `server/github_app/classifier/` | Classifies PR changed files into inGitDB-managed vs out-of-scope |
| `server/github_app/summary/` | Builds the structured change summary (schema changes, data stats, commits) |
| `server/github_app/permissions/` | Parses `github_app` config from `.ingitdb.yaml`; evaluates merge permissions with subcollection inheritance |
| `server/github_app/reviewers/` | Resolves reviewer lists by scope (record → subcollection → collection → default) |
| `server/github_app/commenter/` | Composes and posts PR comments; updates existing App comment on re-runs |
| `server/github_app/merger/` | Executes PR merge via GitHub API; delegates conflict resolution to `ingitdb resolve` |
| `pkg/repocache/` | Clones and caches Git repositories for local operations; shared with `ingitdb serve` |
| `pkg/githubauth/` | GitHub App JWT generation and installation token exchange |

### Interfaces

```go
// RepoDiff returns schema and data change stats between base and head.
type RepoDiff interface {
    Diff(ctx context.Context, owner, repo, base, head string) (DiffResult, error)
}

// RepoCache provides a local clone of a repository ref, creating or reusing one.
type RepoCache interface {
    Acquire(ctx context.Context, owner, repo, ref string) (LocalRepo, error)
}

// LocalRepo is a checked-out working copy of a specific ref.
type LocalRepo interface {
    Dir() string
    Close() error // releases the cache slot
}

// AppAuthTokenSource exchanges App credentials for a short-lived installation token.
type AppAuthTokenSource interface {
    InstallationToken(ctx context.Context, installationID int64) (string, error)
}
```

Concrete implementations are injected; tests supply fakes or recorded fixtures.

---

## Implementation Phases

### Phase 1: Foundation

**Goal:** Receive and verify webhook events; establish App authentication.

- [ ] `pkg/githubauth`: JWT generation from App private key; installation token exchange via
  GitHub API (`POST /app/installations/{id}/access_tokens`)
- [ ] `server/webhook`: `POST /webhooks/github` HTTP handler; verify `X-Hub-Signature-256`
  using HMAC-SHA256; route `pull_request` events (opened / synchronize / reopened) to the
  orchestrator
- [ ] Wire `--github-app` flag into `ingitdb serve`; load `INGITDB_GITHUB_APP_ID`,
  `INGITDB_GITHUB_APP_INSTALLATION_ID`, `INGITDB_GITHUB_APP_PRIVATE_KEY_PATH` from env
- [ ] `pkg/repocache`: clone a repo ref to a temp dir; reuse an existing clone if the same
  owner/repo/ref is already cached; evict by LRU or TTL

**Acceptance:** A `pull_request.opened` webhook is received, signature verified, and logged with
the PR number and head SHA. App auth produces a valid installation token.

---

### Phase 2: File Classification and Validation

**Goal:** Classify PR files; run `ingitdb validate` on the inGitDB subset; create a Check Run.

- [ ] `server/github_app/classifier`: read the PR file list via GitHub API; split into
  inGitDB-managed paths (matched against `root-collections.yaml`) and out-of-scope paths
  (anything not in `allowed_dirs`)
- [ ] Create a GitHub Check Run (`inGitDB Validation`, status `in_progress`) on the PR head SHA
- [ ] Invoke `ingitdb validate` (incremental mode) on the inGitDB files via the cloned repo
  from `repocache`
- [ ] Update Check Run to `success` or `failure` with annotations per validation error

**Acceptance:** A PR with valid inGitDB files produces a passing Check Run. A PR with invalid
records produces a failing Check Run with file/line annotations.

---

### Phase 3: Change Summary

**Goal:** Build the structured change summary included in all PR comments and notifications.

- [ ] `server/github_app/summary`: define `DiffResult` (schema changes, per-collection record
  stats, per-view record stats, commit list); implement `RepoDiff` backed by `pkg/repocache`
  and the CLI diff computation dependency
- [ ] `server/github_app/commenter`: render `DiffResult` into the Markdown template defined in
  the [feature spec](./pr-auto-merge-feature.md); omit empty sections
- [ ] Post the composed comment on the PR; update the existing App comment on `synchronize`
  events to avoid duplicate comments

**Acceptance:** A PR comment shows accurate schema/data change counts and a commit list matching
the PR's actual diff.

---

### Phase 4: Permission Evaluation

**Goal:** Parse the `github_app` config and decide whether the App is allowed to merge.

- [ ] `server/github_app/permissions`: parse `github_app.pr_merge` from `.ingitdb.yaml`;
  resolve effective rule for a given file path using subcollection inheritance (most specific
  prefix wins, fall back to `default`)
- [ ] Evaluate `auto_merge`, `author_must_match_field`, and `allowed_authors` against the PR
  author and the changed records
- [ ] Confirm all changed files are within inGitDB paths or `allowed_dirs`

**Acceptance:** A PR meeting all rules is marked `ALLOWED`; a PR violating any rule is marked
`DENIED` with a reason string.

---

### Phase 5: Reviewer Resolution and Notifications

**Goal:** Notify the right reviewers when the App cannot merge.

- [ ] `server/github_app/reviewers`: resolve reviewer list walking scopes from most specific to
  least (record `reviewer_field` → subcollection → collection → `default`); accumulate all
  matching reviewers
- [ ] Request review via GitHub API
  (`POST /repos/{owner}/{repo}/pulls/{n}/requested_reviewers`)
- [ ] Include the change summary in the reviewer notification message
- [ ] Suppress reviewer notification when inGitDB validation failed

**Acceptance:** A blocked PR triggers a review request to the correct reviewer(s). A PR with
validation errors does not trigger any notification.

---

### Phase 6: Merge and Conflict Resolution

**Goal:** Merge approved PRs; handle conflicts via `ingitdb resolve`.

- [ ] `server/github_app/merger`: attempt merge via GitHub API
  (`PUT /repos/{owner}/{repo}/pulls/{n}/merge`)
- [ ] On merge conflict: acquire a clone of the base branch from `repocache`, apply the PR diff,
  invoke `ingitdb resolve` to rebuild calculated files and apply merge policies on record files
- [ ] If `ingitdb resolve` succeeds: commit resolved files to the PR branch and re-attempt merge
- [ ] If `ingitdb resolve` cannot resolve all conflicts: post a structured PR comment with
  per-file conflict details; do not merge; notify reviewers
- [ ] On successful merge: post merge confirmation comment including the change summary

**Acceptance:** A conflict-free approved PR is merged automatically. A PR with resolvable
conflicts is resolved and merged. A PR with unresolvable conflicts receives a detailed comment
and is left open.

---

### Phase 7: Scheduled Scanner (Missed Webhook Safety Net)

**Goal:** Re-process open PRs missed due to webhook delivery failures.

- [ ] `POST /schedule/scan-open-prs` endpoint: list all open PRs; check whether each has a
  completed inGitDB Check Run; re-trigger the full Phase 2–6 flow for any PR without one
- [ ] Document wiring to Cloud Scheduler or a GitHub Actions scheduled workflow

**Acceptance:** After a server restart, all open PRs that missed their webhook are processed
within one scheduler interval.

---

## Testing Strategy

### Unit tests

Each package (`classifier`, `summary`, `permissions`, `reviewers`, `commenter`, `merger`) is
tested in isolation with mock implementations of the interfaces defined above. Extend the
existing mock infrastructure in `cmd/ingitdb/commands/mocks_test.go` as needed.

Coverage targets: all permission evaluation branches, subcollection inheritance edge cases,
reviewer scope resolution, comment rendering with empty/partial sections, notification
suppression on validation failure.

### Integration tests

A small set of integration tests runs against real GitHub API calls using a dedicated test
repository under the `ingitdb` org. Gated behind `//go:build integration`; require real
credentials in the environment. Not run in normal CI; run before releases and on a nightly
schedule.

### HTTP fixture recording

For tests that need realistic GitHub API responses without live credentials, use
[`go-vcr`](https://github.com/dnaeon/go-vcr) (cassette-based HTTP recording).

**How it works:**
1. On first run (`VCR_MODE=record` + real credentials), live GitHub API calls are made and all
   HTTP interactions are serialised to a YAML cassette file in `testdata/cassettes/`.
2. On subsequent runs (default), the HTTP client replays responses from the cassette. No network
   or credentials needed.
3. Cassette files are committed to the repository and updated deliberately when the API
   interaction changes.

**Usage pattern:**

```go
func TestPRValidationFlow(t *testing.T) {
    t.Parallel()
    r, err := recorder.New("testdata/cassettes/pr_validation_flow")
    if err != nil {
        t.Fatal(err)
    }
    defer r.Stop()

    httpClient := &http.Client{Transport: r}
    // inject httpClient into the GitHub API client under test
}
```

**Cassette hygiene:**
- Sensitive values (tokens, installation IDs) are scrubbed before commit using `go-vcr`'s
  `AddHook` with `AfterCaptureHook`.
- Cassette filenames mirror test names for easy discovery.
- A `make update-cassettes` target re-records all cassettes (requires integration credentials).

---

## Open Decisions

| # | Decision | Status |
|---|---|---|
| 1 | `repocache` eviction policy: LRU by size, TTL, or both? | Open |
| 2 | App posts a single updating comment or a new comment per event? | Lean toward update to avoid noise |
| 3 | Cassette storage: `testdata/cassettes/` per package or a shared top-level dir? | Open |
| 4 | How to handle PRs targeting non-default branches (e.g., release branches)? | Open |

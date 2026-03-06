# Dev Plan: Background Dependency Validation

**Feature spec:** [dependency-validation-feature.md](./dependency-validation-feature.md)

## Goals

Implement the background dependency validation feature of the inGitDB GitHub App. On a
configurable schedule, the App scans inGitDB records for broken cross-collection references and
external source references, then opens or updates a GitHub Issue with a structured violation
report.

---

## Dependencies

### On PR Auto-Merge infrastructure

This feature reuses infrastructure delivered in the
[PR Auto-Merge dev plan](./pr-auto-merge-dev-plan.md). The following must be complete first:

| Component | From PR Auto-Merge plan |
|---|---|
| `pkg/githubauth` — App token exchange | Phase 1 |
| `pkg/repocache` — repo cloning and cache | Phase 1 |
| `server/webhook` — HTTP server wiring | Phase 1 |

### On CLI capabilities

| Capability | Notes |
|---|---|
| `dalgo2ghingitdb` read via GitHub API | Record reading for all collections |
| Cross-collection reference schema annotations | `type: ref` with `ref_collection` in collection definitions |

---

## Architecture

### New packages

| Package | Responsibility |
|---|---|
| `server/github_app/depvalidator/` | Orchestrator: reads records, runs all validators, collects violations |
| `server/github_app/depvalidator/refcheck/` | Cross-collection reference validator |
| `server/github_app/depvalidator/extcheck/` | External source reference validator; one implementation per source type |
| `server/github_app/depvalidator/issuer/` | Creates, updates, and closes GitHub Issues with violation reports |

### Interfaces

```go
// Validator checks a single record against one rule and returns any violations.
type Validator interface {
    Validate(ctx context.Context, record dal.Record) ([]Violation, error)
}

// Violation describes a single failed check.
type Violation struct {
    CollectionID string
    RecordID     string
    Field        string
    Message      string
}

// ExternalSource checks whether an external ID is still valid.
type ExternalSource interface {
    Exists(ctx context.Context, id string) (bool, error)
}

// IssueManager opens, updates, or closes the validation report issue.
type IssueManager interface {
    Report(ctx context.Context, owner, repo string, violations []Violation) error
}
```

### External source implementations

| Type | Implementation |
|---|---|
| `github` | Uses App installation token; calls GitHub Issues or Repos API |
| `jira_cloud` | Calls Jira Cloud REST API with a bearer token from env |
| `http` | Generic: HTTP GET to a configured URL; 200 = exists, 404 = not found |

New source types can be added by implementing `ExternalSource` without touching the orchestrator.

---

## Implementation Phases

### Phase 1: Scheduler Endpoint and Orchestrator

**Goal:** Expose the schedule endpoint and wire the orchestrator skeleton.

- [ ] `POST /schedule/dependency-check` endpoint: parse repository target from request body or
  query params; invoke the orchestrator; return 200 on completion
- [ ] `server/github_app/depvalidator`: load `github_app.dependency_validation` config from
  `.ingitdb.yaml`; iterate over configured collections; call registered validators per record;
  collect all violations
- [ ] Document wiring to Cloud Scheduler and GitHub Actions scheduled workflow

**Acceptance:** Calling the endpoint against a repository with no violations returns 200 and
takes no further action.

---

### Phase 2: Cross-Collection Reference Validator

**Goal:** Detect records with field values that reference non-existent records in another
collection.

- [ ] `server/github_app/depvalidator/refcheck`: for each field annotated `type: ref`, read the
  referenced collection's record IDs and check that the field value exists among them
- [ ] Register `refcheck` as a `Validator` in the orchestrator
- [ ] Cache the referenced collection's record ID set per check run to avoid re-reading on each
  record

**Acceptance:** A record whose `author` field references a deleted user record is reported as a
violation. A record with a valid reference is not.

---

### Phase 3: External Source Reference Validator

**Goal:** Detect records referencing IDs in external systems that no longer exist.

- [ ] `server/github_app/depvalidator/extcheck`: for each field annotated `type: external_ref`,
  resolve the named source from config and call `ExternalSource.Exists`
- [ ] Implement `github` source type using the App installation token
- [ ] Implement `jira_cloud` source type (bearer token from env)
- [ ] Implement generic `http` source type (GET; 200 = exists, 404 = not found)
- [ ] Register `extcheck` as a `Validator` in the orchestrator

**Acceptance:** A record referencing a deleted Jira ticket is reported. A record referencing a
live ticket is not.

---

### Phase 4: Issue Management

**Goal:** Open, update, or close the GitHub Issue with the violation report.

- [ ] `server/github_app/depvalidator/issuer`: search for an existing open issue with the App's
  label (`ingitdb-validation`) in the repository
- [ ] If violations exist and no open issue: open a new issue using the report template defined
  in the [feature spec](./dependency-validation-feature.md)
- [ ] If violations exist and an issue is open: update the issue body with the latest report
- [ ] If no violations and an issue is open: post a closing comment and close the issue
- [ ] Render the report Markdown from `[]Violation`, grouped by collection

**Acceptance:** A first-run violation opens a new issue. A second run with the same violations
updates the existing issue body. A run with no violations closes the issue.

---

## Testing Strategy

### Unit tests

Each package (`refcheck`, `extcheck`, `issuer`) is tested in isolation with mock implementations
of `ExternalSource`, `IssueManager`, and `dal.DB`. Test cases cover: valid refs, missing refs,
external source returning 404, external source returning an error, issue open/update/close
transitions.

### Integration tests

Gated behind `//go:build integration`. Require a real test repository and, for external source
tests, a sandboxed Jira instance or a mock HTTP server fixture.

### HTTP fixture recording

Reuse the `go-vcr` cassette infrastructure established in the PR Auto-Merge dev plan. Cassettes
cover the GitHub Issues API calls (search, create, update, close) and a sample Jira API
response.

---

## Open Decisions

| # | Decision | Status |
|---|---|---|
| 1 | Should one issue cover all collections, or one issue per collection? | Lean toward one issue per repo for simplicity |
| 2 | Rate limiting for external source calls on large collections — batch or throttle? | Open |
| 3 | Should resolved violations be listed in the closing comment for auditability? | Open |
| 4 | Should the `http` generic source support authentication headers? | Open |

# Test Scenarios

Scenario-based functional tests for `ingitdb-cli`. Each scenario is a folder of Markdown step files that describe a complete workflow: what to run, what to expect, and how to verify it.

## What This Is

Scenarios are **executable prose** — human-readable procedures that are precise enough to run manually line by line, and structured enough to drive a future automated harness.

They serve as:
- **Acceptance criteria** for features not yet implemented (e.g. `ingitdb diff`)
- **Regression specs** that document expected behaviour once a feature ships
- **Onboarding material** for contributors implementing a command

## Directory Layout

```
docs/test/scenarios/
  README.md                  ← this file
  001-diff-basic/
    README.md                ← scenario overview, goal, full step table
    step-001.md              ← individual step
    step-002.md
    ...
```

Scenario folders: `NNN-short-kebab-name` (three-digit zero-padded prefix).
Step files: `step-NNN.md` (three-digit zero-padded). Gaps are allowed — do not renumber.

## Step File Structure

Each step file contains:

| Section | Contents |
|---------|----------|
| **Purpose** | One sentence: what this step proves or sets up |
| **Justification** | Why this step exists; how it differs from similar steps |
| **Requires** | _(only present when non-sequential)_ explicit list of steps that must have completed before this one can start |
| **Actions** | Exact shell commands to run, in order |
| **Assertions** | Commands to run + expected exit codes + expected output |
| **Notes** | Caveats, open questions, implementation dependencies |

### Step Sequencing

By default, steps run in order: each step assumes the previous one completed successfully.
Documenting this in every file is redundant — omit it.

Only add a `## Requires` section when a step depends on something other than its immediate
predecessor — for example, when a step can be run after any of several earlier steps, or
when a step skips ahead in the sequence. The `Requires` section lists step titles, not
numbers:

```markdown
## Requires

- Create feature branch
- Add Spain record
```

### Assertion Markers

Fenced code blocks inside Assertions carry HTML comment markers that a future test harness can parse:

| Marker | Meaning |
|--------|---------|
| `<!-- assert:exact -->` | Output must match byte-for-byte (SHA placeholders allowed) |
| `<!-- assert:contains -->` | Substring must appear anywhere in output |
| `<!-- assert:json-path -->` | Bulleted jq-style path checks on JSON output |

These markers render invisibly in GitHub Markdown.

### SHA Placeholders

Git commit SHAs are non-deterministic. Where a SHA appears in expected output, use the placeholder `<sha:XXXXXXX>` (seven chars). A harness replaces these with actual SHAs captured from earlier steps.

## Running a Scenario Manually

```shell
# 1. Create an isolated temp directory
mkdir -p /tmp/ingitdb-scenario-NNN && cd /tmp/ingitdb-scenario-NNN

# 2. Follow each step file in order
#    Run the commands under "Actions", then verify the "Assertions"

# 3. Clean up
cd / && rm -rf /tmp/ingitdb-scenario-NNN
```

No special tooling required — only `git`, `ingitdb`, and standard shell utilities.

## Scenario Index

| ID  | Folder           | Status | Feature        | Description |
|-----|------------------|--------|----------------|-------------|
| 001 | [001-diff-basic](001-diff-basic/README.md) | Draft | `ingitdb diff` | Diff at all depths, output formats, exit codes |

## Step Granularity Rules

Steps must cover **new ground**. A step is justified only if it exercises a distinct state
transition or assertion that no prior step covers.

**Acceptable — each step proves something different:**
- Add 1st record → proves adding to an empty collection works
- Add 2nd record → proves adding to a non-empty collection works

**Not acceptable — third step is identical in nature to the second:**
- Add 1st record
- Add 2nd record
- Add 3rd record ← redundant; use a batch step instead

When multiple items of the same kind are needed purely as data (not as test cases in
themselves), add them together in a single step:

- Add 2nd and 3rd record (batch) → one step, one commit, one assertion

This keeps scenarios focused and fast to read. Ask: _"Does this step prove something the
previous step of the same type did not?"_ If not, merge or batch it.

## Adding a New Scenario

1. Pick the next three-digit ID.
2. Create `NNN-short-name/README.md` with goal, DB setup summary, git state diagram, and step table.
3. Write `step-001.md` … `step-NNN.md`, applying the step granularity rules above.
4. Add a row to the index table above.
5. Mark status as **Draft** until the feature is implemented and the steps are verified to pass.

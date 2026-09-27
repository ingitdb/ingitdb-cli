# ingitdb validate — commit-range (fast CI)

Validate only records changed between two commits. Designed for CI on large databases where a full pass is too slow.

## Command

```bash
ingitdb validate --from-commit=<sha> --to-commit=<sha> [--path <dir>]
```

## Parameters

| Flag | Required | Description |
|---|---|---|
| `--from-commit` | Yes | Base commit SHA. Often `${{ github.event.pull_request.base.sha }}` in CI. |
| `--to-commit` | Yes | Tip commit SHA. Often `${{ github.event.pull_request.head.sha }}` or `HEAD`. |
| `--path` | No | Database directory. Defaults to current directory. |

## When to use

- **CI per pull request:** validate only what the PR changed.
- **Pre-merge gate:** skip records untouched by a branch.

## Notes

- Both SHAs must be reachable from the current working tree.
- Falls back to behaving like a full pass if the diff scope is empty — exit `0` with no output.

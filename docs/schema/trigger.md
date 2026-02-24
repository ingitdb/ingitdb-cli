# Trigger Definition File (`.ingitdb-collection/trigger_<name>.yaml`)

A trigger workflow runs shell commands in response to record lifecycle events in a collection. The syntax is modelled after [GitHub Actions](https://docs.github.com/en/actions/writing-workflows/workflow-syntax-for-github-actions) workflows.

## File location

Trigger definitions live inside the `.ingitdb-collection/` directory alongside view definitions:

```
<collection-dir>/
  .ingitdb-collection/
    trigger_<name>.yaml
```

The `<name>` part is a free-form identifier used to distinguish multiple triggers on the same collection.

## Top-level fields

| Field  | Type              | Required | Description                                              |
| ------ | ----------------- | -------- | -------------------------------------------------------- |
| `on`   | `[]string`        | yes      | List of events that fire this trigger (see values below) |
| `jobs` | `map[string]job`  | yes      | Map of job ID → job definition                           |

### `on` values

| Value     | Description                              |
| --------- | ---------------------------------------- |
| `created` | Fires when a record is created           |
| `updated` | Fires when a record is updated           |
| `deleted` | Fires when a record is deleted           |

## Job fields

Each entry in the `jobs` map is a job definition:

| Field      | Type       | Required | Description                                                       |
| ---------- | ---------- | -------- | ----------------------------------------------------------------- |
| `runs-on`  | `string`   | yes      | Execution environment. Only `"."` (same process as ingitdb) is supported |
| `steps`    | `[]step`   | yes      | Ordered list of steps to run                                      |

## Step fields

| Field | Type     | Required | Description                   |
| ----- | -------- | -------- | ----------------------------- |
| `run` | `string` | yes      | Shell command to execute       |

## Example

```yaml
on:
  - created
  - updated

jobs:
  build:
    runs-on: "."
    steps:
      - run: echo "Record changed, rebuilding site"
      - run: npm run generate

  notify:
    runs-on: "."
    steps:
      - run: curl -s -X POST "$SLACK_WEBHOOK_URL" -d '{"text":"Record updated"}'
```

See [Triggers](../components/triggers.md) for a conceptual overview.

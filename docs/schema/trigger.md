# ⚙️ Trigger Definition File (`.ingitdb-collection/trigger_<name>.yaml`)

Triggers are pluggable components that execute custom commands or HTTP requests when a given collection or document experiences specific data events.

## 📂 File location

Like views, trigger definitions are found within the `.ingitdb-collection/trigger_<name>.yaml` file. These can exist at the root collection level or within subcollection directories to provide finely tuned event-based routing.

## 📂 Top-level fields

| Field    | Type       | Description                                                                   |
| -------- | ---------- | ----------------------------------------------------------------------------- |
| `type`   | `string`   | The mechanism to fire: generally `webhook` or `shell`, defining execution.    |
| `name`   | `string`   | Display name of the active trigger handler.                                   |
| `on`     | `[]string` | RecordEvents to bind against (e.g., `insert`, `update`, `delete`, or `*`).    |
| `config` | `object`   | The mapping used depending entirely on the given `type` field representation. |

## 📂 Webhook Trigger Example

Creating an implicit REST notification webhook:

```yaml
type: webhook
name: NotifySlack
on:
  - insert
  - update
config:
  url: "https://hooks.slack.com/services/T00000000/B00000000/YOUR_WEBHOOK_URL"
  method: POST
```

## 📂 Shell Executable Trigger Example

Creating a command runner bound to the OS system:

```yaml
type: shell
name: BuildStaticSite
on:
  - "*"
config:
  command: "npm run generate"
```

## 📂 Custom Extensible Types

Triggers are simply interface models extending the `Trigger.Fire(ctx, event)` signature, allowing you to build or implement anything you'd like handling the generated system event streams.

See the [Triggers documentation](../components/triggers.md) for more details.

# Slacks

Posts a message to a Slack channel via an [incoming webhook](https://api.slack.com/messaging/webhooks).

## Fields

| Field         | Type     | Required | Description                |
| ------------- | -------- | -------- | -------------------------- |
| `name`        | `string` | no       | Label shown in logs        |
| `webhook_url` | `string` | yes      | Slack incoming webhook URL |

## YAML example

```yaml
subscribers:
  content-slack:
    name: "Notify content team on post/page changes"
    for:
      paths:
        - content/posts
        - content/pages
      events:
        - created
        - updated
    slacks:
      - name: Content team
        webhook_url: https://hooks.slack.com/services/<WORKSPACE_ID>/<CHANNEL_ID>/<WEBHOOK_TOKEN>
      - name: Editors channel
        webhook_url: https://hooks.slack.com/services/<WORKSPACE_ID>/<CHANNEL_ID>/<WEBHOOK_TOKEN_2>
```

# Discords

Posts a message to a Discord channel via a server webhook.

## Fields

| Field         | Type     | Required | Description         |
| ------------- | -------- | -------- | ------------------- |
| `name`        | `string` | no       | Label shown in logs |
| `webhook_url` | `string` | yes      | Discord webhook URL |

## YAML example

```yaml
subscribers:
  new-record-discord:
    for:
      events:
        - created
    discords:
      - name: Announcements
        webhook_url: https://discord.com/api/webhooks/000000000000000000/XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
```

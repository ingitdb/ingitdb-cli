# ntfy.sh

Sends a push notification via [ntfy.sh](https://ntfy.sh) — a simple, open-source, self-hostable notification service.

## Fields

| Field    | Type     | Required | Default           | Description                       |
| -------- | -------- | -------- | ----------------- | --------------------------------- |
| `name`   | `string` | no       |                   | Label shown in logs               |
| `topic`  | `string` | yes      |                   | ntfy topic name                   |
| `server` | `string` | no       | `https://ntfy.sh` | ntfy server URL (for self-hosted) |

## YAML example

```yaml
subscribers:
  push-notifications:
    for:
      events:
        - created
        - updated
    ntfy:
      - name: Public ntfy.sh
        topic: my-ingitdb-alerts
      - name: Internal server
        topic: ingitdb-prod
        server: https://ntfy.internal.example.com
```

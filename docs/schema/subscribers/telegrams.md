# Telegrams

Sends a message to a Telegram chat via the Bot API.

## Fields

| Field     | Type     | Required | Description                                                          |
| --------- | -------- | -------- | -------------------------------------------------------------------- |
| `name`    | `string` | no       | Label shown in logs                                                  |
| `token`   | `string` | yes      | Telegram Bot token (`123456:ABC-DEF...`)                             |
| `chat_id` | `string` | yes      | Target chat ID (group, channel, or user). Prefix with `-` for groups |

## YAML example

```yaml
subscribers:
  new-record-telegram:
    for:
      events:
        - created
    telegrams:
      - name: Ops chat
        token: "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"
        chat_id: "-1001234567890"
```

# WhatsApp

Sends a WhatsApp message via the WhatsApp Business API (Twilio or Meta Cloud API).

## Fields (Twilio)

| Field         | Type     | Required | Description                           |
| ------------- | -------- | -------- | ------------------------------------- |
| `name`        | `string` | no       | Label shown in logs                   |
| `from`        | `string` | yes      | Sender in `whatsapp:+E.164` format    |
| `to`          | `string` | yes      | Recipient in `whatsapp:+E.164` format |
| `account_sid` | `string` | yes      | Twilio Account SID                    |
| `auth_token`  | `string` | yes      | Twilio Auth Token                     |

## YAML example

```yaml
subscribers:
  oncall-whatsapp:
    for:
      events:
        - created
        - updated
    whatsapp:
      - name: On-call alert
        from: "whatsapp:+14155238886"
        to: "whatsapp:+15005550006"
        account_sid: ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
        auth_token: "<AUTH_TOKEN>"
```

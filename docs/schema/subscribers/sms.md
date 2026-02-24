# SMS

Sends an SMS via Twilio or Vonage. Useful for high-priority alerts.

## Fields

| Field         | Type     | Required     | Description                            |
| ------------- | -------- | ------------ | -------------------------------------- |
| `name`        | `string` | no           | Label shown in logs                    |
| `provider`    | `string` | yes          | `twilio` or `vonage`                   |
| `from`        | `string` | yes          | Sender phone number in E.164 format    |
| `to`          | `string` | yes          | Recipient phone number in E.164 format |
| `account_sid` | `string` | yes (Twilio) | Twilio Account SID                     |
| `auth_token`  | `string` | yes          | Twilio Auth Token or Vonage API secret |
| `api_key`     | `string` | yes (Vonage) | Vonage API key                         |

## YAML example

```yaml
subscribers:
  oncall-sms:
    for:
      events:
        - created
    sms:
      - name: Primary (Twilio)
        provider: twilio
        from: "+15005550006"
        to: "+14155238886"
        account_sid: ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
        auth_token: "<AUTH_TOKEN>"

      - name: Fallback (Vonage)
        provider: vonage
        from: "+15005550006"
        to: "+14155238886"
        api_key: "<VONAGE_API_KEY>"
        auth_token: "<VONAGE_API_SECRET>"
```

# Emails

Sends an email notification via SMTP.

## Fields

| Field     | Type       | Required | Description                                                                     |
| --------- | ---------- | -------- | ------------------------------------------------------------------------------- |
| `name`    | `string`   | no       | Label shown in logs                                                             |
| `from`    | `string`   | no       | Sender address (defaults to the SMTP `user`)                                    |
| `to`      | `[]string` | yes      | Recipient addresses                                                             |
| `smtp`    | `string`   | yes      | SMTP server hostname                                                            |
| `port`    | `int`      | no       | SMTP port (default: `587`)                                                      |
| `user`    | `string`   | no       | SMTP username                                                                   |
| `pass`    | `string`   | no       | SMTP password                                                                   |
| `subject` | `string`   | no       | Email subject line. Supports [template variables](README.md#template-variables) |

## YAML example

```yaml
subscribers:
  department-emails:
    name: "Alert teams on department changes"
    for:
      paths:
        - companies/*/departments
      events:
        - created
        - updated
    emails:
      - name: HR team
        to:
          - hr@example.com
        smtp: smtp.example.com
        user: ingitdb@example.com
        pass: "<SMTP_PASSWORD>"

      - name: Ops team
        from: alerts@example.com
        to:
          - ops@example.com
          - oncall@example.com
        smtp: smtp.example.com
        user: alerts@example.com
        pass: "<SMTP_PASSWORD>"
        subject: "{event} on {path}"
```

# Webhooks

Issues an HTTP POST request when a record event fires. Events are batched per request: one call may carry changes from multiple collections.

## Fields

| Field     | Type     | Required | Default | Description                                    |
| --------- | -------- | -------- | ------- | ---------------------------------------------- |
| `name`    | `string` | no       |         | Label shown in logs                            |
| `url`     | `string` | yes      |         | Target URL                                     |
| `method`  | `string` | no       | `POST`  | HTTP method                                    |
| `headers` | `map`    | no       |         | Additional HTTP headers (e.g. `Authorization`) |

## YAML example

```yaml
subscribers:
  all-changes:
    name: "Notify backend on all changes"
    for:
      events:
        - created
        - updated
        - deleted
    webhooks:
      - name: Primary endpoint
        url: https://api.example.com/ingitdb-webhooks/data-change
        headers:
          Authorization: "Bearer <TOKEN>"
      - name: Audit log
        url: https://audit.example.com/ingest
```

With path filtering:

```yaml
subscribers:
  company-structure:
    name: "Department and office changes"
    for:
      paths:
        - companies/*/departments
        - companies/*/offices
      events:
        - created
        - updated
    webhooks:
      - url: https://example.com/ingitdb-webhooks/data-change
        headers:
          Authorization: "Bearer <TOKEN>"
```

Regex pattern — offices whose ID starts with `D` or `L` under acme-inc:

```yaml
subscribers:
  acme-dl-offices:
    name: "Acme-inc D/L offices and subcollections"
    for:
      paths:
        - companies/acme-inc/offices/[DL].+
        - companies/acme-inc/offices/[DL].+/*
    webhooks:
      - url: https://example.com/ingitdb-webhooks/offices
        headers:
          Authorization: "Bearer <TOKEN>"
```

## HTTP request

ingitdb sends a single `POST` request with a JSON body. Events are grouped by collection path.

```
POST https://example.com/ingitdb-webhooks/data-change
Content-Type: application/json
Authorization: Bearer <TOKEN>
```

```json
[
  {
    "collection": "companies/acme-inc/departments",
    "events": [
      {
        "event": "created",
        "id": "engineering",
        "data": {
          "name": "Engineering",
          "head": "jane.doe"
        }
      },
      {
        "event": "updated",
        "id": "marketing",
        "data": {
          "name": "Marketing",
          "head": "john.smith"
        }
      }
    ]
  },
  {
    "collection": "companies/acme-inc/employees",
    "events": [
      {
        "event": "deleted",
        "id": "former-employee",
        "data": null
      }
    ]
  }
]
```

## Event object fields

| Field   | Type     | Description                                             |
| ------- | -------- | ------------------------------------------------------- |
| `event` | `string` | One of `created`, `updated`, `deleted`                  |
| `id`    | `string` | Record key                                              |
| `data`  | `object` | Full record data after the change; `null` for `deleted` |

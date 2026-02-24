# Subscriber Definition File (`.ingitdb/subscribers.yaml`)

Subscribers are built-in, configurable event handlers that react to record lifecycle events (`created`, `updated`, `deleted`). Unlike [triggers](../trigger.md) (which execute arbitrary shell commands), subscribers are first-class integrations with zero external tooling required.

See [Subscribers feature overview](../../features/subscribers.md) for a conceptual introduction.

## File location

```
<database-root>/
  .ingitdb/
    subscribers.yaml
```

## Top-level structure

The file contains a single `subscribers` map. Each key is a **unique ID** that identifies the subscriber group — useful for targeting specific entries when adding paths, modifying events, or disabling a group. The value is a **subscriber definition** ([`SubscriberDef`](../../../pkg/ingitdb/subscriber_def.go)) pairing a `for` selector with one or more handler lists.

```yaml
subscribers:
  <id>:
    name: <optional display name>
    for:
      paths:
        - <path-pattern>
      events:
        - created
        - updated
        - deleted
    webhooks:
      - name: <optional label>
        url: <url>
    emails:
      - to: [<address>]
```

## Subscriber entry fields

| Field         | Type     | Required | Description                                            |
| ------------- | -------- | -------- | ------------------------------------------------------ |
| `name`        | `string` | no       | Human-readable description of this subscriber group    |
| `for`         | `object` | yes      | Selector — which paths and events trigger the handlers |
| handler types | —        | yes      | At least one handler list (e.g. `webhooks`, `emails`)  |

## `for` selector fields

| Field    | Type       | Required | Description                                                                           |
| -------- | ---------- | -------- | ------------------------------------------------------------------------------------- |
| `paths`  | `[]string` | no       | Path patterns to watch. Omit (or use `['*']`) to match all paths                      |
| `events` | `[]string` | no       | Events that fire the handlers. Defaults to all three: `created`, `updated`, `deleted` |

### `events` values

| Value     | Description                    |
| --------- | ------------------------------ |
| `created` | Fires when a record is created |
| `updated` | Fires when a record is updated |
| `deleted` | Fires when a record is deleted |

---

## Path patterns

The `paths` field accepts a list of path patterns. A path pattern is a `/`-separated string. Each segment may be a **literal**, `*`, or a **regex without `/` characters**.

| Segment syntax    | Matches                                                 |
| ----------------- | ------------------------------------------------------- |
| `literal`         | Exactly that collection name or record ID               |
| `*`               | Any single ID — shorthand for the regex `.*`            |
| `[A-Z].+` (regex) | Any ID matching the regular expression (no `/` allowed) |

The handlers fire when the path of the changed record matches **any** of the listed patterns.

### Pattern examples

| Pattern                               | Watches                                                               |
| ------------------------------------- | --------------------------------------------------------------------- |
| `*`                                   | Every record in the entire database                                   |
| `companies/*/departments`             | All records in the `departments` subcollection under any company      |
| `companies/*/offices`                 | All records in the `offices` subcollection under any company          |
| `companies/*/offices/dublin`          | The `dublin` record in `offices` under any company                    |
| `companies/*/departments/*/*`         | All records in any subcollection of any department                    |
| `companies/acme-inc/offices/[DL].+`   | Records in `offices` under `acme-inc` whose ID starts with `D` or `L` |
| `companies/acme-inc/offices/[DL].+/*` | All subcollection records under those matched offices                 |

---

## Template variables

Text fields that accept templates (such as email `subject`) may reference these variables using `{variable}` syntax:

| Variable       | Value                                                   |
| -------------- | ------------------------------------------------------- |
| `{event}`      | Event type: `created`, `updated`, or `deleted`          |
| `{key}`        | Record ID                                               |
| `{collection}` | Collection path (e.g. `companies/acme-inc/departments`) |
| `{path}`       | Full record path — collection + key                     |

Example: `subject: "Record {event}: {path}"`

---

## Handler types

Each handler entry may include an optional `name` field (a free-form label shown in logs). All other fields are type-specific.

| Handler key                           | Implementation Type                                         | Description                                       |
| ------------------------------------- | ----------------------------------------------------------- | ------------------------------------------------- |
| [`webhooks`](webhooks.md)             | [`WebhookDef`](../../../pkg/ingitdb/subscriber_def.go)      | HTTP POST to any URL                              |
| [`emails`](emails.md)                 | [`EmailDef`](../../../pkg/ingitdb/subscriber_def.go)        | SMTP email notification                           |
| [`telegrams`](telegrams.md)           | [`TelegramDef`](../../../pkg/ingitdb/subscriber_def.go)     | Telegram Bot API message                          |
| [`whatsapp`](whatsapp.md)             | [`WhatsAppDef`](../../../pkg/ingitdb/subscriber_def.go)     | WhatsApp Business API message                     |
| [`slacks`](slacks.md)                 | [`SlackDef`](../../../pkg/ingitdb/subscriber_def.go)        | Slack incoming webhook                            |
| [`discords`](discords.md)             | [`DiscordDef`](../../../pkg/ingitdb/subscriber_def.go)      | Discord channel webhook                           |
| [`github_actions`](github_actions.md) | [`GitHubActionDef`](../../../pkg/ingitdb/subscriber_def.go) | Trigger a GitHub Actions `workflow_dispatch`      |
| [`gitlab_ci`](gitlab_ci.md)           | [`GitLabCIDef`](../../../pkg/ingitdb/subscriber_def.go)     | Trigger a GitLab pipeline                         |
| [`ntfy`](ntfy.md)                     | [`NtfyDef`](../../../pkg/ingitdb/subscriber_def.go)         | Push notification via ntfy.sh                     |
| [`sms`](sms.md)                       | [`SMSDef`](../../../pkg/ingitdb/subscriber_def.go)          | SMS via Twilio or Vonage                          |
| [`search_indexes`](search_indexes.md) | [`SearchIndexDef`](../../../pkg/ingitdb/subscriber_def.go)  | Push changes to Algolia / Meilisearch / Typesense |
| [`rss`](rss.md)                       | [`RSSDef`](../../../pkg/ingitdb/subscriber_def.go)          | Regenerate an RSS or Atom feed file               |

---

## Full example

`.ingitdb/subscribers.yaml` combining multiple subscriber groups:

```yaml
subscribers:
  all-changes:
    name: "Notify backend and rebuild site on any change"
    for:
      events:
        - created
        - updated
        - deleted
    webhooks:
      - name: Backend API
        url: https://api.example.com/ingitdb-webhooks/data-change
        headers:
          Authorization: "Bearer <TOKEN>"
      - name: Audit log
        url: https://audit.example.com/ingest
    github_actions:
      - name: Deploy site
        owner: my-org
        repo: my-site
        workflow: deploy.yml
        ref: main
        token: ghp_XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX

  content-updates:
    name: "Content team notifications and search sync"
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
    search_indexes:
      - name: Algolia
        provider: algolia
        app_id: XXXXXXXXXX
        api_key: "<ALGOLIA_WRITE_API_KEY>"
        index: content
    rss:
      - name: Blog RSS
        output: public/feed.xml
        title: "My Blog"
        link: https://example.com
        format: rss2

  company-structure:
    name: "Alert HR and Ops on department/office changes"
    for:
      paths:
        - companies/*/departments
        - companies/*/offices
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
        subject: "{event} on {path}"
      - name: Ops team
        to:
          - ops@example.com
        smtp: smtp.example.com
        user: ingitdb@example.com
        pass: "<SMTP_PASSWORD>"

  acme-dl-offices:
    name: "Acme-inc offices starting with D or L"
    for:
      paths:
        - companies/acme-inc/offices/[DL].+
        - companies/acme-inc/offices/[DL].+/*
    webhooks:
      - name: Acme offices hook
        url: https://api.example.com/ingitdb-webhooks/offices
        headers:
          Authorization: "Bearer <TOKEN>"
    sms:
      - name: On-call
        provider: twilio
        from: "+15005550006"
        to: "+14155238886"
        account_sid: ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
        auth_token: "<AUTH_TOKEN>"
```

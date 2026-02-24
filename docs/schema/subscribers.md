# Subscriber Definition File (`.ingitdb/subscribers.yaml`)

Subscribers are built-in, configurable event handlers that react to record lifecycle events (`created`, `updated`, `deleted`). Unlike [triggers](trigger.md) (which execute arbitrary shell commands), subscribers are first-class integrations with zero external tooling required.

See [Subscribers feature overview](../features/subscribers.md) for a conceptual introduction.

## File location

```
<database-root>/
  .ingitdb/
    subscribers.yaml
```

## Top-level structure

The file contains a single `subscribers` map. Each key is a **unique ID** that identifies the subscriber group — useful for targeting specific entries when adding paths, modifying events, or disabling a group. The value is a **subscriber definition** ([`SubscriberDef`](../../pkg/ingitdb/subscriber_def.go)) pairing a `for` selector with one or more handler lists.

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

| Handler key                            | Implementation Type                                      | Description                                       |
| -------------------------------------- | -------------------------------------------------------- | ------------------------------------------------- |
| [`webhooks`](#webhooks)                | [`WebhookDef`](../../pkg/ingitdb/subscriber_def.go)      | HTTP POST to any URL                              |
| [`emails`](#emails)                    | [`EmailDef`](../../pkg/ingitdb/subscriber_def.go)        | SMTP email notification                           |
| [`telegrams`](#telegrams)              | [`TelegramDef`](../../pkg/ingitdb/subscriber_def.go)     | Telegram Bot API message                          |
| [`whatsapp`](#whatsapp)                | [`WhatsAppDef`](../../pkg/ingitdb/subscriber_def.go)     | WhatsApp Business API message                     |
| [`slacks`](#slacks)                    | [`SlackDef`](../../pkg/ingitdb/subscriber_def.go)        | Slack incoming webhook                            |
| [`discords`](#discords)                | [`DiscordDef`](../../pkg/ingitdb/subscriber_def.go)      | Discord channel webhook                           |
| [`github_actions`](#github-actions)    | [`GitHubActionDef`](../../pkg/ingitdb/subscriber_def.go) | Trigger a GitHub Actions `workflow_dispatch`      |
| [`gitlab_ci`](#gitlab-ci)              | [`GitLabCIDef`](../../pkg/ingitdb/subscriber_def.go)     | Trigger a GitLab pipeline                         |
| [`ntfy`](#ntfysh)                      | [`NtfyDef`](../../pkg/ingitdb/subscriber_def.go)         | Push notification via ntfy.sh                     |
| [`sms`](#sms)                          | [`SMSDef`](../../pkg/ingitdb/subscriber_def.go)          | SMS via Twilio or Vonage                          |
| [`search_indexes`](#search-index-sync) | [`SearchIndexDef`](../../pkg/ingitdb/subscriber_def.go)  | Push changes to Algolia / Meilisearch / Typesense |
| [`rss`](#rssatom-feed)                 | [`RSSDef`](../../pkg/ingitdb/subscriber_def.go)          | Regenerate an RSS or Atom feed file               |

---

## Webhooks

Issues an HTTP POST request when a record event fires. Events are batched per request: one call may carry changes from multiple collections.

### Fields

| Field     | Type     | Required | Default | Description                                    |
| --------- | -------- | -------- | ------- | ---------------------------------------------- |
| `name`    | `string` | no       |         | Label shown in logs                            |
| `url`     | `string` | yes      |         | Target URL                                     |
| `method`  | `string` | no       | `POST`  | HTTP method                                    |
| `headers` | `map`    | no       |         | Additional HTTP headers (e.g. `Authorization`) |

### YAML example

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

### HTTP request

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

### Event object fields

| Field   | Type     | Description                                             |
| ------- | -------- | ------------------------------------------------------- |
| `event` | `string` | One of `created`, `updated`, `deleted`                  |
| `id`    | `string` | Record key                                              |
| `data`  | `object` | Full record data after the change; `null` for `deleted` |

---

## Emails

Sends an email notification via SMTP.

### Fields

| Field     | Type       | Required | Description                                                            |
| --------- | ---------- | -------- | ---------------------------------------------------------------------- |
| `name`    | `string`   | no       | Label shown in logs                                                    |
| `from`    | `string`   | no       | Sender address (defaults to the SMTP `user`)                           |
| `to`      | `[]string` | yes      | Recipient addresses                                                    |
| `smtp`    | `string`   | yes      | SMTP server hostname                                                   |
| `port`    | `int`      | no       | SMTP port (default: `587`)                                             |
| `user`    | `string`   | no       | SMTP username                                                          |
| `pass`    | `string`   | no       | SMTP password                                                          |
| `subject` | `string`   | no       | Email subject line. Supports [template variables](#template-variables) |

### YAML example

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

---

## Telegrams

Sends a message to a Telegram chat via the Bot API.

### Fields

| Field     | Type     | Required | Description                                                          |
| --------- | -------- | -------- | -------------------------------------------------------------------- |
| `name`    | `string` | no       | Label shown in logs                                                  |
| `token`   | `string` | yes      | Telegram Bot token (`123456:ABC-DEF...`)                             |
| `chat_id` | `string` | yes      | Target chat ID (group, channel, or user). Prefix with `-` for groups |

### YAML example

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

---

## WhatsApp

Sends a WhatsApp message via the WhatsApp Business API (Twilio or Meta Cloud API).

### Fields (Twilio)

| Field         | Type     | Required | Description                           |
| ------------- | -------- | -------- | ------------------------------------- |
| `name`        | `string` | no       | Label shown in logs                   |
| `from`        | `string` | yes      | Sender in `whatsapp:+E.164` format    |
| `to`          | `string` | yes      | Recipient in `whatsapp:+E.164` format |
| `account_sid` | `string` | yes      | Twilio Account SID                    |
| `auth_token`  | `string` | yes      | Twilio Auth Token                     |

### YAML example

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

---

## Slacks

Posts a message to a Slack channel via an [incoming webhook](https://api.slack.com/messaging/webhooks).

### Fields

| Field         | Type     | Required | Description                |
| ------------- | -------- | -------- | -------------------------- |
| `name`        | `string` | no       | Label shown in logs        |
| `webhook_url` | `string` | yes      | Slack incoming webhook URL |

### YAML example

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

---

## Discords

Posts a message to a Discord channel via a server webhook.

### Fields

| Field         | Type     | Required | Description         |
| ------------- | -------- | -------- | ------------------- |
| `name`        | `string` | no       | Label shown in logs |
| `webhook_url` | `string` | yes      | Discord webhook URL |

### YAML example

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

---

## GitHub Actions

Triggers a [`workflow_dispatch`](https://docs.github.com/en/actions/writing-workflows/choosing-when-your-workflow-runs/events-that-trigger-workflows#workflow_dispatch) event on a GitHub Actions workflow. Useful for rebuilding a static site or running a deployment pipeline when records change.

### Fields

| Field      | Type     | Required | Description                              |
| ---------- | -------- | -------- | ---------------------------------------- |
| `name`     | `string` | no       | Label shown in logs                      |
| `owner`    | `string` | yes      | GitHub organisation or user name         |
| `repo`     | `string` | yes      | Repository name                          |
| `workflow` | `string` | yes      | Workflow file name (e.g. `deploy.yml`)   |
| `ref`      | `string` | yes      | Branch or tag to run the workflow on     |
| `token`    | `string` | yes      | GitHub Personal Access Token (`ghp_...`) |

### YAML example

```yaml
subscribers:
  deploy-site:
    name: "Rebuild and deploy site on any change"
    for:
      events:
        - created
        - updated
        - deleted
    github_actions:
      - name: Deploy
        owner: my-org
        repo: my-site
        workflow: deploy.yml
        ref: main
        token: ghp_XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
```

---

## GitLab CI

Triggers a GitLab pipeline via the [pipeline trigger API](https://docs.gitlab.com/ee/ci/triggers/).

### Fields

| Field        | Type     | Required | Description                          |
| ------------ | -------- | -------- | ------------------------------------ |
| `name`       | `string` | no       | Label shown in logs                  |
| `project_id` | `string` | yes      | GitLab project ID (numeric string)   |
| `ref`        | `string` | yes      | Branch or tag to run the pipeline on |
| `token`      | `string` | yes      | GitLab pipeline trigger token        |
| `host`       | `string` | no       | GitLab host (default: `gitlab.com`)  |

### YAML example

```yaml
subscribers:
  deploy-pipeline:
    for:
      events:
        - created
        - updated
        - deleted
    gitlab_ci:
      - name: Deploy pipeline
        project_id: "12345678"
        ref: main
        token: glptt-XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
```

---

## ntfy.sh

Sends a push notification via [ntfy.sh](https://ntfy.sh) — a simple, open-source, self-hostable notification service.

### Fields

| Field    | Type     | Required | Default           | Description                       |
| -------- | -------- | -------- | ----------------- | --------------------------------- |
| `name`   | `string` | no       |                   | Label shown in logs               |
| `topic`  | `string` | yes      |                   | ntfy topic name                   |
| `server` | `string` | no       | `https://ntfy.sh` | ntfy server URL (for self-hosted) |

### YAML example

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

---

## SMS

Sends an SMS via Twilio or Vonage. Useful for high-priority alerts.

### Fields

| Field         | Type     | Required     | Description                            |
| ------------- | -------- | ------------ | -------------------------------------- |
| `name`        | `string` | no           | Label shown in logs                    |
| `provider`    | `string` | yes          | `twilio` or `vonage`                   |
| `from`        | `string` | yes          | Sender phone number in E.164 format    |
| `to`          | `string` | yes          | Recipient phone number in E.164 format |
| `account_sid` | `string` | yes (Twilio) | Twilio Account SID                     |
| `auth_token`  | `string` | yes          | Twilio Auth Token or Vonage API secret |
| `api_key`     | `string` | yes (Vonage) | Vonage API key                         |

### YAML example

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

---

## Search index sync

Pushes record changes to a full-text search index. Useful for content-management databases powering a search UI.

### Fields

| Field      | Type     | Required                      | Description                              |
| ---------- | -------- | ----------------------------- | ---------------------------------------- |
| `name`     | `string` | no                            | Label shown in logs                      |
| `provider` | `string` | yes                           | `algolia`, `meilisearch`, or `typesense` |
| `index`    | `string` | yes                           | Target index name                        |
| `app_id`   | `string` | yes (Algolia)                 | Algolia application ID                   |
| `api_key`  | `string` | yes                           | Provider API key                         |
| `host`     | `string` | yes (Meilisearch / Typesense) | Server base URL                          |

### YAML example

```yaml
subscribers:
  content-search-sync:
    name: "Keep search indexes in sync with content"
    for:
      paths:
        - content/posts
        - content/pages
    search_indexes:
      - name: Algolia
        provider: algolia
        app_id: XXXXXXXXXX
        api_key: "<ALGOLIA_WRITE_API_KEY>"
        index: content

      - name: Meilisearch (self-hosted)
        provider: meilisearch
        host: http://localhost:7700
        api_key: "<MEILISEARCH_MASTER_KEY>"
        index: content
```

---

## RSS/Atom feed

Regenerates an RSS or Atom feed file whenever records are created or updated. Intended for content collections (blog posts, changelogs, etc.).

### Fields

| Field    | Type     | Required | Default | Description                                  |
| -------- | -------- | -------- | ------- | -------------------------------------------- |
| `name`   | `string` | no       |         | Label shown in logs                          |
| `output` | `string` | yes      |         | Output file path (relative to database root) |
| `title`  | `string` | yes      |         | Feed title                                   |
| `link`   | `string` | yes      |         | Canonical URL of the site                    |
| `format` | `string` | no       | `rss2`  | Feed format: `rss2` or `atom`                |

### YAML example

```yaml
subscribers:
  blog-feeds:
    for:
      paths:
        - content/posts
      events:
        - created
        - updated
    rss:
      - name: RSS 2.0
        output: public/feed.xml
        title: "My Blog"
        link: https://example.com
        format: rss2
      - name: Atom
        output: public/atom.xml
        title: "My Blog"
        link: https://example.com
        format: atom
```

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

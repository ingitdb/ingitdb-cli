# Subscribers – built-in event handlers

Subscribers are built-in, configurable event handlers that run inside the ingitdb CLI process and react to record lifecycle events (`created`, `updated`, `deleted`). Unlike [triggers](../schema/trigger.md) (which execute arbitrary shell commands), subscribers are first-class integrations with zero external tooling required.

Configuration lives in `.ingitdb/subscribers.yaml` at the root of your database directory. See the [Subscriber schema reference](../schema/subscribers/) for the full YAML specification.

## Structure

`subscribers` is a map of uniquely-identified groups. Each group has a `for` selector (which paths and events to watch) and one or more handler lists:

```yaml
subscribers:
  my-group:
    name: "Optional description"
    for:
      paths:
        - companies/*/departments # path pattern — omit to match all
      events:
        - created
        - updated
        - deleted
    webhooks:
      - url: https://example.com/hook
    emails:
      - to: [alice@example.com]
        smtp: smtp.example.com
```

Using map keys (e.g. `my-group`) makes it easy to target a specific group when adding paths, changing events, or extending its handlers.

## Subscriber catalogue

| Subscriber                              | Status   | Description                                              |
| --------------------------------------- | -------- | -------------------------------------------------------- |
| [Webhook](#webhook)                     | planned  | HTTP POST to any URL                                     |
| [Email](#email)                         | planned  | SMTP email notification                                  |
| [Telegram](#telegram)                   | planned  | Send a Telegram message                                  |
| [WhatsApp](#whatsapp)                   | proposal | Send a WhatsApp message                                  |
| [Slack](#slack)                         | proposal | Post to a Slack channel via incoming webhook             |
| [Discord](#discord)                     | proposal | Post to a Discord channel via webhook                    |
| [GitHub Actions](#github-actions)       | proposal | Trigger a `workflow_dispatch` event                      |
| [GitLab CI](#gitlab-ci)                 | proposal | Trigger a GitLab pipeline                                |
| [ntfy.sh](#ntfysh)                      | proposal | Send a push notification via ntfy.sh (self-hostable)     |
| [SMS](#sms)                             | proposal | Send an SMS via Twilio or Vonage                         |
| [Search index sync](#search-index-sync) | proposal | Push record changes to Algolia / Meilisearch / Typesense |
| [RSS/Atom feed](#rssatom-feed)          | proposal | Regenerate an RSS or Atom feed file                      |

---

## Webhook

Issues an HTTP POST to a URL when a record event fires. Multiple webhooks can be listed under the same `for` selector.

```yaml
subscribers:
  notify-backend:
    for:
      events:
        - created
        - updated
        - deleted
    webhooks:
      - name: Primary endpoint
        url: https://example.com/ingitdb-webhooks/data-change
        headers:
          Authorization: "Bearer <TOKEN>"
```

---

## Email

Sends an email notification via SMTP.

```yaml
subscribers:
  department-emails:
    for:
      paths:
        - companies/*/departments
      events:
        - created
        - updated
    emails:
      - name: HR team
        to:
          - alice@example.com
        smtp: smtp.example.com
        user: ingitdb@example.com
        pass: "<SMTP_PASSWORD>"
      - name: Ops team
        to:
          - bob@example.com
        smtp: smtp.example.com
        user: ingitdb@example.com
        pass: "<SMTP_PASSWORD>"
        subject: "{event} on {path}"
```

---

## Telegram

Sends a message to a Telegram chat via the Bot API.

```yaml
subscribers:
  new-record-telegram:
    for:
      events:
        - created
    telegrams:
      - token: "123456:ABC-DEF..."
        chat_id: "-1001234567890"
```

---

## WhatsApp

Sends a WhatsApp message via the WhatsApp Business API (e.g. Twilio or Meta Cloud API).

```yaml
subscribers:
  oncall-whatsapp:
    for:
      events:
        - created
        - updated
    whatsapp:
      - from: "whatsapp:+14155238886"
        to: "whatsapp:+15005550006"
        account_sid: ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX # Twilio
        auth_token: "<AUTH_TOKEN>"
```

---

## Slack

Posts a message to a Slack channel via an [incoming webhook](https://api.slack.com/messaging/webhooks).

```yaml
subscribers:
  content-slack:
    for:
      paths:
        - content/posts
        - content/pages
      events:
        - created
        - updated
    slacks:
      - webhook_url: https://hooks.slack.com/services/<WORKSPACE_ID>/<CHANNEL_ID>/<WEBHOOK_TOKEN>
```

---

## Discord

Posts a message to a Discord channel via a server webhook.

```yaml
subscribers:
  new-record-discord:
    for:
      events:
        - created
    discords:
      - webhook_url: https://discord.com/api/webhooks/000000000/XXXX
```

---

## GitHub Actions

Triggers a [`workflow_dispatch`](https://docs.github.com/en/actions/writing-workflows/choosing-when-your-workflow-runs/events-that-trigger-workflows#workflow_dispatch) event on a GitHub Actions workflow. Useful for rebuilding a static site or running a pipeline when records change.

```yaml
subscribers:
  deploy-site:
    for:
      events:
        - created
        - updated
        - deleted
    github_actions:
      - owner: my-org
        repo: my-site
        workflow: deploy.yml
        ref: main
        token: ghp_XXXX
```

---

## GitLab CI

Triggers a GitLab pipeline via the [pipeline trigger API](https://docs.gitlab.com/ee/ci/triggers/).

```yaml
subscribers:
  deploy-pipeline:
    for:
      events:
        - created
        - updated
        - deleted
    gitlab_ci:
      - project_id: "12345678"
        ref: main
        token: glptt-XXXX
```

---

## ntfy.sh

Sends a push notification via [ntfy.sh](https://ntfy.sh) — a simple, open-source, self-hostable notification service.

```yaml
subscribers:
  push-notifications:
    for:
      events:
        - created
        - updated
    ntfy:
      - topic: my-ingitdb-alerts
        server: https://ntfy.sh # optional, defaults to ntfy.sh
```

---

## SMS

Sends an SMS via Twilio or Vonage. Useful for high-priority record alerts.

```yaml
subscribers:
  oncall-sms:
    for:
      events:
        - created
    sms:
      - provider: twilio # or: vonage
        from: "+15005550006"
        to: "+14155238886"
        account_sid: ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
        auth_token: "<AUTH_TOKEN>"
```

---

## Search index sync

Pushes record changes to a search index. Useful for content-management databases powering a search UI.

```yaml
subscribers:
  content-search-sync:
    for:
      paths:
        - content/posts
        - content/pages
    search_indexes:
      - provider: algolia # or: meilisearch, typesense
        app_id: XXXXXXXXXX
        api_key: "<API_KEY>"
        index: content

      - provider: meilisearch
        host: http://localhost:7700
        api_key: "<API_KEY>"
        index: content
```

---

## RSS/Atom feed

Regenerates an RSS or Atom feed file whenever records are created or updated. Intended for content collections (blog posts, changelogs, etc.).

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
      - output: public/feed.xml
        title: "My Blog"
        link: https://example.com
        format: rss2 # or: atom
```

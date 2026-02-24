# Subscribers – built-in event handlers

Subscribers are built-in, configurable event handlers that run inside the ingitdb CLI process and react to record lifecycle events (`created`, `updated`, `deleted`). Unlike [triggers](../schema/trigger.md) (which execute arbitrary shell commands), subscribers are first-class integrations with zero external tooling required.

Configuration lives in `.ingitdb/subscribers.yaml` at the root of your database directory.

## Subscriber catalogue

| Subscriber | Status | Description |
|---|---|---|
| [Webhook](#webhook) | planned | HTTP POST to any URL |
| [Email](#email) | planned | SMTP email notification |
| [Telegram](#telegram) | planned | Send a Telegram message |
| [WhatsApp](#whatsapp) | proposal | Send a WhatsApp message |
| [Slack](#slack) | proposal | Post to a Slack channel via incoming webhook |
| [Discord](#discord) | proposal | Post to a Discord channel via webhook |
| [GitHub Actions](#github-actions) | proposal | Trigger a `workflow_dispatch` event |
| [GitLab CI](#gitlab-ci) | proposal | Trigger a GitLab pipeline |
| [ntfy.sh](#ntfysh) | proposal | Send a push notification via ntfy.sh (self-hostable) |
| [SMS](#sms) | proposal | Send an SMS via Twilio or Vonage |
| [Search index sync](#search-index-sync) | proposal | Push record changes to Algolia / Meilisearch / Typesense |
| [RSS/Atom feed](#rssatom-feed) | proposal | Regenerate an RSS or Atom feed file |

---

## Webhook

Issues an HTTP request when a record event fires.

```yaml
subscribers:
  - name: Notify external service
    webhook:
      url: https://example.com/webhook/
      method: POST          # default: POST
```

---

## Email

Sends an email notification via SMTP.

```yaml
subscribers:
  - name: Alert team
    email:
      from: ingitdb@example.com
      to:
        - alice@example.com
        - bob@example.com
      smtp: smtp.example.com
```

---

## Telegram

Sends a message to a Telegram chat via the Bot API.

```yaml
subscribers:
  - name: Telegram alert
    telegram:
      token: "123456:ABC-DEF..."
      chat_id: "-1001234567890"
```

---

## WhatsApp

Sends a WhatsApp message via the WhatsApp Business API (e.g. Twilio or Meta Cloud API).

```yaml
subscribers:
  - name: WhatsApp alert
    whatsapp:
      from: "whatsapp:+14155238886"
      to: "whatsapp:+15005550006"
      account_sid: ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX   # Twilio
      auth_token: your_auth_token
```

---

## Slack

Posts a message to a Slack channel via an [incoming webhook](https://api.slack.com/messaging/webhooks).

```yaml
subscribers:
  - name: Slack notification
    slack:
      webhook_url: https://hooks.slack.com/services/T00000000/B00000000/XXXX
```

---

## Discord

Posts a message to a Discord channel via a server webhook.

```yaml
subscribers:
  - name: Discord notification
    discord:
      webhook_url: https://discord.com/api/webhooks/000000000/XXXX
```

---

## GitHub Actions

Triggers a [`workflow_dispatch`](https://docs.github.com/en/actions/writing-workflows/choosing-when-your-workflow-runs/events-that-trigger-workflows#workflow_dispatch) event on a GitHub Actions workflow. Useful for rebuilding a static site or running a pipeline when records change.

```yaml
subscribers:
  - name: Rebuild site on GitHub Actions
    github_actions:
      owner: my-org
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
  - name: Trigger GitLab pipeline
    gitlab_ci:
      project_id: "12345678"
      ref: main
      token: glptt-XXXX
```

---

## ntfy.sh

Sends a push notification via [ntfy.sh](https://ntfy.sh) — a simple, open-source, self-hostable notification service.

```yaml
subscribers:
  - name: Push notification
    ntfy:
      topic: my-ingitdb-alerts
      server: https://ntfy.sh   # optional, defaults to ntfy.sh
```

---

## SMS

Sends an SMS via Twilio or Vonage. Useful for high-priority record alerts.

```yaml
subscribers:
  - name: SMS alert
    sms:
      provider: twilio          # or: vonage
      from: "+15005550006"
      to: "+14155238886"
      account_sid: ACXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
      auth_token: your_auth_token
```

---

## Search index sync

Pushes record changes to a search index. Useful for content-management databases powering a search UI.

```yaml
subscribers:
  - name: Algolia sync
    search_index:
      provider: algolia         # or: meilisearch, typesense
      app_id: XXXXXXXXXX
      api_key: your_api_key
      index: records

  - name: Meilisearch sync
    search_index:
      provider: meilisearch
      host: http://localhost:7700
      api_key: your_api_key
      index: records
```

---

## RSS/Atom feed

Regenerates an RSS or Atom feed file whenever records are created or updated. Intended for content collections (blog posts, changelogs, etc.).

```yaml
subscribers:
  - name: Blog RSS feed
    rss:
      output: public/feed.xml
      title: "My Blog"
      link: https://example.com
      format: rss2              # or: atom
```

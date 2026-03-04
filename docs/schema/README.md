# ⚙️ inGitDB Schema Definitions

> **Building a frontend client or an AI agent integration?**
> Start with [`root-config.md`](root-config.md) — it is the authoritative entry point for
> understanding the repository-level configuration that any client must parse first.

inGitDB defines schemas for your core records, workflows, and output routing entirely inside
directory-scoped YAML configuration files.

## 📂 Overview

This section outlines the reference file structures for modelling collections, materialized
views, triggers, and hierarchical boundaries.

- **[Root config](root-config.md)** (`.ingitdb/`) — Repository-level configuration: collection
  map, namespace imports, `default_namespace`, and language settings. **Frontend builders and
  AI agents start here.**
- **[Collections](collection.md)** — Both layout styles (`.collection/` dedicated and
  `.collections/` shared), full field reference for `definition.yaml`: storage layouts, column
  types, locale shortcuts, `default_view`, and shared-directory validator rules.
- **[Subcollections](subcollection.md)** (`.collection/subcollections/{sub}/` or
  `.collections/{name}/{sub}/definition.yaml`)
  — Hierarchical subsets of standard collections supporting distinct relation and subset logic
  structures while preserving exact collection syntax.
- **[Views](view.md)** (`.collections/{name}/$views/<name>.yaml`) — Materialized view setups
  to filter, paginate, sort, and pipe your documents into generated outputs.
- **[Triggers](trigger.md)** (`.collection/trigger_<name>.yaml`) — Workflow events mapping your
  modifications directly onto shell hooks and external REST webhooks.
- **[Subscribers](subscribers/)** (`.ingitdb/subscribers.yaml`) — Built-in, configurable event
  handlers (webhook, email, Slack, Telegram, search index sync, and more) with no external
  tooling required.

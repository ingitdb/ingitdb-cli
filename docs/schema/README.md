# ⚙️ inGitDB Schema Definitions

inGitDB defines schemas for your core records, workflows, and output routing entirely inside directory-scoped YAML configuration files.

## 📂 Overview

This section outlines the reference file structures for modeling collections, materialized views, triggers, and hierarchical boundaries.

- **[Collections](collection.md)** (`.collection/definition.yaml`) — The foundational database tables defining underlying properties, storage layouts, and type requirements for mapped documents.
- **[Subcollections](subcollection.md)** (`.collection/subcollections/<name>/.collection/definition.yaml`) — Hierarchical subsets of standard collections supporting distinct relation and subset logic structures while preserving exact collection syntax.
- **[Views](view.md)** (`.collection/views/<name>.yaml`) — Materialized view setups to filter, paginate, sort, and pipe your documents into generated outputs.
- **[Triggers](trigger.md)** (`.collection/trigger_<name>.yaml`) — Workflow events mapping your modifications directly onto shell hooks and external REST webhooks.
- **[Subscribers](subscribers/)** (`.ingitdb/subscribers.yaml`) — Built-in, configurable event handlers (webhook, email, Slack, Telegram, search index sync, and more) with no external tooling required.

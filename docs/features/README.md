# ✨ Features of inGitDB

| Feature                                                   | Status      | Description                                                                                                                                        |
| --------------------------------------------------------- | ----------- | -------------------------------------------------------------------------------------------------------------------------------------------------- |
| [Validator](validator.md)                                 | WIP         | Validates schema & data; rebuilds materialized views in the same pass                                                                              |
| [Materialized Views](materialized-views.md)               | WIP         | Generate a precomputed output dataset from multiple source files using defined filters, joins, and transformation logic for fast reuse and access. |
| [Transactions](transactions.md)                           | WIP         | Read-only and read-write transactions via the DALgo abstraction layer                                                                              |
| [GitHub Direct Access](github-direct-access.md)           | implemented | Read and write records in any GitHub repository without cloning, using the GitHub REST API                                                         |
| [Merge Conflict Resolution](merge-conflict-resolution.md) | pending     | Auto-resolves conflicts in generated files; interactive TUI for source data file conflicts                                                         |
| [Pull](pull.md)                                           | pending     | One-command pull cycle: git pull → resolve conflicts → rebuild views → print change summary                                                        |
| [Watcher](watcher.md)                                     | pending     | Watch DB for file-system changes and emit structured record events to stdout                                                                       |
| [Subscribers](subscribers/)                               | pending     | Receive notifications about inGitDB changes                                                                                                        |
| [MCP Server](mcp-server.md)                               | pending     | MCP server for AI agents to read & modify data                                                                                                     |
| [HTTP API Server](http-api-server.md)                     | pending     | OpenAPI-compatible REST API for reading and editing inGitDB data                                                                                   |
| GraphQL                                                   | pending     | GraphQL interface auto-generated from collection schemas (Phase 8)                                                                                 |
| [Migration Script Generator](migration.md)                | pending     | Generates forward and rollback migration scripts to sync a target database with a desired inGitDB version                                          |

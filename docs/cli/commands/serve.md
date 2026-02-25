### 🔹 serve` — start one or more servers _(not yet implemented)_

[Source Code](../../../cmd/ingitdb/commands/serve.go)


```
ingitdb serve [--path=PATH] [--mcp] [--http] [--watcher]
```

| Flag          | Description                                                                |
| ------------- | -------------------------------------------------------------------------- |
| `--path=PATH` | Path to the database directory. Defaults to the current working directory. |
| `--mcp`       | Enable the MCP (Model Context Protocol) server.                            |
| `--http`      | Enable the HTTP API server.                                                |
| `--watcher`   | Enable the file watcher.                                                   |

At least one service flag must be provided. Multiple flags may be combined to run services together in a single process.

**Examples:**

```shell
# 🤖 Start the MCP server for AI agent access
ingitdb serve --mcp

# 📘 Start the HTTP API server
ingitdb serve --http

# 🧩 Start MCP and the file watcher together in one process
ingitdb serve --mcp --watcher

# 🔁 Start all services for a specific database path
ingitdb serve --mcp --http --watcher --path=/var/db/myapp
```

---


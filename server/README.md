# inGitDB server — removed

The `ingitdb serve` command and its HTTP API / MCP gateway packages
(`server/api`, `server/auth`, `server/mcp`) were **removed**. They were a
still-born, datatug-overlapping surface — see
[`docs/adr/0001-remove-serve-command.md`](../docs/adr/0001-remove-serve-command.md)
for the rationale and recovery instructions (last present at commit `184a40e`).

For programmatic access to a GitHub-backed inGitDB database, import the
`pkg/dalgo2ghingitdb` driver directly; cross-source serving belongs to
[DataTug](https://github.com/datatug/datatug-cli), which already consumes
inGitDB as a backend.

This `server/` directory now holds only the **website** assets
(`ingitdb.com/`, `static/`, `firebase.json`), deployed via the
`deploy-website` workflow.

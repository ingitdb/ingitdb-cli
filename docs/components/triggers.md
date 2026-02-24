# inGitDB Triggers

Triggers run shell commands automatically when records in a collection are created, updated, or deleted. They follow a GitHub Actions-inspired workflow model: an `on` block declares which events fire the trigger, and a `jobs` map defines what to run.

Each job specifies `runs-on: "."` (meaning it runs in the same environment as ingitdb itself) and a list of `steps`, where each step has a `run` field containing the shell command to execute.

See [Trigger Definition File](../schema/trigger.md) for the full YAML reference and an example.

## Subscribers

For common integrations — webhooks, email, Slack, Telegram, search index sync, and more — ingitdb provides [Subscribers](../features/subscribers/): built-in, configurable event handlers that run inside the CLI process with no external tooling required. Subscribers are a higher-level alternative to shell-step triggers for these well-known targets.

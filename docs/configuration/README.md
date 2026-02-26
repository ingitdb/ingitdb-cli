# ⚙️ inGitDB Configuration

## ⚙️ User config — `~/.ingitdb/.ingitdb-user.yaml`

Lists inGitDB databases recently opened by the user — used to improve the experience in
interactive mode. Not required; the `ingitdb` CLI auto-detects the repository config when
started inside any directory under a Git repo.

## ⚙️ Repository config — `.ingitdb/` directory

At the root of every inGitDB-enabled repository there is a `.ingitdb/` directory.
All files inside it are optional — an empty directory (or no directory) represents a valid,
empty inGitDB instance.

| File                              | Purpose                                                                          |
| --------------------------------- | -------------------------------------------------------------------------------- |
| `.ingitdb/root-collections.yaml`  | [Root collections](root-collections.md) — flat map of collection IDs → paths, including [namespace imports](root-collections.md#namespace-imports) |
| `.ingitdb/settings.yaml`          | Repository settings: [`default_namespace`](root-collections.md#default_namespace) and [languages](languages.md) |
| `.ingitdb/README.md`              | Human-readable overview and stats (documentation only, no code impact)           |

Full schema reference for all config files: [`docs/schema/root-config.md`](../schema/root-config.md)

Each collection directory contains an `.collection/definition.yaml` file:

- [Collection schema definitions](../schema/README.md)

# Skills

inGitDB skills fall into two categories: a small set of **infrastructure** skills that handle plugin-level concerns (such as installing the CLI) and a larger set of **CLI-wrapper** skills that expose the `ingitdb` CLI's command surface to agents. Each CLI-wrapper skill covers a single CLI command or coherent group of commands; per-verb detail lives under `references/` and loads on demand.

This plugin follows the same design as the sibling [`specscore`](https://github.com/synchestra-io/ai-plugin-specscore) and [`synchestra`](https://github.com/synchestra-io/ai-plugin-synchestra) plugins.

## Invocation

All skills are prefixed with the plugin's manifest name. Users invoke them as:

```
/ingitdb:<skill-name>
```

## Skill categories

- **Infrastructure skills** — plugin-level actions that are not backed by an `ingitdb` CLI command. Today this category contains only `install`, which bootstraps the CLI itself.
- **CLI-wrapper skills** — one skill per major `ingitdb` CLI surface area. Each wrapper assumes the CLI is already installed and callable; see the [Pre-flight pattern](#pre-flight-pattern) below.

## Available infrastructure skills

| Skill | Purpose |
|---|---|
| [`install/`](install/SKILL.md) | Show install instructions for the `ingitdb` CLI. Runtime prerequisite for every wrapper skill. |

## Available CLI-wrapper skills

| Skill | Wraps | Verbs / references |
|---|---|---|
| [`validate/`](validate/SKILL.md) | `ingitdb validate` | `full`, `definitions`, `records`, `diff` |
| [`list/`](list/SKILL.md) | `ingitdb list` | `collections`, `views` |
| [`describe/`](describe/SKILL.md) | `ingitdb describe` | `collection`, `view` |
| [`record/`](record/SKILL.md) | `ingitdb insert / select / update / delete` | `insert`, `select`, `update`, `delete` |

## Pre-flight pattern

Every CLI-wrapper skill must verify that `ingitdb` is installed before invoking it. Copy the block below verbatim into the top of any new wrapper `SKILL.md`:

> ### Pre-flight check
>
> Before running any `ingitdb` command, verify the CLI is installed:
>
> ```bash
> command -v ingitdb >/dev/null 2>&1
> ```
>
> If this check fails (exit `127` / `command not found`), stop and tell the user exactly:
>
> > The `ingitdb` CLI is not installed. Either:
> > - invoke `/ingitdb:install` to see install options, or
> > - install per <https://github.com/ingitdb/ingitdb-cli#installation>.
> >
> > Then retry your command.
>
> Do not proceed with the original command until `ingitdb version` succeeds.

## Not-yet-wrapped commands

The CLI exposes several additional commands not yet covered by a skill. They can be invoked directly (no skill required) — wrappers will be added as usage patterns emerge:

`ci`, `docs`, `drop`, `materialize`, `pull`, `rebase`, `resolve`, `setup`, `version`.

## Status

**Shipped:** `install/` (infrastructure), `validate/`, `list/`, `describe/`, `record/` (CLI wrappers).

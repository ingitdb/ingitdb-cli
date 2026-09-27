---
name: install
description: Show install instructions for the ingitdb CLI. Use when `ingitdb` is not on PATH, when another skill reports `command not found: ingitdb`, or when the user asks how to install, reinstall, or update the CLI.
user-invocable: true
---

# Install the ingitdb CLI

This skill **shows the user how to install** the [`ingitdb`](https://github.com/ingitdb/ingitdb-cli) CLI — the runtime prerequisite for every other skill in this plugin. It does not execute the installer. The user runs the install command themselves in their own shell.

## When to use

- `ingitdb` is not on `PATH` (e.g., `command -v ingitdb` returns nothing).
- Another skill in this plugin reported `command not found: ingitdb` or a pre-flight failure.
- The user explicitly asked to install, reinstall, or update the `ingitdb` CLI.

## Behavior

**Do not run installers from this skill.** Installing system binaries is a trust decision that belongs to the user, in their own terminal. Surface the options below, then wait.

## What to show the user

Present the platform-appropriate option(s) and let the user pick:

### macOS — Homebrew

```bash
brew tap ingitdb/cli
brew install ingitdb
```

### Linux — Snap

```bash
snap install ingitdb
```

### Linux — Linuxbrew

```bash
brew tap ingitdb/cli
brew install ingitdb
```

### Linux — Arch (AUR)

```bash
yay -S ingitdb-bin
```

### Windows — WinGet

```powershell
winget install ingitdb
```

### Windows — Chocolatey

```powershell
choco install ingitdb
```

### Windows — Scoop

```powershell
scoop bucket add ingitdb https://github.com/ingitdb/scoop-bucket
scoop install ingitdb
```

### Go (any platform)

```bash
go install github.com/ingitdb/ingitdb-cli/cmd/ingitdb@latest
```

### Binary download (any platform)

Download the matching archive from the [GitHub Releases](https://github.com/ingitdb/ingitdb-cli/releases) page.

## Verification

Once the user confirms install is complete, verify with:

```bash
ingitdb version
```

If the binary is found and prints a version, continue with the user's original request. If not, ask the user to check their `PATH` and re-run install.

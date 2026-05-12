# Guidelines for AI Agents

Use [common guidelines](docs/GUIDELINES.md).

Apply learned [skills](.github/copilot/skills/) when the context matches.

## Spec Workflow (SpecScore)

This project uses [SpecScore](https://specscore.md) to manage feature specifications.

### Directory layout

```
spec/
  ideas/     → raw ideas before promotion (spec/ideas/<slug>.md)
  features/  → feature specs grouped by area (spec/features/cli/<slug>/README.md)
  plans/     → implementation plans
```

### Rules

1. **Ideas belong in `spec/ideas/`** — never `docs/`. Scaffold with:
   ```bash
   specscore idea new <slug> --title "..." --hmw "How Might We..."
   ```
2. **Feature specs belong in `spec/features/`** — revise in place for enhancements;
   use a new slug only for wholly new features.
3. **Run lint before finishing any spec work:**
   ```bash
   specscore spec lint          # check
   specscore spec lint --fix    # auto-fix index drift
   ```
   All errors must be resolved. Warnings should be addressed.
4. **Do not hand-edit the `ideas/README.md` index table** — it is maintained by
   `specscore spec lint --fix`.
5. **Spec → plan gate** — never write an implementation plan before the feature spec
   exists and lint is clean.

### Format conventions

Feature specs in this project use a single `README.md` per feature (no separate
`requirements/` subdirectories). ACs use prose format consistent with existing specs
in `spec/features/cli/`. The `---\n*This document follows ...*` footer is required.

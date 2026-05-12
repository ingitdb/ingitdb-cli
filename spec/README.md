# Specification

This directory contains the SpecScore specification artifacts for **ingitdb-cli**.

## Structure

| Directory | Contents |
|-----------|----------|
| [ideas/](ideas/README.md) | Raw ideas before they become Features |
| [features/](features/README.md) | Feature specifications (behavior, requirements, ACs) |
| [plans/](plans/README.md) | Implementation plans linked to Features |

## Workflow

```
idea (spec/ideas/) → feature (spec/features/) → plan (spec/plans/) → code
```

Use the `specscore` CLI to scaffold and lint artifacts:

```bash
specscore idea new <slug>          # scaffold a new idea
specscore spec lint                # validate all spec artifacts
specscore feature list             # list features and their status
```

---
*This document follows the https://specscore.md/spec-root-specification*

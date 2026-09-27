# ingitdb Install Skill

Install / reinstall / update the [`ingitdb`](https://github.com/ingitdb/ingitdb-cli) CLI. Implementation lives in [`SKILL.md`](SKILL.md); the `/ingitdb:install` slash-command alias is wired up via [`commands/install.md`](../../commands/install.md). Design rationale and explicit non-goals live in [`docs/ideas/cli-install-skill.md`](../../docs/ideas/cli-install-skill.md).

## Sister skills

This skill mirrors the install skill in sibling CLI-wrapper plugins:

- [`specscore:install`](https://github.com/synchestra-io/ai-plugin-specscore/blob/main/skills/install/SKILL.md)
- [`synchestra:install`](https://github.com/synchestra-io/ai-plugin-synchestra/blob/main/skills/install/SKILL.md)
- [`datatug:install`](https://github.com/datatug/datatug-ai-skills/blob/main/skills/install/SKILL.md)

**All implementations should remain similar in shape.** Show the user the install commands; do not execute installers from inside the skill. Verify with `<cli> version` after the user confirms install is complete. When changing one, consider whether the same change should land in the others.

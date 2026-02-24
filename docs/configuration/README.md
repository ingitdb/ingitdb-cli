# ⚙️ inGitDB Configuration

# ⚙️ User config - `~/.ingitdb/.ingitdb-user.yaml`

List inGitDB often open by user – used to improve user experience in the interactive mode.
This is not required as `ingitdb` CLI will autodetect repository config when started in a dir under Git repo.

# ⚙️ Repository config

At the root of the repository you should have a `.ingitdb.yaml` file that defines:

- [root_collections](root-collections.md) (including [namespace imports](root-collections.md#namespace-imports) and [`default_namespace`](root-collections.md#default_namespace))
- [languages](languages.md)

Each collection directory contains an `.collection/definition.yaml` file:

- [collection definitions](../schema/README.md)

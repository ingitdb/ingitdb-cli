# 📘 inGitDB README Builder

The `README.md` is a built-in specialized implementation of [materialized views](../../features/materialized-views.md).

The `inGitDB` CLI can create and update `README.md` for collections and collection records
following rules defined in the `.collection/definition.yaml` file.

This is used to create human- and AI-readable summaries about collections and records.

## ⚙️ Collection README

When running `ingitdb materialize collection`, the CLI renders the `README.md` file for the collection. If the generated content differs from the existing file, it is automatically updated.

A collection's `README.md` file includes the following auto-generated sections:

- **Collection name**: Human-readable name of the collection.
- **Path to collection**: Shown if it is a subcollection.
- **Table of columns**: Lists all columns with their name, type, and other properties.
- **Table of subcollections**: Lists nested subcollections with their name and the number of their subcollections.
- **Table of views**: Lists available materialized views with their name and the number of columns.

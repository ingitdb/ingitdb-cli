# Search index sync

Pushes record changes to a full-text search index. Useful for content-management databases powering a search UI.

## Fields

| Field      | Type     | Required                      | Description                              |
| ---------- | -------- | ----------------------------- | ---------------------------------------- |
| `name`     | `string` | no                            | Label shown in logs                      |
| `provider` | `string` | yes                           | `algolia`, `meilisearch`, or `typesense` |
| `index`    | `string` | yes                           | Target index name                        |
| `app_id`   | `string` | yes (Algolia)                 | Algolia application ID                   |
| `api_key`  | `string` | yes                           | Provider API key                         |
| `host`     | `string` | yes (Meilisearch / Typesense) | Server base URL                          |

## YAML example

```yaml
subscribers:
  content-search-sync:
    name: "Keep search indexes in sync with content"
    for:
      paths:
        - content/posts
        - content/pages
    search_indexes:
      - name: Algolia
        provider: algolia
        app_id: XXXXXXXXXX
        api_key: "<ALGOLIA_WRITE_API_KEY>"
        index: content

      - name: Meilisearch (self-hosted)
        provider: meilisearch
        host: http://localhost:7700
        api_key: "<MEILISEARCH_MASTER_KEY>"
        index: content
```

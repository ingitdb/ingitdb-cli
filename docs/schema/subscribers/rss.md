# RSS/Atom feed

Regenerates an RSS or Atom feed file whenever records are created or updated. Intended for content collections (blog posts, changelogs, etc.).

## Fields

| Field    | Type     | Required | Default | Description                                  |
| -------- | -------- | -------- | ------- | -------------------------------------------- |
| `name`   | `string` | no       |         | Label shown in logs                          |
| `output` | `string` | yes      |         | Output file path (relative to database root) |
| `title`  | `string` | yes      |         | Feed title                                   |
| `link`   | `string` | yes      |         | Canonical URL of the site                    |
| `format` | `string` | no       | `rss2`  | Feed format: `rss2` or `atom`                |

## YAML example

```yaml
subscribers:
  blog-feeds:
    for:
      paths:
        - content/posts
      events:
        - created
        - updated
    rss:
      - name: RSS 2.0
        output: public/feed.xml
        title: "My Blog"
        link: https://example.com
        format: rss2
      - name: Atom
        output: public/atom.xml
        title: "My Blog"
        link: https://example.com
        format: atom
```

# ⚙️ inGitDB Repository Configuration — Languages

Languages are configured in `.ingitdb/settings.yaml`:

```yaml
# .ingitdb/settings.yaml
#
# `languages` lists supported languages.
# Prefer ISO 639-1 codes; IETF BCP 47 (RFC 5646 / RFC 4647) is also accepted.
#
# Examples:
#   required: en
#   required: es-ES
#   required: es-MX
#   optional: ru
#
# Required languages must appear before optional ones.
# The language selector shows languages in the order defined here.
languages:
  - required: en
  - required: fr
  - required: es
  - optional: ru
```
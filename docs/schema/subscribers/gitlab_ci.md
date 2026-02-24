# GitLab CI

Triggers a GitLab pipeline via the [pipeline trigger API](https://docs.gitlab.com/ee/ci/triggers/).

## Fields

| Field        | Type     | Required | Description                          |
| ------------ | -------- | -------- | ------------------------------------ |
| `name`       | `string` | no       | Label shown in logs                  |
| `project_id` | `string` | yes      | GitLab project ID (numeric string)   |
| `ref`        | `string` | yes      | Branch or tag to run the pipeline on |
| `token`      | `string` | yes      | GitLab pipeline trigger token        |
| `host`       | `string` | no       | GitLab host (default: `gitlab.com`)  |

## YAML example

```yaml
subscribers:
  deploy-pipeline:
    for:
      events:
        - created
        - updated
        - deleted
    gitlab_ci:
      - name: Deploy pipeline
        project_id: "12345678"
        ref: main
        token: glptt-XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
```

# GitHub Actions

Triggers a [`workflow_dispatch`](https://docs.github.com/en/actions/writing-workflows/choosing-when-your-workflow-runs/events-that-trigger-workflows#workflow_dispatch) event on a GitHub Actions workflow. Useful for rebuilding a static site or running a deployment pipeline when records change.

## Fields

| Field      | Type     | Required | Description                              |
| ---------- | -------- | -------- | ---------------------------------------- |
| `name`     | `string` | no       | Label shown in logs                      |
| `owner`    | `string` | yes      | GitHub organisation or user name         |
| `repo`     | `string` | yes      | Repository name                          |
| `workflow` | `string` | yes      | Workflow file name (e.g. `deploy.yml`)   |
| `ref`      | `string` | yes      | Branch or tag to run the workflow on     |
| `token`    | `string` | yes      | GitHub Personal Access Token (`ghp_...`) |

## YAML example

```yaml
subscribers:
  deploy-site:
    name: "Rebuild and deploy site on any change"
    for:
      events:
        - created
        - updated
        - deleted
    github_actions:
      - name: Deploy
        owner: my-org
        repo: my-site
        workflow: deploy.yml
        ref: main
        token: ghp_XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
```

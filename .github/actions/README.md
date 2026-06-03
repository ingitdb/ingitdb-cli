# GitHub Actions — Reusable Actions

This directory contains reusable composite GitHub Actions that encapsulate common deployment and CI/CD tasks. These actions are used by multiple workflows to maintain consistency and reduce duplication.

## Available Actions

### [Deploy Website](./deploy-website/)

Deploys the inGitDB website to Firebase Hosting.

**Use when**: You need to deploy the website to Firebase after a release or manual trigger.

**Key features**:
- Automatic Firebase CLI version detection and caching
- Node.js setup and configuration
- Non-interactive Firebase deployment
- Secure authentication via Workload Identity Federation

**Used by**:
- `.github/workflows/release.yml` — Auto-deploys after successful release
- `.github/workflows/deploy-website.yml` — Manual deployment trigger

[Full documentation →](./deploy-website/README.md)

## Using Reusable Actions

### Basic Syntax

```yaml
jobs:
  my-job:
    runs-on: ubuntu-latest
    permissions:
      contents: 'read'
      id-token: 'write'

    steps:
      - name: My step
        uses: ./.github/actions/action-name
        with:
          input-1: value1
          input-2: value2
```

### Key Points

- **Reference actions with relative path**: Use `./.github/actions/action-name` to reference actions in the same repository
- **Permissions**: Ensure your job has the necessary permissions (especially `id-token: 'write'` for Workload Identity)
- **Inputs**: All action inputs must be provided either as hardcoded values or from environment variables
- **Checkout**: The action must run in a workflow that has already checked out the repository

## GCP Authentication

All reusable actions that interact with Google Cloud use **Workload Identity Federation** for secure, keyless authentication:

1. **No secrets stored**: Uses OpenID Connect (OIDC) tokens issued by GitHub
2. **Short-lived credentials**: Tokens are automatically issued and expire quickly
3. **Automatic service account impersonation**: GitHub Actions automatically assumes the correct service account
4. **Audit trail**: All actions are logged in GCP for compliance and debugging

### Workload Identity Provider

The inGitDB project uses:
```
projects/152447355531/locations/global/workloadIdentityPools/github/providers/ingitdb
```

Service account:
```
deployer2gcloud@ingitdb.iam.gserviceaccount.com
```

## Adding New Reusable Actions

To create a new reusable action:

1. Create a new directory: `.github/actions/action-name/`
2. Create `action.yml` with the action definition
3. Create `README.md` with comprehensive documentation
4. Reference the action in workflows: `uses: ./.github/actions/action-name`

### Action Structure

```
.github/actions/
├── action-name/
│   ├── action.yml          # Action definition
│   ├── README.md           # Documentation
│   └── scripts/            # (Optional) Helper scripts
```

### Best Practices

- **Validate inputs**: Always validate required inputs early in the action
- **Use shell: bash**: Specify shell explicitly for all bash commands
- **Document thoroughly**: Include examples, prerequisites, and troubleshooting
- **Handle errors**: Exit with appropriate error codes and meaningful messages
- **Cache when possible**: Use `actions/cache@v4` to cache dependencies
- **Use composite actions**: Prefer composite actions over Docker containers for simplicity
- **No secrets in actions**: Never hardcode secrets; always use inputs or environment variables

## Workflows Using Reusable Actions

### Release Workflow (`.github/workflows/release.yml`)

Triggered on:
- Tag push matching `v*` pattern
- Manual trigger (`workflow_dispatch`)
- CI workflow completion on `v*` branches

Jobs:
1. `bump_version` — Bumps version using semantic versioning
2. `goreleaser` — Builds and publishes releases
3. `deploy-website` — Deploys website to Firebase (depends on `goreleaser`)

### Deploy Website Workflow (`.github/workflows/deploy-website.yml`)

Triggered on:
- Manual trigger (`workflow_dispatch`)

Jobs:
1. `deploy` — Deploys website to Firebase using the reusable action

## Monitoring and Debugging

### View Action Execution

1. Go to your repository → **Actions** tab
2. Select the workflow run
3. Expand any job to see detailed logs
4. Each step in a reusable action appears with its name in the logs

### Common Issues

**Action not found**
- Ensure the action path is correct: `./.github/actions/action-name`
- Check that `action.yml` exists in the action directory

**Permission denied**
- Verify job has required permissions in `permissions:` block
- Check that service account has necessary IAM roles

**Missing inputs**
- Verify all required inputs are provided
- Check input names match exactly (case-sensitive)

**Authentication failures**
- Verify Workload Identity Pool is configured correctly
- Check service account has necessary permissions
- Ensure `id-token: 'write'` permission is set

## References

- [GitHub Actions Documentation](https://docs.github.com/en/actions)
- [Composite Actions](https://docs.github.com/en/actions/creating-actions/creating-a-composite-action)
- [Workload Identity Federation Setup](https://docs.github.com/en/actions/deployment/security-hardening-your-deployments/about-security-hardening-with-openid-connect)

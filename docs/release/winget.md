# WinGet Setup

WinGet (Windows Package Manager) allows Windows users to install ingitdb via `winget install ingitdb`.

## 1. Fork microsoft/winget-pkgs

WinGet requires submitting packages via pull request to Microsoft's repository. Goreleaser does this by pushing to a fork in the `ingitdb` org first.

1. Go to [https://github.com/microsoft/winget-pkgs](https://github.com/microsoft/winget-pkgs)
2. Click **Fork** in the top-right corner
3. Set **Owner** to `ingitdb`
4. Click **Create fork**

## 2. Create a Classic PAT for WinGet

WinGet requires creating a PR on `microsoft/winget-pkgs`, which is outside the `ingitdb` org. Fine-grained PATs scoped to an organization cannot call GitHub APIs on external repositories, so a **classic PAT** is required.

1. Go to **GitHub → Settings → Developer settings → Personal access tokens → Tokens (classic)**
2. Click **Generate new token (classic)**
3. Set scope: `public_repo` (under `repo`)
4. Copy the token
5. Add it as a secret in `ingitdb/ingitdb-cli`:
   - **Name:** `WINGET_GITHUB_TOKEN`
   - **Value:** the classic PAT

The token needs access to:
- `ingitdb/winget-pkgs` — to push manifests to the fork
- `microsoft/winget-pkgs` — to open the PR (requires classic PAT with `public_repo`)

## 3. How It Works

On each release goreleaser:

1. Generates WinGet manifests under `manifests/i/ingitdb/ingitdb/<version>/`
2. Pushes them to `ingitdb/winget-pkgs` (the fork)
3. Opens a PR from `ingitdb/winget-pkgs` → `microsoft/winget-pkgs`
4. Microsoft's automated validation checks run on the PR
5. Microsoft team reviews and merges within 1-3 days

## 4. Monitor PR Submission

After a release:

1. Go to [https://github.com/microsoft/winget-pkgs/pulls](https://github.com/microsoft/winget-pkgs/pulls)
2. Search for a PR titled like: `New submit: ingitdb.ingitdb version X.X.X`
3. Automated checks (WinGet Validation, formatting) should pass
4. Once merged, the package is available via `winget install ingitdb`

## 5. Verify WinGet Setup

After the PR is merged:

```powershell
# Search for ingitdb
winget search ingitdb

# Install ingitdb
winget install ingitdb

# Verify installation
ingitdb --version

# Uninstall
winget uninstall ingitdb
```

---

## Troubleshooting

### "403 Resource not accessible by personal access token"

This means a fine-grained PAT is being used. Fine-grained PATs cannot create PRs on repositories outside the configured organization scope. Use a classic PAT with `public_repo` scope stored as `WINGET_GITHUB_TOKEN` (see Step 2).

### "404 Not Found" on fork sync

The fork doesn't exist yet. Complete Step 1 to create it.

### "PR not created"

- Check GitHub Actions logs for the exact error
- Verify the fork is up to date with `microsoft/winget-pkgs`
- Re-run the workflow after creating/fixing the fork

---

## References

- [WinGet Package Manager](https://github.com/microsoft/winget-pkgs)
- [GoReleaser: WinGet Publishing](https://goreleaser.com/customization/winget/)

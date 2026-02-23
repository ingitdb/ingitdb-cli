# WinGet Setup

WinGet (Windows Package Manager) allows Windows users to install ingitdb via `winget install ingitdb`.

## 1. Fork microsoft/winget-pkgs

WinGet requires submitting packages via pull request to Microsoft's repository. Goreleaser does this by pushing to a fork in the `ingitdb` org first.

1. Go to [https://github.com/microsoft/winget-pkgs](https://github.com/microsoft/winget-pkgs)
2. Click **Fork** in the top-right corner
3. Set **Owner** to `ingitdb`
4. Click **Create fork**

## 2. Verify Token Permissions

The `INGITDB_GORELEASER_GITHUB_TOKEN` must have `contents: write` access to `ingitdb/winget-pkgs` (the fork). No additional secrets are needed — the same token is used for the PR to `microsoft/winget-pkgs`.

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

The token is trying to push directly to `microsoft/winget-pkgs`. Ensure:
- The fork `ingitdb/winget-pkgs` exists (see Step 1)
- `INGITDB_GORELEASER_GITHUB_TOKEN` has write access to the fork

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

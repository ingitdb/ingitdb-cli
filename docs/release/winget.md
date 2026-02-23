# WinGet Setup

WinGet (Windows Package Manager) allows Windows users to install ingitdb via `winget install ingitdb`.

## 1. No Setup Required

WinGet uses an **automated pull request submission** system. No account or API key is needed:

- Goreleaser automatically generates a WinGet manifest
- Submits a PR to [microsoft/winget-pkgs](https://github.com/microsoft/winget-pkgs)
- Microsoft's automated checks validate the manifest
- Microsoft team reviews and merges within 1-3 days

## 2. Monitor PR Submission

After a release:

1. Go to [https://github.com/microsoft/winget-pkgs](https://github.com/microsoft/winget-pkgs)
2. Check **Pull requests** tab for a PR with title like: `New submit: ingitdb version X.X.X`
3. The PR should have automated checks (validation, formatting)
4. Once merged, the package is available via `winget install ingitdb`

## 3. Verify WinGet Setup

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

### "PR not created"

- Verify `GITHUB_TOKEN` (or `INGITDB_GORELEASER_GITHUB_TOKEN`) has sufficient permissions
- Check GitHub Actions logs for detailed error messages
- Ensure the repository name matches the pattern expected by WinGet

---

## References

- [WinGet Package Manager](https://github.com/microsoft/winget-pkgs)
- [GoReleaser: WinGet Publishing](https://goreleaser.com/customization/wingets/)

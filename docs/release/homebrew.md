# Homebrew Formula Setup

Homebrew Formula allows Linux users via Linuxbrew to install ingitdb via `brew install ingitdb`.

## 1. Prepare the homebrew-cli Tap Repository

The `ingitdb/homebrew-cli` tap repository must exist and be writable by the GitHub token.

### If you don't have a homebrew tap yet:

1. Create a public GitHub repo named `homebrew-cli` in the `ingitdb` organization
   - Go to [https://github.com/new](https://github.com/new)
   - **Repository name:** `homebrew-cli`
   - **Owner:** ingitdb (organization)
   - **Visibility:** Public
   - **Initialize with:** (leave empty, goreleaser will create the formula)
   - Click **Create repository**

2. Clone it locally:
   ```bash
   git clone https://github.com/ingitdb/homebrew-cli.git
   cd homebrew-cli
   ```

3. Add a README:
   ```bash
   cat > README.md << 'EOF'
   # ingitdb Homebrew Tap

   Homebrew tap for ingitdb (macOS Cask and Linuxbrew Formula).

   ## Installation

   ### macOS (Cask)
   ```bash
   brew tap ingitdb/cli
   brew install ingitdb
   ```

   ### Linux (Linuxbrew / Formula)
   ```bash
   brew tap ingitdb/cli
   brew install ingitdb
   ```

   See [ingitdb.com](https://ingitdb.com) for documentation.
   EOF

   git add README.md
   git commit -m "chore: add tap documentation"
   git push origin main
   ```

### If you already have a homebrew tap:

Ensure it has write permissions for the GitHub token being used (usually `GITHUB_TOKEN` from the release workflow).

## 2. Verify Homebrew Formula Setup

After the first release, goreleaser will create `Formula/ingitdb.rb` in the homebrew-cli repo:

```bash
# Check the formula was created
git clone https://github.com/ingitdb/homebrew-cli.git
ls Formula/

# You should see: ingitdb.rb
```

## 3. Test Homebrew Installation (Optional)

After a release, test the formula on Linux (Linuxbrew):

```bash
# Install Linuxbrew (if not already installed)
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"

# Add Homebrew to PATH
eval "$(/home/linuxbrew/.linuxbrew/bin/brew shellenv)"

# Install ingitdb
brew tap ingitdb/cli
brew install ingitdb

# Verify
ingitdb --version
```

---

## Troubleshooting

### "Repository not found"

- Verify `homebrew-cli` repository exists in `ingitdb` organization
- Check `GITHUB_TOKEN` in the workflow has `contents: write` permission
- Ensure the repo is public

---

## References

- [Homebrew Tap Documentation](https://docs.brew.sh/Taps)
- [GoReleaser: Homebrew Publishing](https://goreleaser.com/customization/homebrew/)

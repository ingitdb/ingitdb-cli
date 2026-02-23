# Chocolatey Setup

Chocolatey allows Windows users to install ingitdb via `choco install ingitdb`.

## 1. Create a Chocolatey Account

1. Go to [https://community.chocolatey.org/account/Register](https://community.chocolatey.org/account/Register)
2. Sign up with:
   - Username (e.g., `ingitdb`)
   - Email address
   - Password
3. Verify your email
4. Log in to [https://community.chocolatey.org](https://community.chocolatey.org)

## 2. Generate API Key

1. Go to [https://community.chocolatey.org/account](https://community.chocolatey.org/account) (log in if needed)
2. Scroll to **API Keys** section
3. Click **Create New API Key**
4. Copy the generated API key

## 3. Store API Key as GitHub Secret

1. Go to your GitHub repo → **Settings** → **Secrets and variables** → **Actions**

2. Click **New repository secret**
   - **Name:** `CHOCOLATEY_API_KEY`
   - **Secret:** Paste the API key from step 2

3. Click **Add secret**

## 4. First Submission Approval

The first time you publish to Chocolatey Community Repository:

1. Your package will be submitted to a moderation queue
2. The Chocolatey team reviews the package (typically 1-2 days)
3. Once approved, the package appears in the Community Repository
4. Future releases auto-publish without additional approval

## 5. Verify Chocolatey Setup

After publishing your first release:

```powershell
# List available versions of ingitdb
choco search ingitdb

# Install ingitdb
choco install ingitdb

# Verify installation
ingitdb --version

# Uninstall
choco uninstall ingitdb
```

---

## Troubleshooting

### "Unauthorized (invalid API key)"

- Verify the `CHOCOLATEY_API_KEY` secret is set correctly
- Check the API key hasn't expired at [community.chocolatey.org/account](https://community.chocolatey.org/account)
- Generate a new API key if needed and update the GitHub secret

---

## References

- [Chocolatey Publishing Guide](https://docs.chocolatey.org/en-us/community-repository/community-packages-maintenance/package-validator-troubleshooting)
- [GoReleaser: Chocolatey Publishing](https://goreleaser.com/customization/chocolatey/)

# Snapcraft Setup

Snapcraft allows universal Linux users to install ingitdb via `snap install ingitdb`.

## 1. Create a Snapcraft Account

1. Go to [https://snapcraft.io/account/register](https://snapcraft.io/account/register)
2. Sign up with:
   - Email address
   - Password
   - Username
3. Verify your email
4. Log in to [https://snapcraft.io](https://snapcraft.io)

## 2. Reserve the Snap Name

1. Go to [https://snapcraft.io/snaps](https://snapcraft.io/snaps)
2. Click **Create a new snap** or **Submit a snap**
3. Register the name: `ingitdb`
4. Fill in basic info:
   - **Summary:** "A CLI for a schema-validated, AI-native database backed by a Git repository"
   - **Description:** Same as in goreleaser config
   - **License:** MIT
5. Click **Save** (you can publish the snap itself later)

## 3. Generate Snapcraft Login Credentials

This creates a login token for CI to use.

```bash
# Install snapcraft (if not already installed)
sudo snap install snapcraft --classic

# Log in and export credentials
snapcraft export-login /tmp/snap.login
# (When prompted, enter your Snapcraft username/password)

# This creates /tmp/snap.login with encrypted credentials
```

## 4. Store Credentials as GitHub Secret

1. Read the credentials file:
   ```bash
   cat /tmp/snap.login
   # Output: base64-encoded credentials
   ```

2. Go to your GitHub repo → **Settings** → **Secrets and variables** → **Actions**

3. Click **New repository secret**
   - **Name:** `SNAPCRAFT_STORE_CREDENTIALS`
   - **Secret:** Paste the entire contents of `/tmp/snap.login`

4. Click **Add secret**

5. Clean up:
   ```bash
   rm /tmp/snap.login
   ```

## 5. Request Classic Confinement (One-Time)

By default, snaps run in confined mode (restricted permissions). Ingitdb needs classic confinement to access the user's filesystem.

> **Note:** This is a one-time manual approval. Until approved, use `confinement: devmode` and `grade: devel` in goreleaser-linux.yaml for testing.

Once you've published a snap with `confinement: classic`, the Snap Store team reviews it:

1. Go to [https://forum.snapcraft.io/c/snap-requests/49](https://forum.snapcraft.io/c/snap-requests/49)

2. Click **New Topic** and include:
   ```
   Title: Classic confinement request for ingitdb

   Snap name: ingitdb

   Reason: ingitdb is a CLI database tool that needs full filesystem access to:
   - Read/write Git repositories
   - Access user home directory
   - Run git commands (requires /bin/sh, /usr/bin/git)

   Publishing link: https://snapcraft.io/ingitdb
   ```

3. The Snap Store team will review and approve within 1-2 days

## 6. Verify Snapcraft Setup

After publishing your first release:

```bash
# Search for the snap
snap search ingitdb

# Install the snap (from devmode release while waiting for approval)
sudo snap install ingitdb --devmode  # until confinement is approved

# Test it works
ingitdb --version
```

---

## Troubleshooting

### "Invalid credentials"

- Regenerate credentials: `snapcraft export-login /tmp/snap.login`
- Update the GitHub secret with new contents
- Ensure you're logged in to [snapcraft.io](https://snapcraft.io) before export

---

## References

- [Snapcraft: Publishing to the Snap Store](https://snapcraft.io/docs/release-to-the-snap-store)
- [GoReleaser: Snapcraft Publishing](https://goreleaser.com/customization/snapcraft/)

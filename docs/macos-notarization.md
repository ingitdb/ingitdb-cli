# macOS Code Signing & Notarization

## Overview

macOS Gatekeeper prevents users from running binaries that are not code-signed by an Apple Developer ID
certificate and notarized by Apple's notarization service. When an unsigned binary is downloaded from
the internet (as happens via Homebrew), Gatekeeper shows a "cannot be opened because the developer
cannot be verified" dialog. Notarization works by submitting the signed binary to Apple's servers, which
scan it for malware and return a cryptographic ticket. That ticket is then stapled to the binary so
Gatekeeper accepts it both online and offline.

## Architecture

```
Apple Developer Portal
  ├─ Developer ID Application cert (.p12)    ──┐
  └─ App Store Connect API key (.p8)         ──┤
                                               ▼
                                    GitHub Secrets (5 secrets)
                                               │
                                               ▼
                                    GitHub Actions (macos-latest runner)
                                               │
                                               ▼
                                    GoReleaser
                                      ├─ build binaries (darwin/amd64, darwin/arm64)
                                      ├─ codesign (Developer ID Application cert)
                                      ├─ notarize (xcrun notarytool via API key)
                                      └─ staple ticket
                                               │
                                               ▼
                              GitHub Release (signed + notarized .tar.gz)
                                               │
                                               ▼
                                    Homebrew Cask (ingitdb/cli tap)
                                               │
                                               ▼
                                    End user (no Gatekeeper dialog)
```

## Quick Reference

| Phase | Step | Method | Estimate |
|-------|------|--------|----------|
| 1 | Create Developer ID Application certificate | Manual (Apple portal) | ~30 min |
| 1 | Export .p12 from Keychain Access | Manual (macOS) | ~10 min |
| 1 | Create App Store Connect API key | Manual (Apple portal) | ~10 min |
| 2 | Encode secrets and store in GitHub | CLI | ~15 min |
| 3 | Update GoReleaser config | Code | ~5 min |
| 4 | Update GitHub Actions workflow | Code | ~5 min |
| 5 | Local verification (optional) | CLI | ~30 min |
| 6 | Release & verify on fresh Mac | CLI | ~10 min + ~30 min CI |

---

## Phase 1: Apple Developer Portal

### [MANUAL] 1.1 Create Developer ID Application Certificate ⏱ ~30 min

> **Important:** You need a **Developer ID Application** certificate — not "Apple Development"
> (used for Xcode device testing). Using the wrong type causes a signing failure.

1. Open **Keychain Access** on your Mac → **Keychain Access** menu → **Certificate Assistant** →
   **Request a Certificate from a Certificate Authority**.
2. Enter your email, select **Saved to disk**, click **Continue**. Save `CertificateSigningRequest.certSigningRequest`.
3. Go to [https://developer.apple.com/account/resources/certificates/list](https://developer.apple.com/account/resources/certificates/list).
4. Click **+** → choose **Developer ID Application** → **Continue**.
5. Upload the `.certSigningRequest` file → **Continue** → **Download** the `.cer` file.
6. Install the `.cer` into your login keychain — do **not** double-click (macOS may reject it as a
   root cert). Use the terminal instead:
   ```bash
   security import developerID_application.cer -k ~/Library/Keychains/login.keychain-db
   ```
7. Verify it appears correctly:
   ```bash
   security find-identity -v -p codesigning
   # expect: "Developer ID Application: <Your Name> (<TEAM_ID>)"
   ```

### [MANUAL] 1.2 Export Certificate as .p12 from Keychain Access ⏱ ~10 min

1. Open **Keychain Access** → **My Certificates**.
2. Right-click `Developer ID Application: <Your Name> (<TEAM_ID>)` → **Export**.
3. Choose format **Personal Information Exchange (.p12)** → save as `cert.p12`.
4. Enter a strong export password — you will need this as `MACOS_SIGN_PASSWORD`.
5. Keep `cert.p12` and the password in a secure location (password manager recommended).

### [MANUAL] 1.3 Create App Store Connect API Key for notarytool ⏱ ~10 min

1. Go to [https://appstoreconnect.apple.com/access/integrations/api](https://appstoreconnect.apple.com/access/integrations/api).
2. Click **+** to generate a new key. Name it `ingitdb-notarization`, role **Developer**.
3. Download the `.p8` file (e.g., `AuthKey_XXXXXXXXXX.p8`) — **you can only download it once**.
4. Note the **Key ID** (10-character string, e.g., `ABC1234567`) — this is `NOTARIZE_KEY_ID`.
5. Note the **Issuer ID** shown at the top of the page (UUID format) — this is `NOTARIZE_ISSUER_ID`.
6. Store the `.p8` file in a secure location.

---

## Phase 2: Prepare Secrets for CI

### [CLI] 2.1 Attach the Apple certificate chain using quill ⏱ ~10 min

GoReleaser uses [Quill](https://github.com/anchore/quill) to sign macOS binaries. Quill requires
the full Apple certificate chain to be embedded in the `.p12` file. The Keychain export only
includes the leaf certificate, so run `quill p12 attach-chain` to embed the chain:

```bash
# Install quill (no Homebrew tap — download directly)
curl -L https://github.com/anchore/quill/releases/download/v0.5.1/quill_0.5.1_darwin_arm64.tar.gz \
  | tar -xz quill
sudo mv quill /usr/local/bin/quill

# Set the p12 password without it touching shell history
read -s QUILL_P12_PASSWORD && export QUILL_P12_PASSWORD

# Attach the chain — creates cert-with-chain.p12
quill p12 attach-chain cert.p12
# Output should show Signing Certificate + Certificate Chain (2 certs)
# and end with: Wrote new p12 file ... to "cert-with-chain.p12"
```

> **Why not `-passin pass:...`?** Passing passwords via CLI flags writes them to shell history
> and exposes them in `ps` output. Using `read -s` + env var is safer.

### [CLI] 2.2 Encode and store the 5 GitHub Secrets ⏱ ~10 min

```bash
# Encode the chain-attached p12 to base64
base64 -i cert-with-chain.p12 | tr -d '\n' | \
  gh secret set MACOS_SIGN_P12 --repo ingitdb/ingitdb-cli

# Set remaining secrets — each prompts for input
gh secret set MACOS_SIGN_PASSWORD --repo ingitdb/ingitdb-cli  # password from export step
gh secret set NOTARIZE_ISSUER_ID  --repo ingitdb/ingitdb-cli
gh secret set NOTARIZE_KEY_ID     --repo ingitdb/ingitdb-cli

# Encode the .p8 API key to base64
base64 -i AuthKey_XXXXXXXXXX.p8 | tr -d '\n' | \
  gh secret set NOTARIZE_KEY --repo ingitdb/ingitdb-cli
```

| Secret name | Value |
|---|---|
| `MACOS_SIGN_P12` | `base64 -i cert.p12` output (single line) |
| `MACOS_SIGN_PASSWORD` | Password chosen when exporting .p12 |
| `NOTARIZE_ISSUER_ID` | UUID from App Store Connect → Integrations → Keys |
| `NOTARIZE_KEY_ID` | 10-character key ID from the same page |
| `NOTARIZE_KEY` | `base64 -i AuthKey_XXXXXXXXXX.p8` output (single line) |

---

## Phase 3: Update GoReleaser Config

### [CODE] 3.1 Add notarize block to `.github/goreleaser-macos.yaml` ⏱ ~5 min

Add the following block between `checksum` and `release` in `.github/goreleaser-macos.yaml`:

```yaml
notarize:
  macos:
    - enabled: '{{ isEnvSet "MACOS_SIGN_P12" }}'
      sign:
        certificate: "{{.Env.MACOS_SIGN_P12}}"
        password: "{{.Env.MACOS_SIGN_PASSWORD}}"
      notarize:
        issuer_id: "{{.Env.NOTARIZE_ISSUER_ID}}"
        key_id:    "{{.Env.NOTARIZE_KEY_ID}}"
        key:       "{{.Env.NOTARIZE_KEY}}"
        wait:      true
        timeout:   20m
```

The `enabled` guard ensures that local builds and CI runs without secrets still work without errors.
GoReleaser automatically creates a temporary keychain, imports the certificate, runs `codesign` on
every darwin binary, submits to Apple for notarization, waits for approval, then staples the ticket.

---

## Phase 4: Update GitHub Actions Workflow

### [CODE] 4.1 Switch goreleaser job to `macos-latest` ⏱ ~5 min

In `.github/workflows/release.yml`, change the `goreleaser` job:

```yaml
# Before
goreleaser:
  runs-on: ubuntu-latest

# After
goreleaser:
  runs-on: macos-latest
```

Also add the 5 new environment variables to the goreleaser step:

```yaml
- uses: goreleaser/goreleaser-action@v6
  with:
    version: v2
    args: release --clean --config .github/goreleaser-macos.yaml
  env:
    GITHUB_TOKEN:          ${{ secrets.INGITDB_GORELEASER_GITHUB_TOKEN }}
    MACOS_SIGN_P12:        ${{ secrets.MACOS_SIGN_P12 }}
    MACOS_SIGN_PASSWORD:   ${{ secrets.MACOS_SIGN_PASSWORD }}
    NOTARIZE_ISSUER_ID:    ${{ secrets.NOTARIZE_ISSUER_ID }}
    NOTARIZE_KEY_ID:       ${{ secrets.NOTARIZE_KEY_ID }}
    NOTARIZE_KEY:          ${{ secrets.NOTARIZE_KEY }}
```

GoReleaser handles keychain creation and teardown internally — no extra keychain steps are needed.

---

## Phase 5: Local Verification (Optional)

### [CLI] 5.1 Verify signing and notarization locally ⏱ ~30 min

After a successful release, download the darwin tarball and verify:

```bash
# Download and extract
curl -L https://github.com/ingitdb/ingitdb-cli/releases/latest/download/ingitdb_<version>_darwin_arm64.tar.gz \
  | tar -xz

# Check Gatekeeper acceptance (expect "source=Notarized Developer ID")
spctl --assess --verbose=4 ./ingitdb

# Verify code signature
codesign --verify --deep --verbose=2 ./ingitdb

# Check notarization ticket is stapled
xcrun stapler validate ./ingitdb

# Check signing identity
codesign -dv ./ingitdb 2>&1 | grep Authority
```

To verify notarization via Apple's servers (requires internet):

```bash
xcrun notarytool history \
  --issuer "$NOTARIZE_ISSUER_ID" \
  --key-id "$NOTARIZE_KEY_ID" \
  --key /path/to/AuthKey_XXXXXXXXXX.p8
```

---

## Phase 6: Release & Verify

### [CLI] 6.1 Push a tag to trigger release ⏱ ~10 min + ~30 min CI

```bash
# Trigger release via workflow_dispatch in Actions UI, or push a tag:
git tag v<next-version>
git push origin v<next-version>
```

Then:

1. Go to **Actions** → **Release** → confirm the `goreleaser` job runs on `macos-latest`.
2. In CI logs, look for `codesign` output (signing step) and `notarytool` output (notarization step).
3. Both should show `accepted` / `success`.

### [CLI] 6.2 Verify on a fresh Mac ⏱ ~10 min

On a Mac that has never run ingitdb before (or after clearing quarantine attributes):

```bash
brew tap ingitdb/cli
brew install ingitdb
ingitdb version   # should open without any Gatekeeper dialog
```

For existing installs, always run `brew update` first to refresh the tap index before upgrading:

```bash
brew update && brew upgrade ingitdb
ingitdb version
```

To verify the binary directly:

```bash
# Download and extract
tar -xzf ingitdb_*_darwin_arm64.tar.gz

# Check Gatekeeper (expect "source=Notarized Developer ID")
spctl --assess --verbose ./ingitdb

# Verify code signature
codesign --verify --deep --verbose=2 ./ingitdb
./ingitdb version
```

> If macOS still shows "Apple could not verify..." on first launch after upgrade, go to
> **System Settings → Privacy & Security** and click **Open Anyway**. This only happens
> once per binary — subsequent launches are silent.

---

## Troubleshooting

### "failed to verify certificate chain: x509: unhandled critical extension"

Two possible causes:

1. **Wrong certificate type** — the `.p12` contains an "Apple Development" cert (for Xcode device
   testing) instead of a "Developer ID Application" cert (for distribution). Check with:
   ```bash
   openssl pkcs12 -in cert.p12 -clcerts -nokeys -legacy | openssl x509 -noout -subject
   # Must show: CN="Developer ID Application: ..."
   # Not:       CN="Apple Development: ..."
   ```
   If wrong, create the correct cert in the Apple Developer Portal (see Phase 1.1).

2. **Missing certificate chain** — the `.p12` was exported without the Apple intermediate certs
   that Quill needs to verify the chain. Run `quill p12 attach-chain` (see Phase 2.1).

### "errSecInternalComponent" during signing

The certificate is not accessible in the keychain. GoReleaser creates a temporary keychain
automatically — make sure the `MACOS_SIGN_P12` and `MACOS_SIGN_PASSWORD` secrets are correct
and the base64 encoding does not have embedded newlines (use `tr -d '\n'` when encoding).

### "The signature does not include a secure timestamp"

Ensure you are using a GoReleaser version that passes `--timestamp` to `codesign`. GoReleaser v2
does this automatically. If needed, upgrade the goreleaser-action version.

### Notarization returns "Invalid" status

Common causes:
- Binary is not signed before notarization (check signing step runs first)
- Wrong Issuer ID or Key ID (double-check in App Store Connect)
- The `.p8` key file was base64-encoded with extra whitespace — re-encode with `tr -d '\n'`
- The API key does not have the Developer role — check App Store Connect → Users & Access

### "Package Approved" but `spctl` still says "rejected"

Run `xcrun stapler validate ./ingitdb` — if the ticket is not stapled, GoReleaser's staple step
may have failed. Check CI logs for staple errors. Manually staple with:
```bash
xcrun stapler staple ./ingitdb
```

### CI fails because `codesign` is not available

Ensure the goreleaser job uses `runs-on: macos-latest`. The `codesign` tool is only available
on macOS runners.

### Local builds fail because secrets are not set

The `enabled: '{{ isEnvSet "MACOS_SIGN_P12" }}'` guard in goreleaser-macos.yaml means signing is
skipped when `MACOS_SIGN_P12` is not in the environment. Local builds will produce unsigned
binaries, which is expected.

---

## References

- [Apple: Notarizing macOS Software Before Distribution](https://developer.apple.com/documentation/security/notarizing_macos_software_before_distribution)
- [Apple: Creating Distribution-Ready Apps](https://developer.apple.com/documentation/xcode/notarizing_macos_software_before_distribution)
- [GoReleaser: macOS Notarization](https://goreleaser.com/customization/notarize/)
- [Anchore Quill: cross-platform macOS code signing](https://github.com/anchore/quill)
- [xcrun notarytool man page](https://developer.apple.com/documentation/security/notarizing_macos_software_before_distribution/customizing_the_notarization_workflow)
- [App Store Connect API Keys](https://developer.apple.com/documentation/appstoreconnectapi/creating_api_keys_for_app_store_connect_api)

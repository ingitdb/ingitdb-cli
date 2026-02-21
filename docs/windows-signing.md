# Windows Code Signing

## Overview

Windows SmartScreen prevents users from running binaries downloaded from the internet that lack a
trusted code-signing certificate. When an unsigned `.exe` is downloaded, SmartScreen shows a
"Windows protected your PC" dialog and requires the user to click "More info" → "Run anyway".
Signing with a trusted certificate from a commercial CA removes this warning. An OV (Organization
Validation) certificate removes the warning once the binary has accumulated SmartScreen reputation;
an EV (Extended Validation) certificate removes the warning immediately on first run.

Windows signing uses a PFX/P12 certificate from a commercial CA (DigiCert, Sectigo, GlobalSign,
etc.) — not Apple. GoReleaser v2 supports cross-platform Windows signing via `notarize.windows`,
which calls `osslsigncode` under the hood. The existing `macos-latest` CI runner can sign Windows
binaries after installing `osslsigncode` via Homebrew — no separate Windows runner is needed.

## Architecture

```
Commercial CA (DigiCert / Sectigo / GlobalSign)
  └─ Code Signing Certificate (.pfx / .p12)    ──┐
                                                  ▼
                                       GitHub Secrets (2 secrets)
                                                  │
                                                  ▼
                                       GitHub Actions (macos-latest runner)
                                                  │
                                                  ▼
                                       GoReleaser
                                         ├─ build ingitdb.exe (windows/amd64)
                                         └─ sign with osslsigncode (PFX cert)
                                                  │
                                                  ▼
                                  GitHub Release (signed ingitdb.exe in .zip)
                                                  │
                                                  ▼
                                       End user (no SmartScreen dialog)
```

## Quick Reference

| Phase | Step | Method | Estimate |
|-------|------|--------|----------|
| 1 | Purchase and obtain code signing certificate | Manual (CA portal) | ~1–3 days |
| 1 | Export certificate as PFX | Manual | ~10 min |
| 2 | Encode PFX + store 2 GitHub secrets | CLI | ~10 min |
| 3 | Update GoReleaser config | Code | ~5 min |
| 4 | Update GitHub Actions workflow | Code | ~5 min |
| 5 | Local verification with osslsigncode | CLI | ~15 min |
| 6 | Trigger release + verify on fresh Windows machine | CLI | ~10 min + CI time |

---

## Phase 1: Obtain a Code Signing Certificate

### [MANUAL] 1.1 Choose a CA and certificate type ⏱ ~1–3 days

**OV (Organization Validation) certificates** — suitable for open-source and small commercial
projects. The CA verifies your organization's existence. SmartScreen warning is removed once the
signed binary accumulates enough download reputation (typically days to weeks after the first
release).

**EV (Extended Validation) certificates** — required for hardware tokens (USB dongles or cloud
HSMs) and provide instant SmartScreen reputation. More expensive and involve stricter vetting.

Recommended CAs:
- [DigiCert](https://www.digicert.com/signing/code-signing-certificates) — industry standard, EV and OV
- [Sectigo](https://sectigo.com/ssl-certificates-tls/code-signing) — competitive pricing, OV and EV
- [GlobalSign](https://www.globalsign.com/en/code-signing-certificate/) — OV and EV

> For most open-source projects, an OV certificate is sufficient. EV certificates require
> hardware token delivery, which adds complexity to CI/CD pipelines.

### [MANUAL] 1.2 Export the certificate as PFX ⏱ ~10 min

After the CA issues the certificate and you have installed it (process varies by CA):

```bash
# On macOS — export from Keychain Access:
# 1. Open Keychain Access → My Certificates
# 2. Right-click the code signing cert → Export
# 3. Choose format: Personal Information Exchange (.p12) → save as cert.pfx
# 4. Enter a strong export password — you will need this as WINDOWS_SIGN_PASSWORD

# Or using openssl if you have the cert and key as separate files:
openssl pkcs12 -export \
  -inkey private.key \
  -in certificate.crt \
  -certfile ca-chain.crt \
  -out cert.pfx \
  -passout pass:"$WINDOWS_SIGN_PASSWORD"
```

Keep `cert.pfx` and the password in a secure location (password manager recommended).

---

## Phase 2: Prepare Secrets for CI

### [CLI] 2.1 Encode and store the 2 GitHub Secrets ⏱ ~10 min

```bash
# Encode the PFX to base64 (single line, no newlines)
base64 -i cert.pfx | tr -d '\n' | \
  gh secret set WINDOWS_SIGN_CERTIFICATE --repo ingitdb/ingitdb-cli

# Set the PFX password
gh secret set WINDOWS_SIGN_PASSWORD --repo ingitdb/ingitdb-cli
# (enter the password when prompted)
```

| Secret name | Value |
|---|---|
| `WINDOWS_SIGN_CERTIFICATE` | `base64 -i cert.pfx \| tr -d '\n'` output (single line) |
| `WINDOWS_SIGN_PASSWORD` | Password chosen when exporting the PFX |

---

## Phase 3: Update GoReleaser Config

### [CODE] 3.1 Add `notarize.windows` block to `.github/goreleaser.yaml` ⏱ ~5 min

Add the following `windows` section inside the existing `notarize:` block (after the `macos:`
section):

```yaml
notarize:
  macos:
    - ... (existing, unchanged)
  windows:
    - enabled: '{{ isEnvSet "WINDOWS_SIGN_CERTIFICATE" }}'
      ids: [ ingitdb-windows ]
      sign:
        certificate: "{{.Env.WINDOWS_SIGN_CERTIFICATE}}"
        password:    "{{.Env.WINDOWS_SIGN_PASSWORD}}"
```

The `enabled` guard ensures local builds and CI runs without secrets skip signing without errors.
GoReleaser calls `osslsigncode` to sign the `.exe` using the PFX certificate.

---

## Phase 4: Update GitHub Actions Workflow

### [CODE] 4.1 Install osslsigncode and add secrets to release.yml ⏱ ~5 min

In `.github/workflows/release.yml`, add a step **before** the goreleaser step:

```yaml
- name: Install osslsigncode (Windows signing)
  run: brew install osslsigncode
```

Add the two new env vars to the `goreleaser-action` step:

```yaml
- uses: goreleaser/goreleaser-action@v6
  with:
    version: v2
    args: release --clean --config .github/goreleaser.yaml
  env:
    ...existing vars...
    WINDOWS_SIGN_CERTIFICATE: ${{ secrets.WINDOWS_SIGN_CERTIFICATE }}
    WINDOWS_SIGN_PASSWORD:    ${{ secrets.WINDOWS_SIGN_PASSWORD }}
```

`osslsigncode` is available via Homebrew on the `macos-latest` runner, so no separate Windows
runner is needed.

---

## Phase 5: Local Verification

### [CLI] 5.1 Verify signing with osslsigncode ⏱ ~15 min

After a successful release, download the Windows zip and verify:

```bash
# Install osslsigncode locally (macOS)
brew install osslsigncode

# Extract the zip
unzip ingitdb_<version>_windows_amd64.zip

# Verify the signature
osslsigncode verify ingitdb.exe
# Expect: Signature verification: ok
# and a line showing the signer's CN and the timestamp
```

To inspect the certificate details:

```bash
osslsigncode verify -in ingitdb.exe 2>&1 | grep -E "CN=|Signing time|Verified"
```

### [CLI] 5.2 Verify with signtool on Windows ⏱ ~5 min

On a Windows machine with the Windows SDK installed:

```powershell
# Verify signature
signtool verify /pa /v ingitdb.exe

# Expect output ending with:
# Successfully verified: ingitdb.exe
```

Alternatively, right-click `ingitdb.exe` → **Properties** → **Digital Signatures** tab — the
signer name should match your CA certificate.

---

## Phase 6: Release & Verify

### [CLI] 6.1 Push a tag to trigger release ⏱ ~10 min + CI time

```bash
# Trigger release via workflow_dispatch in Actions UI, or push a tag:
git tag v<next-version>
git push origin v<next-version>
```

Then:

1. Go to **Actions** → **Release** → confirm the `goreleaser` job runs on `macos-latest`.
2. In CI logs, look for the `Install osslsigncode` step completing successfully.
3. Look for `osslsigncode` output in the GoReleaser signing step.

### [CLI] 6.2 Verify on a fresh Windows machine ⏱ ~10 min

1. Download the `.zip` from the GitHub release page.
2. Extract `ingitdb.exe`.
3. Double-click or run from PowerShell — SmartScreen should **not** appear (EV cert), or will
   disappear after the binary accumulates reputation (OV cert).
4. Right-click `ingitdb.exe` → **Properties** → **Digital Signatures** — confirm the signer name.

> With an OV certificate, Windows may still show SmartScreen on the very first release. This is
> expected and will resolve once the signed binary has been downloaded enough times to build
> SmartScreen reputation (typically within days to weeks).

---

## Troubleshooting

### `osslsigncode: command not found` in CI

The `brew install osslsigncode` step must run **before** the goreleaser step. Check that the step
order in `release.yml` is correct.

### "No certificate found matching the criteria"

The `WINDOWS_SIGN_CERTIFICATE` secret may have been encoded with embedded newlines. Re-encode
using `tr -d '\n'`:

```bash
base64 -i cert.pfx | tr -d '\n' | gh secret set WINDOWS_SIGN_CERTIFICATE --repo ingitdb/ingitdb-cli
```

### "Error reading password"

The `WINDOWS_SIGN_PASSWORD` secret must match the password used when exporting the PFX. If the
password contains special shell characters, ensure it is stored as-is (not shell-escaped) in the
GitHub secret.

### "certificate has expired or is not yet valid"

Code signing certificates have a validity period (typically 1–3 years). After expiry, previously
signed binaries remain valid if they were timestamped at signing time (GoReleaser/osslsigncode
always timestamps by default). New binaries will fail to sign — renew the certificate from your CA.

### Signing is skipped (no `osslsigncode` output in logs)

The `enabled: '{{ isEnvSet "WINDOWS_SIGN_CERTIFICATE" }}'` guard means signing is skipped when
the secret is not set. Confirm the `WINDOWS_SIGN_CERTIFICATE` secret exists in the repository's
GitHub Secrets (Settings → Secrets and variables → Actions).

### SmartScreen still appears after signing (OV certificate)

This is expected for OV certificates until the binary accumulates download reputation. EV
certificates bypass this. With an OV cert, the warning disappears for subsequent releases once
reputation is established. Users can dismiss it by clicking "More info" → "Run anyway".

---

## References

- [GoReleaser: Windows Code Signing](https://goreleaser.com/customization/notarize/)
- [osslsigncode on GitHub](https://github.com/mtrojnar/osslsigncode)
- [Microsoft: SmartScreen and code signing](https://learn.microsoft.com/en-us/windows/security/operating-system-security/virus-and-threat-protection/microsoft-defender-smartscreen/microsoft-defender-smartscreen-overview)
- [DigiCert: Code Signing Certificates](https://www.digicert.com/signing/code-signing-certificates)
- [Sectigo: Code Signing](https://sectigo.com/ssl-certificates-tls/code-signing)

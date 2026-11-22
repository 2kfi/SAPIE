# Windows signing notes

`sign.ps1` signs one executable or installer with Authenticode SHA256 and an RFC 3161 timestamp.
Set `SAPIE_SIGNING_THUMBPRINT` in CI from the OV, EV or Azure Trusted Signing certificate.
Set `SIGNTOOL` only when `signtool.exe` is not on `PATH`. Never commit the certificate, its
password or the thumbprint in a workflow file that is public in cleartext.

The timestamp matters. Without it, a signature stops being useful when the certificate expires.
The script verifies the signed file immediately after signing. `build-package.ps1` signs
`SAPIE.exe`, the NSIS setup and the MSI. The portable zip is not itself signed; every executable
inside it is signed, and the update manifest carries the zip SHA256.

SmartScreen reputation builds with downloads and use over time. A new certificate or a new file
hash can show a warning even when the signature verifies. EV certificates usually shorten that
period. Do not hide the warning or tell users it is harmless. Document the first-run prompt in
release notes.

Signing verification in this worktree is NOT VERIFIED. No signing certificate, no `signtool.exe`
and no Windows runner are available here. The scripts are CI-ready, not proof that a signed
artifact exists.

# Dependency policy

CI runs `govulncheck`, `cargo audit` and `npm audit` in the `dependency-audits`
job of `.github/workflows/ci.yml`. A new high or critical finding fails the job.
Accepted findings are listed below and mirrored in `ACCEPTED_NPM_ADVISORIES` in
that workflow. `tools/check-dependency-policy.py` fails when the two lists
differ, so an advisory cannot be accepted by editing CI alone.

Every accepted advisory here is in development tooling (vite, esbuild,
vitest, tinypool and their transitive packages). None is bundled into the LAN
SPA or the Qt launcher UI. npm only offers a major vitest bump as a fix, which
is tracked separately. The findings were last confirmed on 2026-11-21 and are
explained in `docs/SECURITY.md`.

| Advisory | Reason |
| --- | --- |
| GHSA-4w7w-66w2-5vf9 | Dev-only npm tooling, fix needs a major vitest bump |
| GHSA-587h-7wmj-h5pm | Dev-only npm tooling, fix needs a major vitest bump |
| GHSA-5gmw-xhrv-c9v3 | Dev-only npm tooling, fix needs a major vitest bump |
| GHSA-5xrq-8626-4rwp | Dev-only npm tooling, fix needs a major vitest bump |
| GHSA-67mh-4wv8-2f99 | Dev-only npm tooling, fix needs a major vitest bump |
| GHSA-698v-4r3x-3vcv | Dev-only npm tooling, fix needs a major vitest bump |
| GHSA-82fw-gwwq-j7x9 | Dev-only npm tooling, fix needs a major vitest bump |
| GHSA-85c8-ppgw-ccpr | Dev-only npm tooling, fix needs a major vitest bump |
| GHSA-9jrq-w75r-8gcw | Dev-only npm tooling, fix needs a major vitest bump |
| GHSA-fx2h-pf6j-xcff | Dev-only npm tooling, fix needs a major vitest bump |
| GHSA-qxg6-89mg-rqv8 | Dev-only npm tooling, fix needs a major vitest bump |
| GHSA-r825-w8mm-5pxw | Dev-only npm tooling, fix needs a major vitest bump |
| GHSA-rq7h-c2jc-7f22 | Dev-only npm tooling, fix needs a major vitest bump |
| GHSA-v6wh-96g9-6wx3 | Dev-only npm tooling, fix needs a major vitest bump |

Go and Cargo have no allowlist. `govulncheck` fails on any finding.
`cargo audit` warnings are printed for review and must be recorded in
`docs/SECURITY.md` before they can be treated as accepted.

## Release evidence

The release bundle carries `SHA256SUMS`, one SPDX SBOM per artifact family,
`THIRD_PARTY_NOTICES.md` and signed `update-stable.json` and
`update-beta.json` manifests. `tools/update-manifest.py verify` checks the
Ed25519 signature, channel, version and artifact digests. The public trust root
is `tools/update-signing-public.pem`. The signing key comes from the
`SAPIE_UPDATE_SIGNING_KEY` repository secret and is never stored in the tree.

`tools/scan-secrets.py` runs in CI on every tracked file. It rejects PEM
private keys and common cloud or chat token formats. It does not attempt
entropy scanning, so a low-entropy password in source is not detected by it.

# SAPIE [VERSION] release notes

**Your games. Your server.** Old Steam games, no Steam client.

Date: [DATE]
Channel: state stable or beta for this build.

## What this release does

Describe the change in one or two factual sentences. Name the games, platforms or routes affected.
Do not use "seamless", "powerful" or "next generation" without a recorded check behind the claim.

## Install

Name the artifact for each supported OS and architecture using the scheme in `plan.md` 13.6.5. Link
to the checksum file. State that a build is signed only when a signature exists.

## What was verified

List each check with the command or run and the recorded evidence file. Copy a result only from a
signed-off gate. Do not promote a template line into a pass.

## What was not verified

State every check that was not run, using the explicit wording of the gate templates in
`docs/verification/`. Name the Windows runner, clean VMs, Qt 6.8, signing secrets, real game run and
screen-reader passes when they were not exercised.

## Known limits

One line per limit, with the next step if one exists. Do not paste raw stack traces.

## Artifacts

Use `[REAL DATA]` for any count or size until a release build produces the number. One `SHA256SUMS`
per release, one SBOM per family. Do not invent a download count, user number or testimonial.

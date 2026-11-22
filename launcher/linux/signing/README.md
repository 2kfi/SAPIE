# Linux update signing

`sign-manifest.sh` signs the shared `sapie-update/v1` manifest with the repository tool. Set
`SAPIE_UPDATE_SIGNING_KEY` in CI to the private key path or PEM text. Never commit the key, a
thumbprint or an inlined PEM. Put the matching public key in the launcher resources and name its
path through `CoreBridge::publicKeyPath()`. That bridge method does not exist yet, so the app side
is an integration gap.

The manifest body is signed over its raw bytes with `openssl pkeyutl -sign -rawin`. The verifier in
`src/linux_updater.cpp` uses the matching `pkeyutl -verify -pubin -rawin`, then checks the schema,
channel, artifact name, HTTPS URL, size and SHA256. A failure leaves the installed version in
place. AppImage rollback keeps a copy until the rename succeeds. Flatpak rollback uses the previous
OSTree deployment. Deb rollback is manual through the package manager.

The signing dry run is NOT VERIFIED here. No key, no CI secret and no signed artifact were
available in this worktree.

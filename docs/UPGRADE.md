# Upgrading from 0.4 to 0.5

This is the documented upgrade path. It is preparation only. No 0.5 tag, release or installer was
published when this record was written, and no upgrade was performed for it. Every step marked NOT
VERIFIED has no passing evidence in this tree. Those steps need a Windows host, signing secrets, a
clean VM or a real game folder.

## What carries over

| Item | 0.4 location | 0.5 location |
| --- | --- | --- |
| Launcher state | per-user data dir `com.sapie.launcher/launcher-state.json` | same file and name |
| Session token | OS keychain entry `com.sapie.launcher` / `session-token` | same entry |
| Game library | `steam_api`, `sapie_server.txt`, `steam_appid.txt` and the original backup in each game folder | unchanged |
| Game handoff token | `steam_settings/sapie_client.json` in the game folder | same path, owner-only ACL on Windows |

The 0.5 core reads the existing state file on load and removes any legacy `password`, `token`,
`session_token` or `secret` keys before use (`launcher/core/src/state.rs`). Neither version stores an
account password on disk.

## Windows

A shipped 0.5 artifact needs Qt 6.8 with `windeployqt`, MSVC, `makensis`, WiX (`heat`, `candle`,
`light`) and an Authenticode thumbprint in `SAPIE_SIGNING_THUMBPRINT`. The orchestrator is
[build-package.ps1](../launcher/windows/packaging/build-package.ps1). It emits
`sapie-launcher-<version>-windows-x64-setup.exe`, `sapie-launcher-<version>-windows-x64.msi`, the
portable zip and `SHA256SUMS.txt`. `-UnsignedDryRun` is for local checks only; its artifacts are not shippable.

1. Sign out in 0.4 so the old session is revoked server side. Copy the state file and each game
   folder before changing the install.
   Status: NOT VERIFIED. Needs a Windows host and a real game folder.
2. Build or obtain a signed 0.5 artifact with `build-package.ps1`. Check its hash against
   `SHA256SUMS.txt` and confirm the installer signature.
   Status: NOT VERIFIED. Needs Windows, the packaging tools and signing secrets. No Windows runner
   or signing credential was available for this record.
3. Install with the per-user NSIS setup (no admin needed) or the per-machine MSI. For the portable
   zip, replace the unpacked folder and keep the previous folder for rollback.
   Status: NOT VERIFIED. Needs a Windows host. No clean Windows 10 or 11 run is recorded.
4. Start 0.5, sign in again, then check the library, friends and one game launch.
   Status: NOT VERIFIED. Needs a real server, account and game folder. No such run is recorded.

## Linux

A shipped Linux artifact is produced by
[build-package.sh](../launcher/linux/packaging/build-package.sh). AppImage needs `linuxdeploy` and
`appimagetool`, the deb needs `dpkg-deb`, and Flatpak needs `flatpak-builder` with the SDK. The
update manifest needs `SAPIE_UPDATE_SIGNING_KEY`. See [BUILD.md](BUILD.md) for the build matrix.

1. Sign out in 0.4. Copy `com.sapie.launcher/launcher-state.json` from the XDG data dir and each game
   folder before changing the install.
   Status: NOT VERIFIED. Needs a Linux host and a real game folder.
2. Build or obtain a signed AppImage, deb or Flatpak and check its hash against `SHA256SUMS.txt` and
   the signed manifest.
   Status: NOT VERIFIED. Needs the packaging tools, a Flatpak SDK and signing secrets. No artifact
   was built or signed for this record.
3. Replace the AppImage, install the deb with the package manager, or update the Flatpak. The deb is
   never replaced by the app itself. A Flatpak needs a filesystem grant for game folders outside the
   sandbox.
   Status: NOT VERIFIED. Needs a Linux host. No clean Ubuntu 22.04, Ubuntu 24.04, Fedora or Arch
   run is recorded.
4. Start 0.5, sign in again, then check the library, friends and one game launch.
   Status: NOT VERIFIED. Needs a real server, account and game folder. No such run is recorded.

## Downgrade safety

Keep the 0.4 installer, a copy of the state file and every original library backup until the upgrade
is confirmed on the target machine. A downgrade must not delete them. Do not remove the keychain
entry when stepping back.

- AppImage: `bash launcher/linux/packaging/rollback.sh appimage <previous> <installed>`. Set
  `SAPIE_EXPECT_SHA256` to the signed manifest checksum to verify the previous file.
  Status: NOT VERIFIED. Needs a Linux host and a previous signed AppImage.
- Flatpak: `bash launcher/linux/packaging/rollback.sh flatpak <commit>`. Flatpak keeps the previous
  deployment. Status: NOT VERIFIED. Needs a Linux host and a Flatpak install.
- deb: reinstall the prior signed package with the package manager.
  Status: NOT VERIFIED. Needs a Linux host and a prior signed package.
- Windows: keep the prior signed installer. The updater keeps a rollback copy when it replaces a
  portable folder. Status: NOT VERIFIED. Needs Windows and signing secrets.

## Verification status

| Check | Result |
| --- | --- |
| Upgrade run, Windows | NOT VERIFIED |
| Upgrade run, Linux | NOT VERIFIED |
| Downgrade run, Windows | NOT VERIFIED |
| Downgrade run, Linux | NOT VERIFIED |
| Library, token, friends and launch after upgrade | NOT VERIFIED |
| Signed artifacts and checksums | NOT VERIFIED |

Local checks that did run are recorded in [the 0.5 release candidate checklist](verification/0.5-rc-checklist.md).
None of them is an upgrade run.

## Sign-off

Recorded by: (empty)
Date: (empty)
Result: not signed off

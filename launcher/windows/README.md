# SAPIE for Windows

Windows platform layer for the 0.5 Qt launcher. This directory adds no webview, no HTML and no
remote UI content. The only network request here is the signed update manifest whose URL comes from
the core. No password is written to disk by any code in this directory.

## What is here

- `src/win_platform.cpp`: native frame help for a dark title bar, tray, jump list, taskbar progress
  and a per-user `QLocalServer` single-instance pipe.
- `src/win_native.cpp`: `%LOCALAPPDATA%\SAPIE`, extended-length paths, `sapie://join/<id>` parsing,
  HKCU protocol and file association registration.
- `src/win_acl.cpp`: atomic token handoff with an explicit protected owner-only DACL.
- `src/win_steamless.cpp`: launches the unmodified Steamless sidecar with no shell and a timeout.
- `src/win_updater.cpp`: HTTPS signed-manifest check, CNG signature verification, SHA256 and
  Authenticode checks, silent per-user NSIS apply, portable folder replace and rollback copy.
- `packaging/`: `windeployqt`, NSIS, MSI, portable zip, icon generation and the release orchestrator.
- `signing/`: Authenticode with timestamp and SmartScreen notes.
- `scripts/check-update-manifest.sh`: verifies a manifest with `tools/update-manifest.py`.

The icons in `design/` are provisional G1 art. The packaging scripts use them but do not call them
final. Long paths are enabled by `resources/sapie.exe.manifest` and the Win32 calls use `\\?\`.

## Integration expected from the shared launcher

This directory is intentionally unaware of `sapie_core.h` types. The shared layers must provide:

1. Top-level `launcher/CMakeLists.txt` sets `SAPIE_VERSION`, `SAPIE_WINDOWS_EXE`, then
   `add_subdirectory(windows)` and links `sapie-windows-platform` into the Qt app target.
2. `launcher/app/main.cpp` calls `WindowsPlatform::sendToRunningInstance(argv)` first. If that
   succeeds, it exits. Otherwise it constructs `WindowsPlatform`, calls `listenForSecondInstance`,
   `attachWindow(window)` and `installTray(icon, menu)`, and registers integrations on first run.
3. `CoreBridge` exposes `currentVersion()`, `updateChannel()`, `updateManifestUrl(channel)` and
   `publicKeyPath()`, plus `handleDeepLink(url)`. Connect `WindowsPlatform::protocolLinkReceived`
   and `secondInstanceLaunched` to `handleDeepLink`. Connect updater `updateAvailable` and `failed`
   to QML-visible signals.
4. `sapie_core.h` exposes the current version, the channel, the HTTPS manifest URL and the bundled
   signing public key path. The core owns the token contents and calls the Windows ACL helper when
   it writes the game token file.
5. The core owns the file extension list passed to `registerIntegrations`. No extension is invented
   here. The protocol path and lobby id validation are implemented here, join resolution stays in
   the core.

## Update contract

The manifest is the format emitted by `tools/update-manifest.py`: schema `sapie-update/v1`,
channels `stable` and `beta`, SHA256 per artifact and a base64 detached signature over the raw
manifest bytes. The updater rejects a bad signature, a channel mismatch, a size or hash mismatch,
a non HTTPS URL and an unsigned installer. A failed apply restores the copied pre-update folder.
Manual rollback from the previous signed release remains available.

## Antivirus and SmartScreen

Sign every shipped executable and installer. Antivirus false positives are possible on a new hash.
Do not disable Defender or tell users to ignore a warning. SmartScreen reputation takes time.
See `signing/README.md`.

## Not verified in this worktree

MSVC and the Windows SDK are absent, so the C++ sources were not compiled. `windeployqt`, NSIS, WiX,
Authenticode signing, `icacls`, Steamless unpacking, a clean Windows 10 and 11 run, the long path
test and the 0.4 to 0.5 update dry run were not executed. Signing verification is NOT VERIFIED.

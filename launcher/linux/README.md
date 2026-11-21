# SAPIE for Linux

Linux platform layer for the 0.5 Qt launcher. This directory adds no webview, no HTML and no remote
UI content. The only network requests are the signed update manifest and its artifact. No password
is written to disk by any code in this directory.

## What is here

- `src/linux_native.cpp`: XDG directories, `sapie://join/<id>` parsing, scheme and MIME default
  registration, and `LD_PRELOAD` launch plans built without a shell.
- `src/linux_token.cpp`: atomic token handoff with mode `0600`, plus a check that rejects group or
  world access.
- `src/linux_platform.cpp`: Qt desktop file name, tray through StatusNotifierItem on the session
  bus, a Unix-socket `QLocalServer` single instance, and session and motion detection.
- `src/linux_updater.cpp`: signed manifest verification through `openssl pkeyutl`, HTTPS artifact
  download, SHA256 check, AppImage replace with a rollback copy, Flatpak update, and a clear refusal
  for package-managed installs.
- `resources/`: desktop entry and AppStream metainfo with `x-scheme-handler/sapie`.
- `packaging/`: hicolor icons, linuxdeploy with the Qt plugin, a private Qt tree under `/opt/SAPIE`
  for the deb, a Flatpak on `org.kde.Platform`, an orchestrator and a rollback helper.
- `signing/`: signed update manifest generation with the shared `tools/update-manifest.py`.
- `tests/linux_selfcheck.cpp`: URL, XDG, token mode, launch plan, manifest shape, session detection
  and single-instance forwarding checks.
- `tests/vm-matrix.sh`: host probe and an evidence template with every matrix row NOT VERIFIED.

## Integration expected from the shared launcher

This directory is intentionally unaware of `sapie_core.h` types. The shared layers must provide:

1. Top-level `launcher/CMakeLists.txt` sets `SAPIE_VERSION`, `SAPIE_ARCH`, `SAPIE_FLATPAK_REPO` and
   `SAPIE_LINUX_EXE`, then calls `add_subdirectory(linux)` and links `sapie-linux-platform` into
   the Qt app target.
2. `launcher/app/main.cpp` calls `LinuxPlatform::sendToRunningInstance(arguments)` first. If that
   succeeds, it exits. Otherwise it constructs `LinuxPlatform`, calls `listenForSecondInstance`,
   `attachWindow(window)` and `installTray("org.sapie.SAPIE")`, then registers the protocol on
   first run.
3. `CoreBridge` exposes `currentVersion()`, `updateChannel()`, `updateManifestUrl(channel)`,
   `publicKeyPath()` and `handleDeepLink(url)`. Connect `LinuxPlatform::protocolLinkReceived` and
   `secondInstanceLaunched` to `handleDeepLink`. Connect `LinuxUpdater` signals to QML-visible
   signals. No such methods exist in the current bridge, so this phase cannot wire deep links or
   updates end to end.
4. `sapie_core.h` exposes the current version, channel, HTTPS manifest URL and bundled signing
   public key path. It also needs join resolution for a parsed lobby id. None of those functions
   are in the current ABI.
5. The core passes the token contents to `writeOwnerOnlyFile(tokenFilePath(), ...)` and calls
   `verifyOwnerOnlyFile` before use. The shared core currently writes its own token file, so this
   helper is not yet on that path.
6. The core owns the MIME type list passed to `registerMimeTypes`. No on-disk file format is
   invented in 0.5, so the only registration here is `x-scheme-handler/sapie`, declared in the
   desktop entry and applied per user by `xdg-mime`.

## Deep links

`sapie://join/<numeric-id>` is accepted with an optional query string. User info, a port, a
fragment, a non numeric id and a different scheme are rejected with a specific message. A second
instance forwards its arguments over the Unix socket to the running launcher. Parsing and forwarding
are implemented here; joining the lobby stays in the core and is an integration gap.

## Frames, sessions and HiDPI

The Linux launcher uses the desktop window frame. Qt supplies Wayland and X11 scaling, and the
shared QML owns the dark surface against the system theme. `desktopSession()` reports `wayland` or
`x11`. On a legacy tray with XEMBED only and no StatusNotifierItem, there is no tray icon. Verify
both platform plugins and both scale factors from `tests/vm-matrix.sh`.

## Updates and rollback

The manifest is the format emitted by `tools/update-manifest.py`: schema `sapie-update/v1`, channels
`stable` and `beta`, SHA256 per artifact and a base64 detached signature over the raw manifest
bytes. `LinuxUpdater` rejects a bad signature, a channel mismatch, a size or hash mismatch and a
non HTTPS URL. AppImage apply copies the running file to a temporary rollback path, stages the new
file beside it and renames over the old one. A failed stage restores the copy. Flatpak apply calls
`flatpak update`, which keeps the previous deployment. Deb installs update through the package
manager and are never replaced by the app. Manual rollback is in `packaging/rollback.sh`.

Signing keys are never committed. Set `SAPIE_UPDATE_SIGNING_KEY` in CI. See `signing/README.md`.

## Flatpak sandbox

The manifest grants IPC, Wayland, fallback X11, the session bus, DRI and the SAPIE XDG directories.
Game folders outside the sandbox need an explicit grant, for example
`flatpak override --filesystem=/path/to/games org.sapie.SAPIE`. The M26 game page must show this
before Play. That UI is not in this phase.

## Not verified in this worktree

Qt 6.12 is installed locally. The plan pins Qt 6.8 LTS, so the local configure is a compatibility
check only. `linuxdeploy`, `appimagetool`, a Flatpak SDK, signing keys, clean VMs, distro packages
and real game folders are all unavailable. No AppImage, deb or Flatpak was built here. Tray
registration on a live panel, Wayland and X11 startup, 1x and 2x screenshots, a clean machine with
no distro Qt, a tampered manifest run, the 0.4 to 0.5 update dry run and the rollback dry run are
NOT VERIFIED. The evidence file is a template, not a pass record.

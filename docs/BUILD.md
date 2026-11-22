# Building SAPIE

Builds are done from the repository root on Arch Linux. The client matrix is
5 target triplets × 2 interface eras = 10 shared libraries. Only the
`steam_api` target is built; overlay, detours, `steamclient`,
`steamnetworkingsockets`, `lobby_connect` and `generate_interfaces_file` are
not part of the matrix.

## Packages

```bash
# official repos
sudo pacman -S --needed \
  base-devel cmake ninja git go python \
  protobuf \
  mingw-w64-gcc \
  aarch64-linux-gnu-gcc \
  lib32-glibc lib32-gcc-libs \
  rust nodejs npm libxkbcommon libwayland libx11 libxi libxrandr mesa

# AUR (yay/paru): cross-built protobuf for Windows targets
yay -S mingw-w64-protobuf
```

- `mingw-w64-gcc` provides both `i686-w64-mingw32-*` and `x86_64-w64-mingw32-*`.
- `aarch64-linux-gnu-gcc` pulls binutils/glibc/headers.
- `lib32-*` may require uncommenting `[multilib]` in `/etc/pacman.conf`.
- Host `protoc` comes from `protobuf`. Protobuf 36 requires C++17; the client
  CMake uses C++17.
- CI uses an `archlinux` container with the same list. The launcher is a Tauri v2 app and its
  `Cargo.lock` pulls `webkit2gtk`. The release workflow (`.github/workflows/build.yml`) installs
  `libwebkit2gtk-4.1-dev`, `libayatana-appindicator3-dev` and `librsvg2-dev` alongside the
  X11/Wayland/GL dev libs on Linux native runners (`windows-latest`, `ubuntu-24.04`,
  `ubuntu-24.04-arm`). The pull-request workflow (`.github/workflows/ci.yml`) installs only the
  narrower X11/Wayland/GL set; see `dev_notes.md` for the unverified gap.

## Cross protobuf-lite (required)

`protoc` runs on the host for all targets, but each cross target needs a
matching **static** `libprotobuf-lite` built with that target's compiler. The
system/host `/usr/lib/libprotobuf-lite.so` is only used for native `linux-x64`.
The pinned source tag is `v36.1` (package version 36.1.0).

```bash
for t in windows-x86 windows-x64 linux-x86 linux-arm64; do
  tools/build-protobuf.sh "$t"
done
# linux-x64 needs no step:
tools/build-protobuf.sh linux-x64
```

Each script run clones into a temporary directory under `/tmp`, configures
with `client/cmake/toolchains/<target>.cmake`, builds only `libprotobuf-lite`,
and installs into `third_party/protobuf/<target>/{include,lib}`. Generated
output is not committed. The script fails with a clear message on any clone,
configure, build or install error.

## Client build matrix

```bash
build() {
  local target="$1" era="$2" tc=()
  [[ "$target" == linux-x64 ]] || \
    tc=(-DCMAKE_TOOLCHAIN_FILE="$PWD/client/cmake/toolchains/$target.cmake")
  cmake -S client -B "build/client/$target-$era" -G Ninja \
    -DCMAKE_BUILD_TYPE=Release \
    -DSAPIE_INTERFACE_ERA="$era" \
    -DCMAKE_LIBRARY_OUTPUT_DIRECTORY="$PWD/out/client/$target-$era" \
    "${tc[@]}"
  cmake --build "build/client/$target-$era" --parallel
}

build windows-x86 legacy
build windows-x86 modern
build windows-x64 legacy
build windows-x64 modern
build linux-x86 legacy
build linux-x86 modern
build linux-x64 legacy
build linux-x64 modern
build linux-arm64 legacy
build linux-arm64 modern
```

`SAPIE_INTERFACE_ERA=legacy` defines `SAPIE_LEGACY_INTERFACES`; `modern`
defines nothing. Interface string tables are wired up in a later milestone, so
both eras currently produce the same upstream Goldberg interface set.

## Output layout

```
out/client/
├── windows-x86-legacy/steam_api.dll
├── windows-x86-modern/steam_api.dll
├── windows-x64-legacy/steam_api64.dll
├── windows-x64-modern/steam_api64.dll
├── linux-x86-legacy/libsteam_api.so
├── linux-x86-modern/libsteam_api.so
├── linux-x64-legacy/libsteam_api.so
├── linux-x64-modern/libsteam_api.so
├── linux-arm64-legacy/libsteam_api.so
└── linux-arm64-modern/libsteam_api.so
```

CMake also emits an import library (`libsteam_api*.dll.a`) next to each
Windows DLL; the DLL itself is the artifact to copy next to the game exe.
`CMAKE_LIBRARY_OUTPUT_DIRECTORY` is mirrored to
`CMAKE_RUNTIME_OUTPUT_DIRECTORY` so Windows DLLs land in the same directory.

## Windows runtime dependencies

The Windows toolchains (`windows-x86.cmake`, `windows-x64.cmake`) link with
`-static-libgcc -static-libstdc++ -static`, so `steam_api.dll` and
`steam_api64.dll` import no `libwinpthread-1.dll` and ship self-contained.
If a future toolchain change drops those flags, the launcher warns about the
missing runtime DLL, which is never bundled. Linux x64 is not static: it uses
the host `libprotobuf-lite.so` by design.

## Known gaps and build notes

- Cross `protobuf-lite` is the hard prerequisite. A cross build with no
  `third_party/protobuf/<target>/` present fails at compile (missing
  `google/protobuf/message_lite.h`) or at link (protobuf/absl symbols), never
  against the host `.so`. An installed distro `mingw-w64-protobuf` may be a
  different protobuf/absl version than host `protoc` 36.1 and will fail to link.
- `dll/wrap.cpp` maps the glibc ≤2.32 `__xstat`/`__lxstat`/`__xmknod` calls to
  modern `stat`/`lstat`/`mknod` on glibc 2.33+.
- MinGW builds define `_S4N_` so the Steamworks headers keep by-value virtual
  returns, matching Goldberg's implementations.
- Windows builds define `STEAM_WIN32`; `utf8_decode` returns a wrapper that is
  both a `std::wstring` and a `const wchar_t*` so GCC 16 `fstream` overloads
  resolve.
- `glew/` is absent from the upstream reference snapshot; the overlay is
  excluded from the matrix, so it is not needed.
- LGPLv3 headers in all vendored files are unchanged.

## Native launcher

`client/src-tauri/` is a Tauri v2 app. The Rust shell is in `src/` and the bundled React UI is in
`ui/`. The directory keeps its historical name; no egui source remains. The binary is
`sapie-launcher` (`sapie-launcher.exe` on Windows), and Windows builds set
`windows_subsystem = "windows"`.

```sh
cd client/src-tauri && cargo build --release
cd client/src-tauri/ui && npm ci && npm run build
```

The shell bundle is loaded from `ui/dist` with a locked CSP and no remote URLs, relative to
`client/src-tauri`. On Linux the release build needs the webkit2gtk and X11/Wayland/GL development
packages listed above. No cross-compiled Windows launcher has been executed locally.

## Web UI build

```sh
cd web && npm ci && npm run build   # tsc --noEmit + vite build -> web/dist
```

The server hosts `web/dist` with `--ui-dir` (default `./web/dist`) on `--ui-listen`
(default `0.0.0.0:8081`). No browser click-through has been done locally.

## CI and releases

`.github/workflows/ci.yml` runs on pushes to `dev-client`, pull requests to `dev-client` and
`main-client`, and manual dispatch. It runs proto sync, tracked-build-product checks, the 10-target
client matrix, server tests, launcher Rust checks and build, launcher UI and web builds and tests,
and the dependency audits.

`.github/workflows/build.yml` runs only on pushed tags matching plain numeric `0.*`. It stamps the
tag version into `Cargo.toml`, `tauri.conf.json` and both `package.json` files, builds the client,
server and launcher targets, builds `web/dist`, assembles the release archives, creates the GitHub
Release and pushes the server container. `tools/release-bundle.sh` copies `web/dist` to the archive
root as `web/dist`, so `--ui-dir ./web/dist` works from an unpacked bundle. Tag `0.4` run
44882984036 passed and attached NSIS, MSI and AppImage installers. The installers were not executed.

### 0.5 release inputs

`tools/release-bundle.sh` takes `<tag> <artifacts-dir> <output-dir>`. It expects
these artifact directories from the packaging jobs:

- `launcher-windows-x64`, `launcher-linux-x64`, `launcher-linux-arm64`: the
  packaged launcher payload for each OS. The script copies the directory tree to
  `launcher/` inside the archive.
- `client-<target>-<era>` for the existing five targets and two eras: the ten
  `steam_api` variants.
- `web-dist`: the built SPA.
- `server-windows-amd64`, `server-windows-arm64`, `server-linux-amd64`,
  `server-linux-arm64`: plain files or same-named artifact directories.

It writes one `sapie-server-<tag>-<os>-<arch>.<ext>` archive per OS and arch, a
`sapie-server-<tag>-container.tar.gz` with the Dockerfile, compose file, all four
server binaries and `web/dist`, six SPDX SBOMs in `SBOM-<tag>-<family>.spdx.json`,
`THIRD_PARTY_NOTICES.md`, `SHA256SUMS`,
`update-signing-public.pem`, and signed `update-stable.json` and
`update-beta.json` with `.sig` files. The manifests pin the exact tag, version,
per-artifact SHA-256, checksum-file SHA-256 and optional previous version. Verify
one with:

```sh
python3 tools/update-manifest.py verify \
  --manifest update-stable.json --public-key update-signing-public.pem \
  --channel stable --version 0.5.0
```

The signing key is the `SAPIE_UPDATE_SIGNING_KEY` repository secret. It can be
an inline PEM key or a path to one. The public trust root is
`tools/update-signing-public.pem` and ships in every bundle. Signing is a release
gate: the bundle step fails if the key is missing or a manifest does not verify.
Rollback stays manual. Download the previous signed release and reinstall it.

`compose.yaml` publishes the web UI on 8081 and keeps the admin API bound to
loopback inside the container. It declares the same healthcheck as the
Dockerfile. The admin port is not published.

### 0.5 packaging jobs

Two tag-triggered jobs in `build.yml` build the Qt launchers from `launcher/`:

- `launcher-windows-qt` uses Qt 6.8 MSVC, `windeployqt`, NSIS, MSI and a portable
  zip through `launcher/windows/packaging/build-package.ps1`. It uploads
  `launcher-windows-x64` for the release bundle and `installer-windows-qt` for
  the GitHub Release, with `SHA256SUMS.txt`, SPDX SBOMs and a signed stable
  update manifest.
- `launcher-linux-qt` runs on x64 and arm64 runners with Qt 6.8, then calls
  `launcher/linux/packaging/build-package.sh` for AppImage, deb and Flatpak. It
  uploads `launcher-linux-x64` and `launcher-linux-arm64` payloads plus
  `installer-linux-qt-<arch>` with `SHA256SUMS.txt` and SPDX SBOMs. Flatpak is
  built only when the `org.kde` 6.8 runtime and SDK are installed in the runner's
  user Flatpak installation; otherwise the step reports `Flatpak NOT VERIFIED`
  and the release still ships the AppImage and deb.

Repository secrets used by these jobs:

- `SAPIE_SIGNING_THUMBPRINT`: the Authenticode certificate thumbprint for
  `launcher/windows/signing/sign.ps1`. The certificate itself is not in the repo.
- `SAPIE_UPDATE_PUBLIC_KEY`: the update manifest public key, as inline PEM or a
  path. The checked-in `tools/update-signing-public.pem` is the trust root in
  release bundles.
- `SAPIE_UPDATE_SIGNING_KEY`: the signing key for the final release manifests,
  already used by the release job.

No value for these secrets is stored in the tree. Windows runner, MSVC, signing secrets and the
Flatpak SDK were not available locally, so those jobs are NOT VERIFIED here. linuxdeploy and
appimagetool were fetched to a temporary directory for a local AppImage content check; the shipped
AppImage comes from CI.

The 0.1 release is published and frozen at tag `0.1` (`62007f5`) at
https://github.com/2kfi/SAPIE/releases/tag/0.1 with the assets
`SAPIE-0.1-linux-x64.tar.gz`, `SAPIE-0.1-linux-arm64.tar.gz` and `SAPIE-0.1-windows-x64.zip`.
Its assets and GHCR image are unchanged. Container tags are
`ghcr.io/2kfi/sapie-server:<tag>` and `ghcr.io/2kfi/sapie-server:latest`
(for 0.1, `ghcr.io/2kfi/sapie-server:0.1`). 0.2 is tagged `0.2` and published at
https://github.com/2kfi/SAPIE/releases/tag/0.2, with images
`ghcr.io/2kfi/sapie-server:0.2` and `ghcr.io/2kfi/sapie-server:latest`. Tag `0.3` exists on a
docs-only commit on `dev-client`; no asset claim is made here. Tag `0.4` does not exist. 0.1 remains
frozen at `62007f5`; G0 is deferred to the user.

`ci.yml` accepts manual dispatch (`workflow_dispatch --ref <branch>`). `build.yml` has no manual
dispatch trigger; only a pushed `0.*` tag starts it. No such run has been confirmed in this tree.

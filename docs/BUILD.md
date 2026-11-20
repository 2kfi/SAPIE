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

# AUR (yay/paru) — cross-built protobuf for Windows targets
yay -S mingw-w64-protobuf
```

- `mingw-w64-gcc` provides both `i686-w64-mingw32-*` and `x86_64-w64-mingw32-*`.
- `aarch64-linux-gnu-gcc` pulls binutils/glibc/headers.
- `lib32-*` may require uncommenting `[multilib]` in `/etc/pacman.conf`.
- Host `protoc` comes from `protobuf`. Protobuf 36 requires C++17; the client
  CMake uses C++17.
- CI uses an `archlinux` container with the same list; the native launcher
  builds on native runners (`windows-latest`, `ubuntu-24.04`, `ubuntu-24.04-arm`)
  with libxkbcommon/wayland/x11/xi/xrandr/gl1-mesa dev libs. There is no
  webkit2gtk dependency.

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

`client/src-tauri/` is now a native Rust egui/eframe app; the directory name is kept, but there is
no Tauri webview, WebKit, Electron or embedded SPA. The binary is `sapie-launcher`
(`sapie-launcher.exe` on Windows), and Windows builds set `windows_subsystem = "windows"`.

```sh
cd client/src-tauri && cargo build --release
```

On Linux the native GUI needs libxkbcommon, wayland, x11, xi, xrandr and gl1-mesa development
libraries. No cross-compiled Windows launcher has been executed locally.

## Web UI build

```sh
cd web && npm ci && npm run build   # tsc --noEmit + vite build -> web/dist
```

The server hosts `web/dist` with `--ui-dir` (default `./web/dist`) on `--ui-listen`
(default `0.0.0.0:8081`). No browser click-through has been done locally.

## CI and releases

Any tag triggers `.github/workflows/build.yml`: it builds the client, server and native launcher
targets, plus a `web` job that builds `web/dist` and uploads it as the `web-dist` artifact. The
release job assembles the three archives, creates the GitHub Release and pushes the server
container. `tools/release-bundle.sh` copies `web/dist` to the archive root as `web/dist`, so
`--ui-dir ./web/dist` works from an unpacked bundle.

The 0.1 release is published and frozen at tag `0.1` (`62007f5`) at
https://github.com/2kfi/SAPIE/releases/tag/0.1 with the assets
`SAPIE-0.1-linux-x64.tar.gz`, `SAPIE-0.1-linux-arm64.tar.gz` and `SAPIE-0.1-windows-x64.zip`.
Its assets and GHCR image are unchanged. Container tags are
`ghcr.io/2kfi/sapie-server:<tag>` and `ghcr.io/2kfi/sapie-server:latest`
(for 0.1, `ghcr.io/2kfi/sapie-server:0.1`). 0.2 is code-complete locally but is not tagged
or released; G0 is deferred to the user.

Caveat: in this new repository, push-triggered runs never fired. Use manual dispatch
(`workflow_dispatch --ref <tag>`) to run the workflow.

# Third-party notices

This file ships in every release bundle and in the container bundle. Version
details for code dependencies come from the lockfiles listed here and from the
SPDX SBOM in the same bundle.

## Qt 6.8 (LGPLv3)

Both launchers link Qt dynamically. The Qt modules used are Core, Gui, Quick,
QuickControls2, Network, Svg and the platform plugins required by each OS. Qt is
licensed under LGPLv3. The bundled Qt shared libraries keep their original
notices. To relink against a modified Qt, build the launcher core and platform
shell from source, then link the resulting objects against your Qt build. The
source for the modified SAPIE parts is in this repository.

## Goldberg `steam_api` fork (LGPLv3)

The game-side library in `client/dll/` keeps the upstream LGPLv3 notices. The
fork is published with its modifications.

## Steamless (CC BY-NC-ND 4.0)

The Windows Steamless sidecar is copied unmodified from
`client/src-tauri/resources/steamless`. Its own `NOTICE.md` ships beside it.
Steamless is non-commercial and is not sublicensed by SAPIE.

## Fonts (OFL 1.1)

Manrope is the UI face and IBM Plex Mono is the machine-value face. Both are
licensed under the SIL Open Font License 1.1. The font files ship in the
launcher resources and the web build. Font names are not modified.

## GSAP (web UI only)

The LAN web UI uses `gsap` and `@gsap/react` under the GSAP standard license.
See https://gsap.com/standard-license. GSAP is not part of the Qt launchers.

## Rust crates

The launcher core and shell dependencies are recorded in
`client/src-tauri/Cargo.lock`. The SPDX SBOM lists every crate in that lockfile.

## Go modules

The server dependency set is recorded in `server/go.mod`. The SPDX SBOM lists
every required module.

## Protobuf

The client build uses protobuf-lite v36.1 from vendored build inputs. The
generated protocol code keeps its upstream license header.

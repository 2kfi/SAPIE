#!/usr/bin/env bash
set -euo pipefail

evidence=""
host_only=0
while [[ $# -gt 0 ]]; do
  case "$1" in
    --evidence) evidence="${2:-}"; shift 2 ;;
    --host) host_only=1; shift ;;
    *) echo "usage: $0 [--host] [--evidence <file>]" >&2; exit 2 ;;
  esac
done

if (( host_only )); then
  echo "XDG_SESSION_TYPE=${XDG_SESSION_TYPE:-unset}"
  echo "WAYLAND_DISPLAY=${WAYLAND_DISPLAY:-unset}"
  echo "DISPLAY=${DISPLAY:-unset}"
  echo "QT_QPA_PLATFORM=${QT_QPA_PLATFORM:-unset}"
  echo "QT_SCALE_FACTOR=${QT_SCALE_FACTOR:-unset}"
  echo "Qt platform plugins available to this process:"
  plugin_dir="$(qmake6 -query QT_INSTALL_PLUGINS 2>/dev/null || true)"
  if [[ -d "${plugin_dir}/platforms" ]]; then
    find "${plugin_dir}/platforms" -maxdepth 1 -type f -printf '%f\n' | sort || true
  else
    echo "  none found"
  fi
  echo "This is host information only. It is not VM evidence."
  exit 0
fi

if [[ -z "${evidence}" ]]; then
  echo "usage: $0 [--host] [--evidence <file>]" >&2
  exit 2
fi
mkdir -p "$(dirname "${evidence}")"

cat > "${evidence}" <<'EOF'
# 0.5 Linux distro and desktop matrix, evidence template

This is a template, not a pass record. No clean VM, no signed release artifact and no real game
folder were available in the M25 worktree. Every row stays NOT VERIFIED until a runner with the
matching desktop captures a screenshot and a launcher log. The M25-e owner copies the completed
table to `docs/verification/0.5-linux-matrix.md`.

| Distro | Desktop | Session | Scale | Install kind | Result | Evidence |
| --- | --- | --- | --- | --- | --- | --- |
| Ubuntu 24.04 | GNOME | Wayland | 1x | AppImage | NOT VERIFIED | no VM run |
| Ubuntu 24.04 | GNOME | Wayland | 2x | AppImage | NOT VERIFIED | no VM run |
| Ubuntu 24.04 | GNOME | X11 | 1x | deb | NOT VERIFIED | no VM run |
| Ubuntu 22.04 | GNOME | X11 | 2x | deb | NOT VERIFIED | no VM run |
| KDE neon | KDE Plasma | Wayland | 1x | Flatpak | NOT VERIFIED | no VM run |
| KDE neon | KDE Plasma | Wayland | 2x | Flatpak | NOT VERIFIED | no VM run |
| Fedora | GNOME | Wayland | 1x | AppImage | NOT VERIFIED | no VM run |
| Fedora | KDE Plasma | X11 | 2x | AppImage | NOT VERIFIED | no VM run |
| Arch | KDE Plasma | Wayland | 2x | Flatpak | NOT VERIFIED | no VM run |
| Ubuntu 24.04 arm64 | GNOME | Wayland | 1x | AppImage | NOT VERIFIED | no arm64 runner |

## Commands for each row

    export QT_QPA_PLATFORM=wayland   # or xcb for X11
    export QT_SCALE_FACTOR=2         # omit for 1x
    sapie-launcher --version
    sapie-launcher

Record the window size, the tray icon presence, and whether `sapie://join/123` from the browser
opens the running launcher. A row passes only with the screenshot and the command output attached.

## Sandbox note

The Flatpak grants XDG dirs for SAPIE data, config, cache and runtime plus Wayland, fallback X11,
the session bus and DRI. A game directory outside the sandbox needs an explicit grant:

    flatpak override --filesystem=/path/to/games org.sapie.SAPIE

The game page in M26 must show that a grant is needed before Play is pressed. That UI is outside
this phase and is NOT VERIFIED here.
EOF

echo "wrote ${evidence} with all rows NOT VERIFIED"

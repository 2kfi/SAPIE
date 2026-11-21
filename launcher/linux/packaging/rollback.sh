#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 2 ]]; then
  echo "usage: $0 appimage <previous-appimage> <installed-appimage>" >&2
  echo "       $0 flatpak <commit>" >&2
  exit 2
fi

kind="$1"
case "${kind}" in
  appimage)
    if [[ $# -ne 3 ]]; then
      echo "error: appimage rollback needs the previous and installed paths" >&2
      exit 2
    fi
    previous="$2"
    installed="$3"
    for path in "${previous}" "${installed}"; do
      if [[ ! -f "${path}" ]]; then
        echo "error: file not found: ${path}" >&2
        exit 1
      fi
    done
    if [[ -n "${SAPIE_EXPECT_SHA256:-}" ]]; then
      actual="$(sha256sum "${previous}" | awk '{print $1}')"
      if [[ "${actual}" != "${SAPIE_EXPECT_SHA256}" ]]; then
        echo "error: previous AppImage checksum does not match the signed manifest" >&2
        exit 1
      fi
    fi
    mv -f "${previous}" "${installed}"
    echo "restored ${installed} from ${previous}"
    ;;
  flatpak)
    if [[ $# -ne 2 ]]; then
      echo "error: flatpak rollback needs one commit" >&2
      exit 2
    fi
    if ! command -v flatpak >/dev/null 2>&1; then
      echo "error: flatpak is not installed" >&2
      exit 1
    fi
    flatpak update -y --commit="$2" org.sapie.SAPIE
    echo "requested Flatpak commit $2"
    ;;
  *)
    echo "error: unknown rollback kind: ${kind}" >&2
    exit 2
    ;;
esac

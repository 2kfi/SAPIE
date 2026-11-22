#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 8 ]]; then
  echo "usage: $0 <exe> <repo-root> <resources-dir> <hicolor-dir> <output-dir> <arch> <version> <flatpak-repo>" >&2
  exit 2
fi

exe="$1"
repo_root="$2"
resources_dir="$3"
hicolor_dir="$4"
output_dir="$5"
arch="$6"
version="$7"
flatpak_repo="$8"
scripts="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

mkdir -p "${output_dir}"

if command -v linuxdeploy >/dev/null 2>&1 && command -v appimagetool >/dev/null 2>&1; then
  "${scripts}/build-appimage.sh" "${exe}" "${repo_root}" "${resources_dir}" "${hicolor_dir}" \
    "${output_dir}/appimage" "${arch}" "${version}"
else
  echo "AppImage NOT VERIFIED: linuxdeploy or appimagetool is not installed"
fi

if command -v dpkg-deb >/dev/null 2>&1; then
  "${scripts}/build-deb.sh" "${exe}" "${resources_dir}" "${hicolor_dir}" \
    "${output_dir}/deb" "${version}" "${arch}"
else
  echo "deb NOT VERIFIED: dpkg-deb is not installed"
fi

if command -v flatpak-builder >/dev/null 2>&1 && command -v flatpak >/dev/null 2>&1 \
  && flatpak info --user "org.kde.Platform//6.8" >/dev/null 2>&1 \
  && flatpak info --user "org.kde.Sdk//6.8" >/dev/null 2>&1; then
  "${scripts}/build-flatpak.sh" "${repo_root}" "${output_dir}/flatpak" \
    "${arch}" "${version}" "${flatpak_repo}"
else
  echo "Flatpak NOT VERIFIED: flatpak-builder, flatpak or the org.kde 6.8 runtime and SDK are not installed"
fi

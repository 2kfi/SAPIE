#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 5 ]]; then
  echo "usage: $0 <repo-root> <output-dir> <arch> <version> <flatpak-repo>" >&2
  exit 2
fi

repo_root="$1"
output_dir="$2"
arch="$3"
version="$4"
flatpak_repo="$5"

if ! command -v flatpak-builder >/dev/null 2>&1 || ! command -v flatpak >/dev/null 2>&1; then
  echo "error: flatpak-builder and flatpak are required" >&2
  exit 1
fi
manifest="${repo_root}/launcher/linux/packaging/org.sapie.SAPIE.yaml"
if [[ ! -f "${manifest}" ]]; then
  echo "error: flatpak manifest not found: ${manifest}" >&2
  exit 1
fi
if [[ ! -d "${flatpak_repo}" ]]; then
  echo "error: flatpak repo not found: ${flatpak_repo}" >&2
  exit 1
fi

mkdir -p "${output_dir}"
flatpak-builder --force-clean --user --install "${output_dir}/build" "${manifest}"
bundle="${output_dir}/sapie-launcher-${version}-linux-${arch}.flatpak"
flatpak build-bundle "${flatpak_repo}" "${bundle}" org.sapie.SAPIE stable
echo "wrote ${bundle}"

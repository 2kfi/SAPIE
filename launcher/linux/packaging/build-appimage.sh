#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 6 ]]; then
  echo "usage: $0 <exe> <resources-dir> <hicolor-dir> <output-dir> <arch> <version>" >&2
  exit 2
fi

exe="$1"
resources_dir="$2"
hicolor_dir="$3"
output_dir="$4"
arch="$5"
version="$6"

linuxdeploy="${LINUXDEPLOY:-$(command -v linuxdeploy || true)}"
appimagetool="${APPIMAGETOOL:-$(command -v appimagetool || true)}"
if [[ -z "${linuxdeploy}" || ! -x "${linuxdeploy}" ]]; then
  echo "error: linuxdeploy is required (set LINUXDEPLOY to its path)" >&2
  exit 1
fi
if [[ -z "${appimagetool}" || ! -x "${appimagetool}" ]]; then
  echo "error: appimagetool is required (set APPIMAGETOOL to its path)" >&2
  exit 1
fi
if [[ ! -x "${exe}" ]]; then
  echo "error: launcher executable not found: ${exe}" >&2
  exit 1
fi
if [[ ! -d "${hicolor_dir}" ]]; then
  echo "error: hicolor icons not found; run the sapie-linux-icons target first" >&2
  exit 1
fi

app_dir="${output_dir}/SAPIE.AppDir"
rm -rf "${app_dir}"
mkdir -p "${app_dir}/usr/bin" "${app_dir}/usr/share/applications" \
  "${app_dir}/usr/share/metainfo"

install -m755 "${exe}" "${app_dir}/usr/bin/sapie-launcher"
install -m644 "${resources_dir}/org.sapie.SAPIE.desktop" \
  "${app_dir}/usr/share/applications/org.sapie.SAPIE.desktop"
install -m644 "${resources_dir}/org.sapie.SAPIE.metainfo.xml" \
  "${app_dir}/usr/share/metainfo/org.sapie.SAPIE.metainfo.xml"
cp -a "${hicolor_dir}/." "${app_dir}/usr/share/icons/"

qmake_bin="${QMAKE:-$(command -v qmake6 || command -v qmake || true)}"
if [[ -z "${qmake_bin}" ]]; then
  echo "error: qmake6 is required by the linuxdeploy Qt plugin" >&2
  exit 1
fi

QMAKE="${qmake_bin}" "${linuxdeploy}" \
  --appdir "${app_dir}" \
  --executable "${app_dir}/usr/bin/sapie-launcher" \
  --desktop-file "${app_dir}/usr/share/applications/org.sapie.SAPIE.desktop" \
  --icon-file "${hicolor_dir}/256x256/apps/org.sapie.SAPIE.png" \
  --plugin qt

mkdir -p "${output_dir}"
appimage="${output_dir}/SAPIE-${version}-linux-${arch}.AppImage"
"${appimagetool}" "${app_dir}" "${appimage}"
echo "wrote ${appimage}"

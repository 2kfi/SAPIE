#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 7 ]]; then
  echo "usage: $0 <exe> <repo-root> <resources-dir> <hicolor-dir> <output-dir> <arch> <version>" >&2
  exit 2
fi

exe="$1"
repo_root="$2"
resources_dir="$3"
hicolor_dir="$4"
output_dir="$5"
arch="$6"
version="$7"

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
for qml_dir in "${repo_root}/launcher/app" "${repo_root}/launcher/ui"; do
  [[ -d "${qml_dir}" ]] || { echo "error: QML source directory not found: ${qml_dir}" >&2; exit 1; }
done
qt_plugin_dir="$("${qmake_bin}" -query QT_INSTALL_PLUGINS)"
wayland_plugins="$(find "${qt_plugin_dir}/platforms" -maxdepth 1 -type f -name 'libqwayland*.so' \
  -printf '%f\n' | LC_ALL=C sort | paste -sd ';' -)"
[[ -n "${wayland_plugins}" ]] || { echo "error: no Wayland platform plugin found in ${qt_plugin_dir}" >&2; exit 1; }

QMAKE="${qmake_bin}" \
QML_SOURCES_PATHS="${repo_root}/launcher/app:${repo_root}/launcher/ui" \
EXTRA_PLATFORM_PLUGINS="${wayland_plugins}" \
"${linuxdeploy}" \
  --appdir "${app_dir}" \
  --executable "${app_dir}/usr/bin/sapie-launcher" \
  --desktop-file "${app_dir}/usr/share/applications/org.sapie.SAPIE.desktop" \
  --icon-file "${hicolor_dir}/256x256/apps/org.sapie.SAPIE.png" \
  --plugin qt

[[ -n "$(find "${app_dir}" -path '*/QtQuick/Window/qmldir' -print -quit)" ]] || {
  echo "error: QtQuick.Window was not bundled into the AppImage" >&2
  exit 1
}
[[ -n "$(find "${app_dir}/usr/plugins/platforms" -maxdepth 1 -type f -name 'libqwayland*.so' -print -quit)" ]] || {
  echo "error: the Wayland platform plugin was not bundled into the AppImage" >&2
  exit 1
}

mkdir -p "${output_dir}"
appimage="${output_dir}/sapie-launcher-${version}-linux-${arch}.AppImage"
"${appimagetool}" "${app_dir}" "${appimage}"
echo "wrote ${appimage}"

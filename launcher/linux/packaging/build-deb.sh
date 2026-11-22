#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 6 ]]; then
  echo "usage: $0 <exe> <resources-dir> <hicolor-dir> <output-dir> <version> <arch>" >&2
  exit 2
fi

exe="$1"
resources_dir="$2"
hicolor_dir="$3"
output_dir="$4"
version="$5"
arch="$6"

if ! command -v dpkg-deb >/dev/null 2>&1; then
  echo "error: dpkg-deb is required" >&2
  exit 1
fi
qmake_bin="${QMAKE:-$(command -v qmake6 || command -v qmake || true)}"
if [[ -z "${qmake_bin}" ]]; then
  echo "error: qmake6 is required to locate the bundled Qt tree" >&2
  exit 1
fi
if [[ ! -x "${exe}" ]]; then
  echo "error: launcher executable not found: ${exe}" >&2
  exit 1
fi
case "${arch}" in
  x64) deb_arch="amd64" ;;
  arm64) deb_arch="arm64" ;;
  *) echo "error: unsupported architecture: ${arch}" >&2; exit 1 ;;
esac

qt_lib_dir="$("${qmake_bin}" -query QT_INSTALL_LIBS)"
qt_plugin_dir="$("${qmake_bin}" -query QT_INSTALL_PLUGINS)"
qt_qml_dir="$("${qmake_bin}" -query QT_INSTALL_QML)"
for path in "${qt_lib_dir}" "${qt_plugin_dir}" "${qt_qml_dir}"; do
  if [[ ! -d "${path}" ]]; then
    echo "error: Qt install path is missing: ${path}" >&2
    exit 1
  fi
done

stage="${output_dir}/sapie-deb"
rm -rf "${stage}"
mkdir -p "${stage}/DEBIAN" "${stage}/opt/SAPIE/bin" "${stage}/opt/SAPIE/lib" \
  "${stage}/opt/SAPIE/plugins" "${stage}/opt/SAPIE/qml" \
  "${stage}/usr/bin" "${stage}/usr/share/applications" \
  "${stage}/usr/share/metainfo" "${stage}/usr/share/icons" "${output_dir}"

install -m755 "${exe}" "${stage}/opt/SAPIE/bin/sapie-launcher.bin"
cp -a "${qt_lib_dir}"/libQt6*.so* "${stage}/opt/SAPIE/lib/"
cp -a "${qt_plugin_dir}/." "${stage}/opt/SAPIE/plugins/"
cp -a "${qt_qml_dir}/." "${stage}/opt/SAPIE/qml/"
if [[ ! -d "${hicolor_dir}" ]]; then
  echo "error: hicolor icons not found; run the sapie-linux-icons target first" >&2
  exit 1
fi
cp -a "${hicolor_dir}/." "${stage}/usr/share/icons/"

cat > "${stage}/opt/SAPIE/bin/qt.conf" <<'EOF'
[Paths]
Prefix=/opt/SAPIE
Binaries=bin
Libraries=lib
Plugins=plugins
Qml2Imports=qml
EOF

cat > "${stage}/usr/bin/sapie-launcher" <<'EOF'
#!/bin/sh
SAPIE_LIB=/opt/SAPIE/lib
LD_LIBRARY_PATH="${SAPIE_LIB}${LD_LIBRARY_PATH:+:${LD_LIBRARY_PATH}}"
QT_PLUGIN_PATH=/opt/SAPIE/plugins
QML2_IMPORT_PATH=/opt/SAPIE/qml
export LD_LIBRARY_PATH QT_PLUGIN_PATH QML2_IMPORT_PATH
exec /opt/SAPIE/bin/sapie-launcher.bin "$@"
EOF
chmod 755 "${stage}/usr/bin/sapie-launcher"

install -m644 "${resources_dir}/org.sapie.SAPIE.desktop" \
  "${stage}/usr/share/applications/org.sapie.SAPIE.desktop"
install -m644 "${resources_dir}/org.sapie.SAPIE.metainfo.xml" \
  "${stage}/usr/share/metainfo/org.sapie.SAPIE.metainfo.xml"

cat > "${stage}/DEBIAN/control" <<EOF
Package: sapie
Version: ${version}
Section: games
Priority: optional
Architecture: ${deb_arch}
Maintainer: SAPIE maintainers <sapie@localhost>
Depends: libc6, libstdc++6, libdbus-1-3, libxcb1, libxkbcommon0
Description: SAPIE for Linux
 Old Steam games, no Steam client.
 .
 Non-commercial. Built for games you already own.
EOF
chmod 644 "${stage}/DEBIAN/control"

cat > "${stage}/DEBIAN/postinst" <<'EOF'
#!/bin/sh
set -e
if command -v update-desktop-database >/dev/null 2>&1; then
  update-desktop-database /usr/share/applications || true
fi
EOF
chmod 755 "${stage}/DEBIAN/postinst"

deb="${output_dir}/SAPIE-${version}-linux-${deb_arch}.deb"
dpkg-deb --build --root-owner-group "${stage}" "${deb}"
echo "wrote ${deb}"

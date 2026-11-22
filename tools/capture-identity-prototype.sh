#!/usr/bin/env bash
# Capture real offscreen PNGs of the M22-e QML prototype into an output directory.
# Usage: tools/capture-identity-prototype.sh [output-directory]
# Default output directory is a fresh mktemp directory. Nothing is committed by this script.
set -eu

root=$(cd "$(dirname "$0")/.." && pwd)
out=${1:-$(mktemp -d /tmp/opencode/sapie-identity-XXXXXX)}
mkdir -p "$out"

cd "$out"
QT_QPA_PLATFORM=offscreen /usr/lib/qt6/bin/qmltestrunner \
    -input "$root/launcher/ui/prototype/capture_prototype.qml"

printf 'Captures in %s\n' "$out"

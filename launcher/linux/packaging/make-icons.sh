#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 2 ]]; then
  echo "usage: $0 <repo-root> <hicolor-dir>" >&2
  exit 2
fi

repo_root="$1"
hicolor_dir="$2"
icon_dir="${repo_root}/design/icons"

if [[ ! -d "${icon_dir}" ]]; then
  echo "error: icon source directory not found: ${icon_dir}" >&2
  exit 1
fi

renderer=""
renderer_kind=""
if command -v rsvg-convert >/dev/null 2>&1; then
  renderer="rsvg-convert"
  renderer_kind="rsvg"
elif command -v magick >/dev/null 2>&1; then
  renderer="magick"
  renderer_kind="magick"
elif command -v convert >/dev/null 2>&1; then
  renderer="convert"
  renderer_kind="magick"
elif command -v inkscape >/dev/null 2>&1; then
  renderer="inkscape"
  renderer_kind="inkscape"
else
  echo "error: no SVG renderer found (rsvg-convert, magick or inkscape)" >&2
  exit 1
fi

for size in 16 22 32 48 64 128 256 512; do
  source_svg="${icon_dir}/sapie-mark-${size}.svg"
  output_png="${hicolor_dir}/${size}x${size}/apps/org.sapie.SAPIE.png"
  if [[ ! -f "${source_svg}" ]]; then
    echo "error: missing provisional icon source: ${source_svg}" >&2
    exit 1
  fi
  mkdir -p "$(dirname "${output_png}")"
  case "${renderer_kind}" in
    rsvg) "${renderer}" -w "${size}" -h "${size}" -o "${output_png}" "${source_svg}" ;;
    magick) "${renderer}" -background none -resize "${size}x${size}" "${source_svg}" "${output_png}" ;;
    inkscape) "${renderer}" --export-type=png --export-filename="${output_png}" \
      --export-width="${size}" --export-height="${size}" "${source_svg}" ;;
  esac
done

install -Dm644 "${icon_dir}/sapie-mark-512.svg" \
  "${hicolor_dir}/scalable/apps/org.sapie.SAPIE.svg"
echo "wrote hicolor icons to ${hicolor_dir} from provisional G1 art"

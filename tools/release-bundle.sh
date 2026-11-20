#!/usr/bin/env bash
# Assemble full runtime bundles for a release tag from CI artifacts.
set -euo pipefail

readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

if [[ $# -ne 3 ]]; then
  echo "usage: $0 <tag> <artifacts-dir> <output-dir>" >&2
  exit 2
fi

tag="$1"
artifacts="$2"
output="$3"

require_dir() {
  [[ -d "$1" ]] || { echo "error: missing artifact directory: $1" >&2; exit 1; }
}

require_file() {
  [[ -f "$1" ]] || { echo "error: missing file: $1" >&2; exit 1; }
}

# download-artifact@v4 extracts a single-file artifact into a same-named directory.
server_path() {
  local path="$artifacts/$1"
  if [[ -d "$path" ]]; then
    path="$path/$1"
  fi
  require_file "$path"
  printf '%s\n' "$path"
}

for target in windows-x86 windows-x64 linux-x86 linux-x64 linux-arm64; do
  for era in legacy modern; do
    require_dir "$artifacts/client-$target-$era"
  done
done

launcher_win="$artifacts/launcher-x86_64-pc-windows-msvc/sapie-launcher.exe"
launcher_x64="$artifacts/launcher-x86_64-unknown-linux-gnu/sapie-launcher"
launcher_arm64="$artifacts/launcher-aarch64-unknown-linux-gnu/sapie-launcher"
require_file "$launcher_win"
require_file "$launcher_x64"
require_file "$launcher_arm64"

server_win="$(server_path server-windows-amd64)"
server_x64="$(server_path server-linux-amd64)"
server_arm64="$(server_path server-linux-arm64)"

for doc in docs/QUICKSTART.md docs/samples/sapie_server.txt docs/samples/steam_appid.txt; do
  require_file "$REPO_ROOT/$doc"
done
steamless="$REPO_ROOT/client/src-tauri/resources/steamless"
require_dir "$steamless"

mkdir -p "$output"
output="$(cd "$output" && pwd)"
stage="$output/.stage"

bundle() {
  local archive="$1" launcher="$2" server="$3" windows="$4"
  local launcher_name server_name dest

  rm -rf "$stage"
  mkdir -p "$stage/steam_api" "$stage/samples"
  if [[ "$windows" == yes ]]; then
    launcher_name=sapie.exe
    server_name=sapie-server.exe
  else
    launcher_name=sapie
    server_name=sapie-server
  fi

  cp "$launcher" "$stage/$launcher_name"
  cp "$server" "$stage/$server_name"
  if [[ "$windows" == no ]]; then
    chmod +x "$stage/$launcher_name" "$stage/$server_name"
  fi

  for target in windows-x86 windows-x64 linux-x86 linux-x64 linux-arm64; do
    for era in legacy modern; do
      dest="$stage/steam_api/$target-$era"
      mkdir -p "$dest"
      cp -R "$artifacts/client-$target-$era/." "$dest/"
    done
  done

  cp "$REPO_ROOT/docs/QUICKSTART.md" "$stage/QUICKSTART.md"
  cp "$REPO_ROOT/docs/samples/sapie_server.txt" "$REPO_ROOT/docs/samples/steam_appid.txt" "$stage/samples/"
  if [[ "$windows" == yes ]]; then
    cp -R "$steamless" "$stage/steamless"
    (cd "$stage" && zip -r "$archive" "$launcher_name" "$server_name" steam_api QUICKSTART.md samples steamless)
  else
    tar -czf "$archive" -C "$stage" "$launcher_name" "$server_name" steam_api QUICKSTART.md samples
  fi
}

bundle "$output/SAPIE-$tag-windows-x64.zip" "$launcher_win" "$server_win" yes
bundle "$output/SAPIE-$tag-linux-x64.tar.gz" "$launcher_x64" "$server_x64" no
bundle "$output/SAPIE-$tag-linux-arm64.tar.gz" "$launcher_arm64" "$server_arm64" no

rm -rf "$stage"

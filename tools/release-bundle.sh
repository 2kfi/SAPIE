#!/usr/bin/env bash
# Assemble full 0.5 runtime bundles for a release tag from CI artifacts.
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

require_nonempty_dir() {
  require_dir "$1"
  [[ -n "$(ls -A "$1")" ]] || { echo "error: empty artifact directory: $1" >&2; exit 1; }
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
    require_nonempty_dir "$artifacts/client-$target-$era"
  done
done

require_nonempty_dir "$artifacts/launcher-windows-x64"
require_nonempty_dir "$artifacts/launcher-linux-x64"
require_nonempty_dir "$artifacts/launcher-linux-arm64"

server_win="$(server_path server-windows-amd64)"
server_win_arm="$(server_path server-windows-arm64)"
server_x64="$(server_path server-linux-amd64)"
server_arm64="$(server_path server-linux-arm64)"

web_dist="$artifacts/web-dist"
require_nonempty_dir "$web_dist"

for doc in docs/QUICKSTART.md docs/THIRD_PARTY_NOTICES.md \
  docs/samples/sapie_server.txt docs/samples/steam_appid.txt Dockerfile compose.yaml; do
  require_file "$REPO_ROOT/$doc"
done
steamless="$REPO_ROOT/client/src-tauri/resources/steamless"
require_nonempty_dir "$steamless"
require_file "$SCRIPT_DIR/update-signing-public.pem"

mkdir -p "$output"
output="$(cd "$output" && pwd)"
stage="$output/.stage"

bundle() {
  local archive="$1" launcher_dir="$2" server="$3" windows="$4"
  local server_name dest

  rm -rf "$stage"
  mkdir -p "$stage/launcher" "$stage/steam_api" "$stage/samples" "$stage/web/dist"
  if [[ "$windows" == yes ]]; then
    server_name=sapie-server.exe
  else
    server_name=sapie-server
  fi

  cp -R "$launcher_dir/." "$stage/launcher/"
  cp "$server" "$stage/$server_name"
  if [[ "$windows" == yes ]]; then
    cp "$server_win_arm" "$stage/sapie-server-arm64.exe"
  else
    chmod +x "$stage/$server_name"
  fi

  for target in windows-x86 windows-x64 linux-x86 linux-x64 linux-arm64; do
    for era in legacy modern; do
      dest="$stage/steam_api/$target-$era"
      mkdir -p "$dest"
      cp -R "$artifacts/client-$target-$era/." "$dest/"
    done
  done

  cp "$REPO_ROOT/docs/QUICKSTART.md" "$stage/QUICKSTART.md"
  cp "$REPO_ROOT/docs/THIRD_PARTY_NOTICES.md" "$stage/THIRD_PARTY_NOTICES.md"
  cp "$REPO_ROOT/docs/samples/sapie_server.txt" "$REPO_ROOT/docs/samples/steam_appid.txt" "$stage/samples/"
  cp -R "$web_dist/." "$stage/web/dist/"
  if [[ "$windows" == yes ]]; then
    cp -R "$steamless" "$stage/steamless"
    python3 - "$archive" "$stage" <<'PY'
import os
import sys
import zipfile

archive, root = sys.argv[1], sys.argv[2]
with zipfile.ZipFile(archive, "w", zipfile.ZIP_DEFLATED) as bundle:
    for directory, _, files in os.walk(root):
        for name in sorted(files):
            path = os.path.join(directory, name)
            bundle.write(path, os.path.relpath(path, root))
PY
  else
    tar -czf "$archive" -C "$stage" launcher "$server_name" steam_api QUICKSTART.md THIRD_PARTY_NOTICES.md samples web
  fi
}

bundle "$output/SAPIE-$tag-windows-x64.zip" "$artifacts/launcher-windows-x64" "$server_win" yes
bundle "$output/SAPIE-$tag-linux-x64.tar.gz" "$artifacts/launcher-linux-x64" "$server_x64" no
bundle "$output/SAPIE-$tag-linux-arm64.tar.gz" "$artifacts/launcher-linux-arm64" "$server_arm64" no

python3 "$SCRIPT_DIR/gen-sbom.py" --tag "$tag" --output-dir "$output" --repo "$REPO_ROOT"
container="$output/container"
rm -rf "$container"
mkdir -p "$container/server" "$container/web/dist"
cp "$REPO_ROOT/Dockerfile" "$REPO_ROOT/compose.yaml" "$container/"
cp "$REPO_ROOT/docs/THIRD_PARTY_NOTICES.md" "$container/"
cp "$server_x64" "$container/server/sapie-server-linux-amd64"
cp "$server_arm64" "$container/server/sapie-server-linux-arm64"
cp "$server_win" "$container/server/sapie-server-windows-amd64.exe"
cp "$server_win_arm" "$container/server/sapie-server-windows-arm64.exe"
cp -R "$web_dist/." "$container/web/dist/"
tar -czf "$output/container.tar.gz" -C "$output" container
rm -rf "$container"

cp "$SCRIPT_DIR/update-signing-public.pem" "$output/update-signing-public.pem"

(
  cd "$output"
  find . -type f ! -path './.stage/*' ! -name SHA256SUMS -printf '%P\n' | LC_ALL=C sort |
    while IFS= read -r file; do
      sha256sum "$file"
    done > SHA256SUMS
)

if [[ -n "${SAPIE_UPDATE_SIGNING_KEY:-}" ]]; then
  for channel in stable beta; do
    python3 "$SCRIPT_DIR/update-manifest.py" generate \
      --tag "$tag" --artifacts "$output" --output "$output/update-$channel.json" \
      --public-key "$output/update-signing-public.pem" --channel "$channel" \
      --previous-version "${SAPIE_PREVIOUS_VERSION:-}" --checksums "$output/SHA256SUMS"
    python3 "$SCRIPT_DIR/update-manifest.py" verify \
      --manifest "$output/update-$channel.json" --public-key "$output/update-signing-public.pem" \
      --channel "$channel"
  done
else
  # shortcut: no signing secret in this repository. Bundles and checksums still ship, but auto-update is disabled. Configure SAPIE_UPDATE_SIGNING_KEY to sign manifests.
  cat > "$output/UNSIGNED-UPDATES.txt" <<'EOF'
Update manifests are not signed for this release because no SAPIE_UPDATE_SIGNING_KEY is configured.
The bundles and SHA256SUMS are intact. Auto-update is disabled until a signed manifest is published.
EOF
fi

rm -rf "$stage"

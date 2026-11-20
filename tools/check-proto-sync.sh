#!/usr/bin/env bash
# Fail when the client's proto copies drift from the server's canonical protos.
set -euo pipefail

readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

status=0
for proto in "$REPO_ROOT"/proto/*.proto; do
  name="${proto##*/}"
  copy="$REPO_ROOT/client/dll/$name"
  if ! diff -u \
    <(grep -v '^[[:space:]]*option go_package' "$proto") \
    <(grep -v '^[[:space:]]*option go_package' "$copy"); then
    echo "ERROR: $name differs between proto/ and client/dll/ (go_package lines ignored)." >&2
    status=1
  fi
done
exit "$status"

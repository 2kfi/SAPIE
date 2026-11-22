#!/usr/bin/env bash
# Self-check for update manifest signing, verification and tamper rejection.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

openssl genpkey -algorithm ED25519 -out "$tmp/private.pem" 2>/dev/null
openssl pkey -in "$tmp/private.pem" -pubout -out "$tmp/public.pem" 2>/dev/null
mkdir -p "$tmp/artifacts"
printf 'archive' > "$tmp/artifacts/sapie-launcher-0.5.0-linux-x64.tar.gz"
printf 'checksums' > "$tmp/SHA256SUMS"

python3 "$script_dir/update-manifest.py" generate \
  --tag 0.5.0 --artifacts "$tmp/artifacts" --output "$tmp/update-stable.json" \
  --public-key "$tmp/public.pem" --private-key "$tmp/private.pem" \
  --channel stable --previous-version 0.4 --checksums "$tmp/SHA256SUMS" >/dev/null
python3 "$script_dir/update-manifest.py" verify \
  --manifest "$tmp/update-stable.json" --public-key "$tmp/public.pem" \
  --channel stable --version 0.5.0 >/dev/null

cp "$tmp/update-stable.json" "$tmp/tampered.json"
cp "$tmp/update-stable.json.sig" "$tmp/tampered.json.sig"
sed -i 's/"channel":"stable"/"channel":"beta"/' "$tmp/tampered.json"
if python3 "$script_dir/update-manifest.py" verify \
  --manifest "$tmp/tampered.json" --public-key "$tmp/public.pem" >/dev/null 2>&1; then
  echo "error: tampered manifest was accepted" >&2
  exit 1
fi
echo "update manifest self-check: pass"

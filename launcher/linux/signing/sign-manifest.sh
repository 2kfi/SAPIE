#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "${script_dir}/../../.." && pwd)"
cd "${repo_root}"

if [[ $# -lt 3 ]]; then
  echo "usage: $0 <artifacts-dir> <output-manifest> <public-key> [stable|beta] [version] [previous-version]" >&2
  exit 2
fi

artifacts="$1"
output="$2"
public_key="$3"
channel="${4:-stable}"
version="${5:-}"
previous="${6:-}"

if [[ -z "${SAPIE_UPDATE_SIGNING_KEY:-}" ]]; then
  echo "error: SAPIE_UPDATE_SIGNING_KEY is not set; no private key is committed" >&2
  exit 1
fi
if [[ -f .venv/bin/activate ]]; then
  # shellcheck disable=SC1091
  . .venv/bin/activate
fi

python_bin="${SAPIE_PYTHON:-python3}"
args=(generate --tag "${version}" --artifacts "${artifacts}" --output "${output}"
      --public-key "${public_key}" --channel "${channel}")
if [[ -n "${previous}" ]]; then
  args+=(--previous-version "${previous}")
fi

"${python_bin}" tools/update-manifest.py "${args[@]}"
"${script_dir}/../scripts/check-update-manifest.sh" "${output}" "${public_key}" \
  "${channel}" "${version}"

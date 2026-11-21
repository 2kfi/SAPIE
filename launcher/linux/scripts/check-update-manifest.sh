#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "${script_dir}/../../.." && pwd)"
cd "${repo_root}"

if [[ $# -lt 2 ]]; then
  echo "usage: $0 <manifest> <public-key> [stable|beta] [version]" >&2
  exit 2
fi

manifest="$1"
public_key="$2"
channel="${3:-stable}"
version="${4:-}"

if [[ -f .venv/bin/activate ]]; then
  # shellcheck disable=SC1091
  . .venv/bin/activate
fi

python_bin="${SAPIE_PYTHON:-python3}"
args=(verify --manifest "${manifest}" --public-key "${public_key}" --channel "${channel}")
if [[ -n "${version}" ]]; then
  args+=(--version "${version}")
fi

exec "${python_bin}" tools/update-manifest.py "${args[@]}"

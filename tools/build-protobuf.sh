#!/usr/bin/env bash
# Build a target-specific static protobuf-lite for the SAPIE client matrix.
set -euo pipefail

readonly PROTOBUF_TAG=v36.1
readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

target="${1:-}"
case "$target" in
  windows-x86|windows-x64|linux-x86|linux-arm64) ;;
  linux-x64)
    echo "linux-x64 is the host target: using system /usr/lib/libprotobuf-lite.so"
    exit 0
    ;;
  *)
    echo "usage: $0 <windows-x86|windows-x64|linux-x86|linux-x64|linux-arm64>" >&2
    exit 2
    ;;
esac

toolchain="$REPO_ROOT/client/cmake/toolchains/$target.cmake"
if [[ ! -f "$toolchain" ]]; then
  echo "error: missing toolchain file $toolchain" >&2
  exit 1
fi
prefix="$REPO_ROOT/third_party/protobuf/$target"
tmp="$(mktemp -d /tmp/sapie-protobuf.XXXXXX)"
trap 'rm -rf "$tmp"' EXIT

echo "Building protobuf-lite ($PROTOBUF_TAG) for $target into $prefix"
git clone --depth 1 --branch "$PROTOBUF_TAG" https://github.com/protocolbuffers/protobuf.git "$tmp/src" \
  || { echo "error: git clone of protobuf $PROTOBUF_TAG failed" >&2; exit 1; }

cmake -S "$tmp/src" -B "$tmp/build" -G Ninja \
  -DCMAKE_TOOLCHAIN_FILE="$toolchain" \
  -DCMAKE_INSTALL_PREFIX="$prefix" \
  -DCMAKE_BUILD_TYPE=Release \
  -DCMAKE_CXX_STANDARD=17 \
  -DCMAKE_INSTALL_LIBDIR=lib \
  -DBUILD_SHARED_LIBS=OFF \
  -Dprotobuf_BUILD_SHARED_LIBS=OFF \
  -Dprotobuf_BUILD_PROTOBUF_BINARIES=ON \
  -Dprotobuf_BUILD_PROTOC_BINARIES=OFF \
  -Dprotobuf_BUILD_LIBPROTOBUF=OFF \
  -Dprotobuf_BUILD_LIBPROTOC=OFF \
  -Dprotobuf_BUILD_LIBUPB=OFF \
  -Dprotobuf_BUILD_TESTS=OFF \
  -Dprotobuf_BUILD_CONFORMANCE=OFF \
  -Dprotobuf_BUILD_EXAMPLES=OFF \
  -Dprotobuf_WITH_ZLIB=OFF \
  -Dprotobuf_FORCE_FETCH_DEPENDENCIES=ON \
  -Dprotobuf_INSTALL=ON \
  || { echo "error: protobuf configure for $target failed" >&2; exit 1; }

cmake --build "$tmp/build" --target libprotobuf-lite --parallel \
  || { echo "error: protobuf-lite build for $target failed" >&2; exit 1; }
cmake --install "$tmp/build" \
  || { echo "error: protobuf-lite install for $target failed" >&2; exit 1; }

echo "Done: $prefix/lib/libprotobuf-lite.a and $prefix/include"

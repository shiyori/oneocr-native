#!/usr/bin/env bash
set -euo pipefail
script_dir="$(cd "$(dirname "$0")" && pwd)"
platform="$(go env GOOS)-$(go env GOARCH)"
output="${ONEOCR_NATIVE_OUTPUT:-$script_dir/../dist/native/$platform}"
mkdir -p "$output"
output="$(cd "$output" && pwd)"
cd "$script_dir/.."
case "$(go env GOOS)" in
  darwin)
    CGO_ENABLED=1 go build -trimpath -buildmode=c-shared \
      -ldflags='-extldflags=-Wl,-install_name,@rpath/liboneocr.dylib,-rpath,@loader_path' \
      -o "$output/liboneocr.dylib" ./cmd/oneocr-shared ;;
  linux)
    CGO_ENABLED=1 go build -trimpath -buildmode=c-shared -ldflags='-extldflags=-Wl,-soname,liboneocr.so,-rpath,$ORIGIN' \
      -o "$output/liboneocr.so" ./cmd/oneocr-shared ;;
  windows)
    CGO_ENABLED=1 go build -trimpath -buildmode=c-shared -o "$output/oneocr.dll" ./cmd/oneocr-shared ;;
  *) echo 'Use android/build-native.sh for Android' >&2; exit 2 ;;
esac
echo "Shared library: $output"

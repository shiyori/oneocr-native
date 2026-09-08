#!/usr/bin/env bash
set -euo pipefail
# Requires Go >=1.24, NDK >=27, CMake >=3.22. No Gradle or Python required.
script_dir="$(cd "$(dirname "$0")" && pwd)"
: "${ANDROID_NDK_HOME:?Set ANDROID_NDK_HOME to an installed Android NDK}"
abi="${1:-arm64-v8a}"
api="${ANDROID_API:-26}"
case "$abi" in
  arm64-v8a) go_arch=arm64; triple=aarch64-linux-android ;;
  x86_64) go_arch=amd64; triple=x86_64-linux-android ;;
  *) echo 'Supported ABIs: arm64-v8a, x86_64' >&2; exit 2 ;;
esac
case "$(uname -s)" in
  Darwin) host=darwin-x86_64 ;;
  Linux) host=linux-x86_64 ;;
  *) echo 'Use WSL or cross-build with the NDK manually on Windows' >&2; exit 2 ;;
esac
toolchain="$ANDROID_NDK_HOME/toolchains/llvm/prebuilt/$host"
output="${ONEOCR_ANDROID_OUTPUT:-$script_dir/../../dist/native/android}"
mkdir -p "$output/$abi"
output="$(cd "$output" && pwd)"
(
  cd "$script_dir/../.."
  CGO_ENABLED=1 GOOS=android GOARCH="$go_arch" \
    CC="$toolchain/bin/$triple$api-clang" \
    CXX="$toolchain/bin/$triple$api-clang++" \
    go build -trimpath -buildmode=c-shared -ldflags='-extldflags=-Wl,-soname,liboneocr.so,-z,max-page-size=16384,-z,common-page-size=16384' \
      -o "$output/$abi/liboneocr.so" ./cmd/oneocr-shared
)
cmake -S "$script_dir" -B "$output/cmake-$abi" \
  -DCMAKE_TOOLCHAIN_FILE="$ANDROID_NDK_HOME/build/cmake/android.toolchain.cmake" \
  -DANDROID_ABI="$abi" -DANDROID_PLATFORM="android-$api" \
  -DANDROID_STL=c++_static -DONEOCR_GO_LIBRARY="$output/$abi/liboneocr.so" \
  -DCMAKE_BUILD_TYPE=Release
cmake --build "$output/cmake-$abi" --parallel 2
cp "$output/cmake-$abi/liboneocr_jni.so" "$output/$abi/"
if ! "$toolchain/bin/llvm-readelf" -d "$output/$abi/liboneocr_jni.so" | grep -q 'NEEDED.*\[liboneocr.so\]'; then
  echo 'JNI must depend on portable liboneocr.so, not a host build path' >&2
  exit 1
fi
echo "Native libraries: $output/$abi (add target ONNX Runtime separately)"

# Download and install

[简体中文](../zh-CN/installation.md) | [English](../en/installation.md) | [日本語](../ja/installation.md)

Go and Python use their language package managers; Android uses AAR downloads. No Windows/macOS platform-specific artifacts are published. Linux prebuilt packages are optional. Normal integration needs no runtime path.

[GitHub Releases v0.1.0](https://github.com/shiyori/oneocr-native/releases/tag/v0.1.0)

## Go

For code integration, run `go get github.com/shiyori/oneocr-native@v0.1.0` in your project, prepare resources with `oneocr.Install`, then call `oneocr.Open`. See [Go integration](go.md). For the CLI:

```sh
go install github.com/shiyori/oneocr-native/cmd/oneocr@v0.1.0
oneocr install
oneocr recognize image.png
```

## Python

The universal wheel supports Python 3.11–3.13 and installs from any directory:

```sh
python -m pip install "https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr_native-0.1.0-py3-none-any.whl"
python -m oneocr_native install
```

## Android

[AAR](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-android-0.1.0.aar) · [Core AAR](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-android-core-0.1.0.aar)

The complete AAR includes the default model and runtime. Apps with an existing host ORT use the Core AAR. Both contain arm64-v8a / x86_64 and require Android API 26+. See [Android integration](android.md).

## Optional Linux downloads

| | SDK | Core |
|---|---|---|
| Linux x64 | [SDK](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-sdk-linux-amd64-0.1.0.zip) | [Core](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-core-linux-amd64-0.1.0.zip) |
| Linux ARM64 | [SDK](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-sdk-linux-arm64-0.1.0.zip) | [Core](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-core-linux-arm64-0.1.0.zip) |

The complete package contains the CLI, C/C++ headers, shared library, default model and ORT. Core keeps the interfaces and CLI without the model or ORT. Linux builds use Ubuntu 22.04. Go integration does not require these packages.

Run the complete package after extraction; prepare the Core package first:

```sh
/path/to/sdk/bin/oneocr recognize image.png
/path/to/core-sdk/bin/oneocr install
/path/to/core-sdk/bin/oneocr recognize image.png
```

## Dependencies and offline preparation

The installer reuses a compatible existing runtime. Otherwise Windows/macOS download a pinned official ONNX Runtime release and verify SHA-256; Linux can use this project’s runtime archive. Python uses pip to install missing ORT from upstream and preserves compatible CPU/GPU distributions. Recognition never downloads files.

For offline preparation, place this release’s `release-manifest.json`, `SHA256SUMS` and default model in one directory. Also save the official `onnxruntime-win-x64-1.29.0.zip` / `onnxruntime-osx-arm64-1.29.0.tgz` archive for Windows/macOS, or this project’s runtime ZIP for Linux. The installer extracts it; no runtime path is needed:

```sh
oneocr install --source /path/to/resources --offline
```

Windows must satisfy [ONNX Runtime’s Visual C++ runtime prerequisite](https://onnxruntime.ai/docs/install/#requirements). Go also requires a C compiler. Installation configuration changes only after model sessions validate.

## Language guides

[Go](go.md) · [C/C++](native.md) · [Python](python.md) · [Android](android.md)

---

[oneocr-native](../../README.en.md) · [AGPL-3.0-only](../../LICENSE)

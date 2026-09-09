# Download and install

[简体中文](../zh-CN/installation.md) | [English](../en/installation.md) | [日本語](../ja/installation.md)

Download from [GitHub Releases v0.1.0-rc.1](https://github.com/shiyori/oneocr-native/releases/tag/v0.1.0-rc.1). No source checkout is needed. Normal installation and recognition require no runtime path.

| Platform | Complete, offline | Core |
|---|---|---|
| Windows x64 | [SDK](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0-rc.1/oneocr-sdk-windows-amd64-0.1.0-rc.1.zip) | [Core](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0-rc.1/oneocr-core-windows-amd64-0.1.0-rc.1.zip) |
| macOS ARM64 | [SDK](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0-rc.1/oneocr-sdk-darwin-arm64-0.1.0-rc.1.zip) | [Core](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0-rc.1/oneocr-core-darwin-arm64-0.1.0-rc.1.zip) |
| Linux x64 | [SDK](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0-rc.1/oneocr-sdk-linux-amd64-0.1.0-rc.1.zip) | [Core](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0-rc.1/oneocr-core-linux-amd64-0.1.0-rc.1.zip) |
| Linux ARM64 | [SDK](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0-rc.1/oneocr-sdk-linux-arm64-0.1.0-rc.1.zip) | [Core](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0-rc.1/oneocr-core-linux-arm64-0.1.0-rc.1.zip) |
| Android arm64-v8a / x86_64 | [AAR](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0-rc.1/oneocr-android-0.1.0-rc.1.aar) | [Core AAR](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0-rc.1/oneocr-android-core-0.1.0-rc.1.aar) |


The complete desktop SDK includes the CLI, model, ONNX Runtime, C/C++ headers, Go sources, offline Go dependencies and these guides. The core SDK includes the same integration tools without the model or ORT. Desktop targets are Windows x64, macOS ARM64 and Linux x64/ARM64; Linux builds use an Ubuntu 22.04 baseline. Android requires API 26 or later.

## Complete desktop SDK

Extract the ZIP anywhere. Run its CLI directly:

```sh
/path/to/sdk/bin/oneocr recognize image.png
```

On Windows use `C:\path\to\sdk\bin\oneocr.exe`. Images can be anywhere; pass their paths. To make the resources available to your own Go program and future CLI calls, run:

```sh
/path/to/sdk/bin/oneocr install --offline
```

Installation records the resources in your user configuration directory. Keep using the CLI by its path, or add the SDK's `bin` directory to your `PATH`.

## Core SDK

```sh
/path/to/core-sdk/bin/oneocr install
/path/to/core-sdk/bin/oneocr recognize image.png
```

The installer reuses a compatible local runtime, otherwise downloads the pinned model and runtime from the same GitHub Release. Repeating the command reuses verified resources. Recognition never accesses the network.

For offline core installation, download `release-manifest.json`, `SHA256SUMS`, the model asset and your platform's runtime ZIP into one directory, then run:

```sh
/path/to/core-sdk/bin/oneocr install --source /path/to/release-files --offline
```

Assets are checked for the correct version, platform and SHA-256 before installation. Configuration is updated only after model sessions can be created. Existing host runtimes are preserved.

## Language-specific setup

[Go](go.md) · [C/C++](native.md) · [Python](python.md) · [Android](android.md) · [Existing runtime](runtime.md)

---

[oneocr-native](../../README.en.md) · [AGPL-3.0-only](../../LICENSE)

# Downloads and installation

[简体中文](../zh-CN/installation.md) | [English](../en/installation.md) | [日本語](../ja/installation.md)

Complete integration is recommended: the full Android AAR and Linux packages include the default model and ONNX Runtime. Code integrations use the Go/Python installer to prepare complete dependencies, obtaining resources from Releases, source repositories or official dependency repositories.

[GitHub Releases · Latest](https://github.com/shiyori/oneocr-native/releases/latest)

## Complete packages (recommended)

| | Complete package |
|---|---|
| Android arm64-v8a / x86_64 | [AAR](https://github.com/shiyori/oneocr-native/releases/latest/download/oneocr-android.aar) |
| Linux x64 | [Complete package](https://github.com/shiyori/oneocr-native/releases/latest/download/oneocr-linux-amd64.zip) |
| Linux ARM64 | [Complete package](https://github.com/shiyori/oneocr-native/releases/latest/download/oneocr-linux-arm64.zip) |

Extract the Linux package and run it directly. Go installation and model/runtime paths are not required. It contains `bin/oneocr`, `models/`, `lib/`, licenses and usage documentation, targeting Ubuntu 22.04 and compatible systems.

```sh
/path/to/oneocr-linux-amd64/bin/oneocr recognize image.png
/path/to/oneocr-linux-amd64/bin/oneocr recognize --format json image.png
```

The complete Android AAR includes arm64-v8a / x86_64 and requires API 26+. Add it to the application and follow the [Android guide](android.md). Core is an advanced option only for applications already managing a host ORT.

## Code integration

### Go

Run `go get github.com/shiyori/oneocr-native@v0.1.3` in your project, use `oneocr.Install` to prepare complete dependencies, then call `oneocr.Open`. For the command line:

```sh
go install github.com/shiyori/oneocr-native/cmd/oneocr@v0.1.3
oneocr install
oneocr recognize image.png
oneocr recognize --format json image.png
```

### Python

Python 3.11–3.13 uses the universal wheel. The preparation command supplies the model and any missing runtime:

```sh
python -m pip install "https://github.com/shiyori/oneocr-native/releases/download/v0.1.3/oneocr_native-0.1.3-py3-none-any.whl"
python -m oneocr_native install
python -m oneocr_native recognize image.png
python -m oneocr_native recognize --format json image.png
```

[Go](go.md) · [C/C++](native.md) · [Python](python.md) · [Android](android.md)

## Advanced: dependencies and offline preparation

Installers reuse compatible existing runtimes first. Desktop Go installations download pinned archives from official ONNX Runtime Releases and verify SHA-256; Android preparation downloads the official Maven AAR; Python obtains dependencies through pip. Recognition never downloads files. New OneOCR Releases do not publish separate runtime ZIPs.

For offline code integration, place `release-manifest.json`, `SHA256SUMS`, the default model and the matching unmodified official ORT archive in one directory. Linux uses `onnxruntime-linux-x64-1.29.0.tgz` or `onnxruntime-linux-aarch64-1.29.0.tgz`; Windows/macOS use their official ZIP/TGZ; Android uses `onnxruntime-android-1.29.0.aar`. Then run:

```sh
oneocr install --source /path/to/resources --offline
```

Go code integration requires Go 1.24+ and a C compiler. Windows also needs the [ORT Visual C++ runtime prerequisites](https://onnxruntime.ai/docs/install/#requirements). The complete Linux package runs directly with its bundled command.

---

[oneocr-native](../../README.en.md) · [AGPL-3.0-only](../../LICENSE)

# oneocr-native

[简体中文](README.md) | [English](README.en.md) | [日本語](README.ja.md)

Offline Chinese, Japanese, Korean and English OCR with Go, C/C++, Python and Android interfaces.

## Downloads

[GitHub Releases · Latest](https://github.com/shiyori/oneocr-native/releases/latest)

| Platform | Complete package (recommended) |
|---|---|
| Android arm64-v8a / x86_64 | [Complete AAR](https://github.com/shiyori/oneocr-native/releases/latest/download/oneocr-android.aar) |
| Linux x64 | [Complete runtime package](https://github.com/shiyori/oneocr-native/releases/latest/download/oneocr-linux-amd64.zip) |
| Linux ARM64 | [Complete runtime package](https://github.com/shiyori/oneocr-native/releases/latest/download/oneocr-linux-arm64.zip) |

Use complete packages with the default model and runtime included. Core AAR is an advanced option for an existing host ORT; see the [Android guide](docs/en/android.md). Windows/macOS use Go or Python to prepare complete dependencies; no platform-specific packages are published.

## Go integration

For code integration, use `go get github.com/shiyori/oneocr-native@v0.1.1`; see the [Go guide](docs/en/go.md). Install the CLI with:

```sh
go install github.com/shiyori/oneocr-native/cmd/oneocr@v0.1.1
oneocr install
oneocr recognize image.png
```

## Python

```sh
python -m pip install "https://github.com/shiyori/oneocr-native/releases/download/v0.1.1/oneocr_native-0.1.1-py3-none-any.whl"
python -m oneocr_native install
python -m oneocr_native recognize image.png
```

## Integration guides

[Installation](docs/en/installation.md) · [Go](docs/en/go.md) · [C and C++](docs/en/native.md) · [Python](docs/en/python.md) · [Android](docs/en/android.md) · [Existing ONNX Runtime](docs/en/runtime.md)

---

[AGPL-3.0-only](LICENSE).

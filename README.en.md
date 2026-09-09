# oneocr-native

[简体中文](README.md) | [English](README.en.md) | [日本語](README.ja.md)

Offline Chinese, Japanese, Korean and English OCR with Go, C/C++, Python and Android interfaces.

## Downloads

[GitHub Releases v0.1.0](https://github.com/shiyori/oneocr-native/releases/tag/v0.1.0)

| | Complete | Core |
|---|---|---|
| Android arm64-v8a / x86_64 | [AAR](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-android-0.1.0.aar) | [Core AAR](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-android-core-0.1.0.aar) |
| Linux x64 | [SDK](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-sdk-linux-amd64-0.1.0.zip) | [Core](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-core-linux-amd64-0.1.0.zip) |
| Linux ARM64 | [SDK](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-sdk-linux-arm64-0.1.0.zip) | [Core](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-core-linux-arm64-0.1.0.zip) |

On Windows/macOS, use the Go module, Go command or universal Python wheel; dependencies are prepared on demand. No platform-specific packages are published. Linux downloads are optional.

## Go integration

For code integration, use `go get github.com/shiyori/oneocr-native@v0.1.0`; see the [Go guide](docs/en/go.md). Install the CLI with:

```sh
go install github.com/shiyori/oneocr-native/cmd/oneocr@v0.1.0
oneocr install
oneocr recognize image.png
```

## Python

```sh
python -m pip install "https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr_native-0.1.0-py3-none-any.whl"
python -m oneocr_native install
python -m oneocr_native recognize image.png
```

## Integration guides

[Installation](docs/en/installation.md) · [Go](docs/en/go.md) · [C and C++](docs/en/native.md) · [Python](docs/en/python.md) · [Android](docs/en/android.md) · [Existing ONNX Runtime](docs/en/runtime.md)

---

[AGPL-3.0-only](LICENSE).

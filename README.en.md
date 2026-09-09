# oneocr-native

[简体中文](README.md) | [English](README.en.md) | [日本語](README.ja.md)

Offline Chinese, Japanese, Korean and English OCR for Go, C/C++, Python and Android.

## Download

[GitHub Releases v0.1.0](https://github.com/shiyori/oneocr-native/releases/tag/v0.1.0)

| Platform | Complete, offline | Core |
|---|---|---|
| Windows x64 | [SDK](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-sdk-windows-amd64-0.1.0.zip) | [Core](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-core-windows-amd64-0.1.0.zip) |
| macOS ARM64 | [SDK](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-sdk-darwin-arm64-0.1.0.zip) | [Core](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-core-darwin-arm64-0.1.0.zip) |
| Linux x64 | [SDK](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-sdk-linux-amd64-0.1.0.zip) | [Core](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-core-linux-amd64-0.1.0.zip) |
| Linux ARM64 | [SDK](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-sdk-linux-arm64-0.1.0.zip) | [Core](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-core-linux-arm64-0.1.0.zip) |
| Android arm64-v8a / x86_64 | [AAR](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-android-0.1.0.aar) | [Core AAR](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-android-core-0.1.0.aar) |

## Get started

The complete SDK includes the model and runtime. After extraction, run `bin/oneocr recognize image.png`; for the core SDK, run `bin/oneocr install` first. On Windows use `bin\oneocr.exe`.

Install Python from any directory:

```sh
python -m pip install "https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr_native-0.1.0-py3-none-any.whl"
python -m oneocr_native install
python -m oneocr_native recognize image.png
```

## Integration guides

[Download and install](docs/en/installation.md) · [Go](docs/en/go.md) · [C and C++](docs/en/native.md) · [Python](docs/en/python.md) · [Android](docs/en/android.md) · [Existing ONNX Runtime](docs/en/runtime.md)

---

[AGPL-3.0-only](LICENSE). Third-party dependencies retain their own licenses.

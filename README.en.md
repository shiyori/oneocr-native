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

For code integration, use `go get github.com/shiyori/oneocr-native@v0.1.3`; see the [Go guide](docs/en/go.md). Install the CLI with:

```sh
go install github.com/shiyori/oneocr-native/cmd/oneocr@v0.1.3
oneocr install
oneocr recognize image.png
oneocr recognize --format json image.png
```

## Python

```sh
python -m pip install "https://github.com/shiyori/oneocr-native/releases/download/v0.1.3/oneocr_native-0.1.3-py3-none-any.whl"
python -m oneocr_native install
python -m oneocr_native recognize image.png
python -m oneocr_native recognize --format json image.png
```

## Test results

[Complete PaddleOCR results: paired images and JSON for all 9 originals](docs/en/test-results.md)

Actual OneOCR v0.1.3 output for four images from `testdata`. Only results with recognition confidence ≥ **0.70** and detection score ≥ **0.70** are shown. The left image adds boxes to the original; the right keeps the original image and overlays blue recognized text at the same positions, with a local light background for readability. Click either image for full resolution.

Each pair uses the same crop and coordinates. Outer whitespace is trimmed equally; recognized text is not manually corrected. Confidence scores are uncalibrated.

### CJK and English

6 / 6 nonempty recognition results shown.

| Original + boxes | Original + recognized text (blue) |
|---|---|
| [![CJK and English — Original + boxes](docs/assets/ocr-results/mixed-boxes.webp)](docs/assets/ocr-results/mixed-boxes.webp) | [![CJK and English — Original + recognized text (blue)](docs/assets/ocr-results/mixed-text.webp)](docs/assets/ocr-results/mixed-text.webp) |

### Photographed book

47 / 48 nonempty recognition results shown.

| Original + boxes | Original + recognized text (blue) |
|---|---|
| [![Photographed book — Original + boxes](docs/assets/ocr-results/book-boxes.webp)](docs/assets/ocr-results/book-boxes.webp) | [![Photographed book — Original + recognized text (blue)](docs/assets/ocr-results/book-text.webp)](docs/assets/ocr-results/book-text.webp) |

### Document with formulas

84 / 96 nonempty recognition results shown.

| Original + boxes | Original + recognized text (blue) |
|---|---|
| [![Document with formulas — Original + boxes](docs/assets/ocr-results/formula-boxes.webp)](docs/assets/ocr-results/formula-boxes.webp) | [![Document with formulas — Original + recognized text (blue)](docs/assets/ocr-results/formula-text.webp)](docs/assets/ocr-results/formula-text.webp) |

### Chinese table

96 / 96 nonempty recognition results shown.

| Original + boxes | Original + recognized text (blue) |
|---|---|
| [![Chinese table — Original + boxes](docs/assets/ocr-results/table-boxes.webp)](docs/assets/ocr-results/table-boxes.webp) | [![Chinese table — Original + recognized text (blue)](docs/assets/ocr-results/table-text.webp)](docs/assets/ocr-results/table-text.webp) |

[Generation record and reproduction](docs/assets/ocr-results/README.md) · [Sample provenance](testdata/paddleocr/README.md) · [Third-party license](testdata/paddleocr/UPSTREAM_LICENSE)

## Integration guides

[Installation](docs/en/installation.md) · [Go](docs/en/go.md) · [C and C++](docs/en/native.md) · [Python](docs/en/python.md) · [Android](docs/en/android.md) · [Existing ONNX Runtime](docs/en/runtime.md)

---

[AGPL-3.0-only](LICENSE).

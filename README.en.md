# oneocr-native

[简体中文](README.md) · [English](README.en.md) · [日本語](README.ja.md)

[![Go Reference](https://pkg.go.dev/badge/github.com/shiyori/oneocr-native.svg)](https://pkg.go.dev/github.com/shiyori/oneocr-native)
[![Go Report Card](https://goreportcard.com/badge/github.com/shiyori/oneocr-native)](https://goreportcard.com/report/github.com/shiyori/oneocr-native)
[![Stars](https://img.shields.io/github/stars/shiyori/oneocr-native)](https://github.com/shiyori/oneocr-native/stargazers)
[![Downloads](https://img.shields.io/github/downloads/shiyori/oneocr-native/total)](https://github.com/shiyori/oneocr-native/releases)
[![Release](https://img.shields.io/github/v/release/shiyori/oneocr-native)](https://github.com/shiyori/oneocr-native/releases)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Offline OneOCR with a root Go module, Android AAR, C++ SDK and independent Python SDK. One `.ocrpack` travels across platforms; inference reads resources individually without extraction. Go/C++/Android need no Python. Python runs its own complete OCR pipeline directly through ORT and does not load this project's Go library.

An unofficial experimental implementation for learning and research. This purpose statement adds no restrictions to MIT. Third-party model rights remain unchanged.

## Models

| Model | Scripts | Size |
|---|---|---:|
| [oneocr-cjk-en.ocrpack](models/oneocr-cjk-en.ocrpack) | Default: Chinese, Japanese, Korean and English (CJK + Latin) | 30.46 MiB |
| [oneocr-extended.ocrpack](models/oneocr-extended.ocrpack) | Adds Cyrillic and Arabic | 40.07 MiB |

[Model origin / rights / checksums](models/README.md). Models are separate from SDKs and excluded from the root Go module ZIP by a nested data-only module.

## Install and integrate four SDKs

Build from source using the instructions below. Local `dist/` artifacts are excluded from Git; no version tag or release has been published. After a root-module release, `go install github.com/shiyori/oneocr-native/cmd/oneocr@<tag>` becomes available. Dependency installation may need the network; inference is offline.

### Go / CLI

Go ≥1.24 + CGO + ONNX Runtime 1.29:

```bash
go install ./cmd/oneocr
oneocr install --model models/oneocr-cjk-en.ocrpack --runtime /path/to/libonnxruntime.dylib
oneocr recognize image.png
```

```go
import oneocr "github.com/shiyori/oneocr-native"

engine, err := oneocr.Open(oneocr.Config{ModelPath: "models/oneocr-cjk-en.ocrpack"})
if err != nil { return err }
defer engine.Close()
result, err := engine.RecognizeFile(ctx, "image.png", oneocr.Options{})
```

ORT: `ONEOCR_RUNTIME`, `Config.RuntimeLibrary`, SDK `lib/`. Local external module:

```bash
go mod edit -replace github.com/shiyori/oneocr-native=/path/to/oneocr-native
go get github.com/shiyori/oneocr-native
```

### Android

`dist/oneocr-android-0.1.0.aar` → `app/libs/`; `.ocrpack` → `app/src/main/assets/`:

```kotlin
android { defaultConfig { minSdk = 26 } }
dependencies { implementation(files("libs/oneocr-android-0.1.0.aar")) }
```

```java
// Run on a background thread. Reuse the engine.
try (dev.oneocr.OneOcr ocr = dev.oneocr.OneOcr.fromAsset(
        context, "oneocr-cjk-en.ocrpack", 2)) {
    String json = ocr.recognize(pngOrJpegBytes);
}
```

AAR: Java + JNI + Go native + ORT. `fromAsset` copies one file to private storage; no resource extraction.

### C++17

```cmake
find_package(OneOCR CONFIG REQUIRED)
add_executable(example main.cpp)
target_link_libraries(example PRIVATE OneOCR::oneocr)
```

`-DCMAKE_PREFIX_PATH=/path/to/oneocr-sdk-darwin-arm64`:

```cpp
#include <oneocr.hpp>
#include <iostream>
int main() {
    oneocr::Engine engine("oneocr-cjk-en.ocrpack");
    std::cout << engine.recognizeFile("image.png") << '\n';
}
```

### Python 3.11–3.13

```bash
python -m pip install ./python
# Or: python -m pip install dist/oneocr_native-0.1.0-py3-none-any.whl
oneocr-native recognize --model models/oneocr-cjk-en.ocrpack image.png
```

```python
from oneocr_native import OneOcrEngine

with OneOcrEngine("models/oneocr-cjk-en.ocrpack") as engine:
    print(engine.recognize("image.png").text)  # PIL.Image.Image also accepted
```

## Validation and limits

| Platform | Go / C++ | Independent Python | Android |
|---|---|---|---|
| macOS ARM64 | OCR, C++/external Go and relocated SDK tested | OCR and isolated wheel installation tested | Cross-build host |
| Android arm64-v8a / x86_64, API 26+ | Go native + JNI built | No Android Python distribution | AAR/consumer compilation and ELF checks; see emulator compatibility issue below |
| Windows x64 | CPU OCR/C++ validated; CUDA ran with the result differences below | CPU OCR, independent stages and package tests validated | N/A |
| Linux desktop | Basic CI passed; no manual host validation | CI passed; no manual host validation | N/A |

This is not full parity with the original DLL. Automatic mode skips unavailable scripts with warnings; explicit unavailable scripts fail. Results include text, line quads, scripts and warnings; `confidence` and `words` are null. Grouping and reading order remain experimental; handwriting and natural vertical CJK are unvalidated. Complex inverse RTL is ambiguous: Python uses ICU on macOS and a portable approximation elsewhere, as does Go.

Go/C++/Android use full ONNX Runtime 1.29 CPU with contrib ops. Desktop SDKs and AAR include it; Go source alone requires a separate runtime. Python installs ORT through pip. Compilation does not establish runtime compatibility on every platform.

Android compatibility: an ARM64 emulator hit SIGILL in the ORT 1.29 default KleidiAI path. A standalone detector probe ran with that path disabled, but this SDK does not yet apply the workaround; full Android OCR remains unvalidated.

## Documentation

[Go](docs/GO.md) · [Python](python/README.md) · [SDK](sdk/SDK.md) · [Build](docs/BUILD.md) · [OCRPACK](docs/PACK_FORMAT.md) · [Resources](docs/BUNDLE.md) · [OneModel](docs/FORMAT.md) · [Validation](validation/migration.json)

Source: [MIT](LICENSE). [Third-party notices](THIRD_PARTY_NOTICES.md).

## Frequent calls and experimental acceleration

Go/C/C++ keep CPU as the default and allow explicit CoreML, CUDA and DirectML selection, device/stage fallback settings, and character-class restrictions. Reuse Engines, call `Warmup`, and use a small number of independent Engines for occasional concurrency. Recognition avoids an extra copy of the probability matrix.

The offline `oneocr-native adapt` tool creates source-verified experimental model variants; originals remain available. Provider registration is not proof of GPU execution or a speedup. CoreML and Windows CUDA have executed, but adapted models change results. A compatible DirectML runtime has not yet been validated. See the [acceleration guide](docs/ACCELERATION.md).

Independent detection and cropped-line recognition: [API guide](docs/STAGES.md). Go, CLI, C/C++ and Python expose separate entry points.

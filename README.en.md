# oneocr-native

[简体中文](README.md) · [English](README.en.md) · [日本語](README.ja.md)

[![Go Reference](https://pkg.go.dev/badge/github.com/shiyori/oneocr-native.svg)](https://pkg.go.dev/github.com/shiyori/oneocr-native)
[![Stars](https://img.shields.io/github/stars/shiyori/oneocr-native)](https://github.com/shiyori/oneocr-native/stargazers)
[![Downloads](https://img.shields.io/github/downloads/shiyori/oneocr-native/total)](https://github.com/shiyori/oneocr-native/releases)
[![Release](https://img.shields.io/github/v/release/shiyori/oneocr-native)](https://github.com/shiyori/oneocr-native/releases)
[![License](https://img.shields.io/badge/license-AGPL--3.0--only-blue.svg)](LICENSE)

Offline OCR for Chinese, Japanese, Korean, English and digits, with Go, Android, C++ and independent Python SDKs. All use one default model; no model selection or per-call model path is needed.

## Setup

```bash
git clone https://github.com/shiyori/oneocr-native.git
cd oneocr-native
```

The default [oneocr-cjk-en.ocrpack](models/oneocr-cjk-en.ocrpack) is in `models/`. Commands run from the repository root find it automatically. For another project, place this file in `models/` under the working directory or next to the executable. Android uses assets as shown below. The model is separate from the SDK; inference works offline.

## Go / CLI

Requires Go ≥1.24, CGO and ONNX Runtime 1.29 CPU. Supply the runtime once when installing from source. Desktop SDKs include it, so `oneocr install` needs no arguments.

```bash
go install ./cmd/oneocr
oneocr install --runtime /path/to/libonnxruntime.dylib
oneocr recognize image.png
oneocr recognize --format json image.png
```

```go
package main

import (
    "context"
    "fmt"
    "log"

    oneocr "github.com/shiyori/oneocr-native"
)

func main() {
    engine, err := oneocr.Open(oneocr.Config{})
    if err != nil { log.Fatal(err) }
    defer engine.Close()

    result, err := engine.RecognizeFile(context.Background(), "image.png", oneocr.Options{})
    if err != nil { log.Fatal(err) }
    fmt.Println(result.Text)
}
```

After installation, run from any directory. `ONEOCR_RUNTIME` can also select the runtime: `onnxruntime.dll` on Windows, `libonnxruntime.dylib` on macOS, or `libonnxruntime.so` on Linux. See the [Go guide](docs/GO.md) for external projects, memory inputs and timeouts.

## Android

Put the AAR in `app/libs/` and the default model in `app/src/main/assets/oneocr-cjk-en.ocrpack`. Add this to the application module's `build.gradle.kts`:

```kotlin
android {
    defaultConfig {
        minSdk = 26
        ndk { abiFilters += listOf("arm64-v8a", "x86_64") }
    }
}
dependencies {
    implementation(files("libs/oneocr-android-0.1.0.aar"))
}
```

```java
import dev.oneocr.OneOcr;
import org.json.JSONObject;

// Execute on a background thread; the caller handles IOException / JSONException.
try (OneOcr engine = OneOcr.fromAsset(context)) {
    String json = engine.recognize(bitmap);
    String text = new JSONObject(json).getString("text");
    // engine.recognize(pngOrJpegBytes) is also available.
}
```

The AAR includes JNI, Go native and ONNX Runtime; do not add a second ORT dependency. `fromAsset(context)` streams the default asset into private app storage. Reuse the engine for multiple images and close it when finished. See the [SDK guide](sdk/SDK.md) for building and integrating the AAR.

## C++17

Extract the desktop SDK and preserve `include/`, `lib/` and `lib/cmake/OneOCR/`. Link it from your application's `CMakeLists.txt`:

```cmake
cmake_minimum_required(VERSION 3.22)
project(ocr_example LANGUAGES CXX)
find_package(OneOCR CONFIG REQUIRED)
add_executable(ocr_example main.cpp)
target_link_libraries(ocr_example PRIVATE OneOCR::oneocr)
```

```cpp
#include <oneocr.hpp>
#include <iostream>

int main() {
    try {
        oneocr::Engine engine;
        std::cout << engine.recognizeFile("image.png") << '\n';
    } catch (const std::exception& error) {
        std::cerr << error.what() << '\n';
        return 1;
    }
}
```

```bash
cmake -S . -B build -DCMAKE_PREFIX_PATH=/absolute/path/to/oneocr-sdk
cmake --build build --config Release
```

Place the default model in `models/` under the application's working directory. Runtime libraries are included in the SDK. Results are UTF-8 JSON; the engine owns its resources and also accepts encoded bytes and RGB buffers. See the [SDK guide](sdk/SDK.md) for the C API and deployment.

## Python 3.11–3.13

Install from the repository root; pip installs the runtime dependencies. A built wheel can be installed with `python -m pip install oneocr_native-0.1.0-py3-none-any.whl`.

```bash
python -m pip install ./python
oneocr-native recognize image.png
oneocr-native recognize --format json image.png
```

```python
from oneocr_native import OneOcrEngine

with OneOcrEngine() as engine:
    result = engine.recognize("image.png")
    print(result.text)
    for line in result.lines:
        print(line.text, line.quad)
```

Pillow images are accepted too. Python runs its own OCR pipeline without the Go shared library. When running elsewhere, place the default model as described in Setup. See the [Python guide](python/README.md) for image inputs, detection and cropped-line recognition.

## Documentation

[Go](docs/GO.md) · [Python](python/README.md) · [Android / C++ / C](sdk/SDK.md) · [Build](docs/BUILD.md) · [Detection and cropped-line recognition](docs/STAGES.md)

## License and notice

Source is licensed under **AGPL-3.0-only**, GNU AGPL version 3 only; see [LICENSE](LICENSE). Third-party models and dependencies retain their own rights and licenses; see [third-party notices](THIRD_PARTY_NOTICES.md).

This unofficial implementation is shared for learning and discussion. It does not represent an original vendor product or service and is provided without warranty.

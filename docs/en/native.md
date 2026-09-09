# C and C++

[简体中文](../zh-CN/native.md) | [English](../en/native.md) | [日本語](../ja/native.md)

Use the [complete desktop SDK](installation.md). No runtime path is needed. C++ requires C++17.

## CMake and C++

Create `main.cpp` in your application's directory:

```cpp
#include <oneocr.hpp>
#include <iostream>

int main() {
    oneocr::Engine engine;
    std::cout << engine.recognize(oneocr::Input::fromFile("image.png"));
}
```

Add `CMakeLists.txt`:

```cmake
cmake_minimum_required(VERSION 3.22)
project(ocr_app LANGUAGES CXX)
find_package(OneOCR CONFIG REQUIRED)
add_executable(ocr_app main.cpp)
target_link_libraries(ocr_app PRIVATE OneOCR::oneocr)
oneocr_copy_dependencies(ocr_app)
```

Build from that same application directory:

```sh
cmake -S . -B build -DCMAKE_PREFIX_PATH=/path/to/sdk
cmake --build build --config Release
```

`oneocr_copy_dependencies` places the available SDK libraries and model beside your executable. The core SDK uses resources prepared by `bin/oneocr install`.

```cpp
oneocr::Options options;
options.threads = 2;
oneocr::Engine engine(options);
auto input = oneocr::Input::fromEncoded(imageBytes);
auto regions = engine.detect(input);
auto line = engine.recognizeLine(input, {"CJK", 30000});
```

`Input::fromFile`, `fromEncoded` and `fromPixels` are shared by all three operations. Byte inputs remain borrowed for the synchronous call. Engines own their handles, support moves and release resources on destruction. `close()`, `warmup()` and `diagnostics()` are also available.

## C

Link the SDK's native library and include `oneocr.h`:

```c
#include <oneocr.h>
#include <stdio.h>

int main(void) {
    char *error = NULL;
    uint64_t engine = OneOCROpen(NULL, &error);
    if (!engine) { fprintf(stderr, "%s\n", error); OneOCRFree(error); return 1; }
    OneOCRInput input = {0};
    input.kind = ONEOCR_FILE;
    input.path = "image.png";
    char *json = OneOCRRecognize(engine, &input, NULL, &error);
    if (json) { puts(json); OneOCRFree(json); }
    else { fprintf(stderr, "%s\n", error); OneOCRFree(error); }
    if (OneOCRClose(engine, &error) != 0) { fprintf(stderr, "%s\n", error); OneOCRFree(error); }
    return 0;
}
```

`OneOCROpen` takes an optional `OneOCRConfig`. Each operation takes `OneOCRInput` and optional `OneOCRCallOptions`; `timeout_ms=0` means no deadline. The returned UTF-8 JSON follows the [result structures](stages.md). Free each non-null result or error with `OneOCRFree`.

Use `ONEOCR_ENCODED` with `data/length`, or `ONEOCR_PIXELS` with buffer dimensions, row stride, pixel format and premultiplied-alpha flag. File inputs use `ONEOCR_FILE` and `path`. The SDK does not retain these pointers after return. Stop submitting calls before closing a handle.

---

[oneocr-native](../../README.en.md) · [AGPL-3.0-only](../../LICENSE)

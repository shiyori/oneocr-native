# oneocr-native

[简体中文](README.md) · [English](README.en.md) · [日本語](README.ja.md)

[![Go Reference](https://pkg.go.dev/badge/github.com/shiyori/oneocr-native.svg)](https://pkg.go.dev/github.com/shiyori/oneocr-native)
[![Stars](https://img.shields.io/github/stars/shiyori/oneocr-native)](https://github.com/shiyori/oneocr-native/stargazers)
[![Downloads](https://img.shields.io/github/downloads/shiyori/oneocr-native/total)](https://github.com/shiyori/oneocr-native/releases)
[![Release](https://img.shields.io/github/v/release/shiyori/oneocr-native)](https://github.com/shiyori/oneocr-native/releases)
[![License](https://img.shields.io/badge/license-AGPL--3.0--only-blue.svg)](LICENSE)

支持中日韩英和数字的离线 OCR，提供 Go、Android、C++、Python SDK。

## 准备

```bash
git clone https://github.com/shiyori/oneocr-native.git
cd oneocr-native
```

仓库已包含[默认模型](models/oneocr-cjk-en.ocrpack)。接入自己的项目时，将模型放入 `models/`；Android 放入 assets。

## Go / CLI

源码需 Go ≥1.24、CGO 和 ONNX Runtime 1.29 CPU；桌面 SDK 已包含运行库，安装时可省略 `--runtime`。

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

更多安装方式和图片输入见 [Go 使用指南](docs/GO.md)。

## Android

将 AAR 放入 `app/libs/`，把默认模型放入 `app/src/main/assets/oneocr-cjk-en.ocrpack`。在应用模块的 `build.gradle.kts` 添加：

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

AAR 已包含运行库。完整接入方法见 [SDK 使用指南](sdk/SDK.md)。

## C++17

解压桌面 SDK，保留 `include/`、`lib/` 和 `lib/cmake/OneOCR/`。在应用项目的 `CMakeLists.txt` 中链接 SDK：

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

运行库随 SDK 提供，识别结果为 JSON。C API 和部署方式见 [SDK 使用指南](sdk/SDK.md)。

## Python 3.11–3.13

在仓库根目录运行以下命令，依赖由 pip 安装：

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

更多图片输入和调用方式见 [Python 使用指南](python/README.md)。

## 文档

[Go](docs/GO.md) · [Python](python/README.md) · [Android / C++ / C](sdk/SDK.md) · [构建](docs/BUILD.md) · [独立检测与单行识别](docs/STAGES.md)

## 许可证与声明

源码采用 [AGPL-3.0-only](LICENSE)。模型与依赖见 [第三方声明](THIRD_PARTY_NOTICES.md)。

本项目为非官方实现，供交流学习，不代表原厂产品或服务，不提供任何担保。

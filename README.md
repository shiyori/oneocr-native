# oneocr-native

[简体中文](README.md) · [English](README.en.md) · [日本語](README.ja.md)

[![Go Reference](https://pkg.go.dev/badge/github.com/shiyori/oneocr-native.svg)](https://pkg.go.dev/github.com/shiyori/oneocr-native)
[![Stars](https://img.shields.io/github/stars/shiyori/oneocr-native)](https://github.com/shiyori/oneocr-native/stargazers)
[![Downloads](https://img.shields.io/github/downloads/shiyori/oneocr-native/total)](https://github.com/shiyori/oneocr-native/releases)
[![Release](https://img.shields.io/github/v/release/shiyori/oneocr-native)](https://github.com/shiyori/oneocr-native/releases)
[![License](https://img.shields.io/badge/license-AGPL--3.0--only-blue.svg)](LICENSE)

离线 OCR，支持中文、日文、韩文、英文和数字，提供 Go、Android、C++ 和独立 Python SDK。默认使用同一个模型，无需选择模型或在每次调用时传入路径。

## 准备

```bash
git clone https://github.com/shiyori/oneocr-native.git
cd oneocr-native
```

默认模型 [oneocr-cjk-en.ocrpack](models/oneocr-cjk-en.ocrpack) 已放在仓库的 `models/` 中。从仓库根目录运行可直接找到它；接入自己的项目时，将这个文件放入工作目录或程序目录的 `models/`。Android 使用下面的 assets 方式。模型与 SDK 分开存放，推理无需联网。

## Go / CLI

需要 Go ≥1.24、CGO 和 ONNX Runtime 1.29 CPU。源码安装时指定一次运行库；桌面 SDK 已带运行库，可直接执行 `oneocr install`。

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

安装后可在其他目录直接调用。运行库也可通过 `ONEOCR_RUNTIME` 设置；Windows 使用 `onnxruntime.dll`，macOS 使用 `libonnxruntime.dylib`，Linux 使用 `libonnxruntime.so`。在其他 Go 项目中接入源码、内存图片和超时调用见 [Go 使用指南](docs/GO.md)。

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

AAR 包含 JNI、Go native 和 ONNX Runtime，不需要再添加 ORT 依赖。`fromAsset(context)` 会把默认模型流式保存到应用私有目录；连续识别时复用引擎，结束后关闭。AAR 构建与完整接入方法见 [SDK 使用指南](sdk/SDK.md)。

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

默认模型放入应用工作目录的 `models/`，运行时库随 SDK 提供。返回值是 UTF-8 JSON；引擎自动释放资源，也支持编码字节与 RGB 输入。C API 和部署方式见 [SDK 使用指南](sdk/SDK.md)。

## Python 3.11–3.13

在仓库根目录安装，依赖由 pip 一并安装；也可用 `python -m pip install oneocr_native-0.1.0-py3-none-any.whl` 安装构建好的 wheel。

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

也支持 Pillow 图片对象。Python SDK 自己执行 OCR 管线，不依赖 Go 动态库；从其他目录运行时按“准备”一节放置默认模型。图片输入、检测与单行识别见 [Python 使用指南](python/README.md)。

## 文档

[Go](docs/GO.md) · [Python](python/README.md) · [Android / C++ / C](sdk/SDK.md) · [构建](docs/BUILD.md) · [独立检测与单行识别](docs/STAGES.md)

## 许可证与声明

源码采用 **AGPL-3.0-only**，仅适用 GNU AGPL 第 3 版，见 [LICENSE](LICENSE)。第三方模型和依赖保留各自的权利与许可，见 [第三方声明](THIRD_PARTY_NOTICES.md)。

本项目为非官方实现，供交流学习，不代表原厂产品或服务，不提供任何担保。

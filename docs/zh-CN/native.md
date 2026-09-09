# C 与 C++ 接入

[简体中文](../zh-CN/native.md) | [English](../en/native.md) | [日本語](../ja/native.md)

Linux 使用[完整 SDK](installation.md)，无需指定运行库路径。C++ 要求 C++17。Windows/macOS 如需 C/C++ 共享库，按[开发指南](development.md)自行构建；不发布平台专属包。

## CMake 与 C++

在自己的应用目录创建 `main.cpp`：

```cpp
#include <oneocr.hpp>
#include <iostream>

int main() {
    oneocr::Engine engine;
    std::cout << engine.recognize(oneocr::Input::fromFile("image.png"));
}
```

添加 `CMakeLists.txt`：

```cmake
cmake_minimum_required(VERSION 3.22)
project(ocr_app LANGUAGES CXX)
find_package(OneOCR CONFIG REQUIRED)
add_executable(ocr_app main.cpp)
target_link_libraries(ocr_app PRIVATE OneOCR::oneocr)
oneocr_copy_dependencies(ocr_app)
```

在同一个应用目录构建：

```sh
cmake -S . -B build -DCMAKE_PREFIX_PATH=/path/to/sdk
cmake --build build --config Release
```

`oneocr_copy_dependencies` 会把 SDK 中的库和模型复制到可执行文件旁。精简包则使用 `bin/oneocr install` 准备好的资源。

```cpp
oneocr::Options options;
options.threads = 2;
oneocr::Engine engine(options);
auto input = oneocr::Input::fromEncoded(imageBytes);
auto regions = engine.detect(input);
auto line = engine.recognizeLine(input, {"CJK", 30000});
```

`Input::fromFile`、`fromEncoded`、`fromPixels` 由三个操作共用。字节缓冲区在同步调用期间保持有效。Engine 管理句柄所有权，支持移动，在析构时释放资源；也提供 `close()`、`warmup()` 和 `diagnostics()`。

## C

链接 SDK 原生库并引用 `oneocr.h`：

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

`OneOCROpen` 接受可选的 `OneOCRConfig`。每种操作只使用一个 `OneOCRInput` 和可选的 `OneOCRCallOptions`；`timeout_ms=0` 表示不设置超时。返回 UTF-8 JSON，字段见[识别结果](stages.md)。每个非空结果或错误字符串均需用 `OneOCRFree` 释放。

编码图片使用 `ONEOCR_ENCODED` 和 `data/length`；像素缓冲区使用 `ONEOCR_PIXELS`，填写尺寸、行跨度、像素格式与预乘 Alpha 标志；文件使用 `ONEOCR_FILE` 和 `path`。返回后 SDK 不保留这些指针。关闭句柄前停止提交新调用。

---

[oneocr-native](../../README.md) · [AGPL-3.0-only](../../LICENSE)

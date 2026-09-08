# Android、C++ 与 C SDK

四端共用默认模型 `oneocr-cjk-en.ocrpack`，支持中日韩英和数字。模型与 SDK 分开存放；桌面应用放入工作目录的 `models/`，Android 放入 assets。

## Android

### 添加依赖

将 `oneocr-android-0.1.0.aar` 放入 `app/libs/`，模型放入 `app/src/main/assets/oneocr-cjk-en.ocrpack`。应用模块的 `build.gradle.kts`：

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

AAR 包含 Java、JNI、Go native、ONNX Runtime 1.29 和 JNI keep 规则，不需要重复引入 ORT。构建 AAR 的命令见 [源码构建](https://github.com/shiyori/oneocr-native/blob/main/docs/BUILD.md)。

### 调用

在后台线程执行，识别完成后再把文本交给 UI：

```java
import android.content.Context;
import android.graphics.Bitmap;
import dev.oneocr.OneOcr;
import java.io.IOException;
import org.json.JSONException;
import org.json.JSONObject;

public static String readText(Context context, Bitmap image)
        throws IOException, JSONException {
    try (OneOcr engine = OneOcr.fromAsset(context)) {
        return new JSONObject(engine.recognize(image)).getString("text");
    }
}
```

已有 PNG/JPEG 字节时使用 `engine.recognize(encodedBytes)`。连续处理图片时，把引擎保存为后台任务的成员并复用，在任务结束时调用 `close()`。`OneOcr.fromAsset(context, 2)` 可指定 CPU 线程数。模型由 SDK 流式复制到应用私有目录，不展开内部资源。

独立步骤可用 `engine.detect(encodedBytes)` 和 `engine.recognizeLine(encodedBytes)`，返回 JSON。文件或资源访问失败由 `IOException` 表示；原生识别失败以运行时异常返回。

## C++17

### CMake

解压桌面 SDK，保留 `include/`、`lib/` 与 `lib/cmake/OneOCR/` 的相对结构。创建 `CMakeLists.txt`：

```cmake
cmake_minimum_required(VERSION 3.22)
project(ocr_app LANGUAGES CXX)
find_package(OneOCR CONFIG REQUIRED)
add_executable(ocr_app main.cpp)
target_link_libraries(ocr_app PRIVATE OneOCR::oneocr)
```

创建 `main.cpp`：

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

在应用工作目录放置 `models/oneocr-cjk-en.ocrpack` 后运行生成的程序。SDK 已带 ORT；发布应用时一同部署对应平台的 native 库。macOS/Linux 可使用 CMake target 配置的运行库搜索路径；Windows 将所需 DLL 放在可执行文件旁。

### 内存输入与结果

```cpp
std::string json = engine.recognize(encodedBytes); // std::vector<uint8_t>
std::string regions = engine.detect(encodedBytes);
std::string line = engine.recognizeLine(croppedLineBytes);
```

RGB 缓冲使用 `recognizeRGB(data, size, width, height, stride)`；`size` 是缓冲长度，`stride` 是每行字节数。返回字符串为 UTF-8 JSON，包含文本和行坐标等字段。C++ 包装器自动管理引擎句柄和返回字符串；失败时抛出 `std::runtime_error`。

SDK 的命令行程序也可直接使用：

```bash
./bin/oneocr-cpp image.png
./bin/oneocr recognize image.png
```

## C API

包含 `oneocr.h`，链接 SDK native 库。默认模型和运行库均可传 `NULL`：

```c
#include <oneocr.h>
#include <stdio.h>

int recognize(const unsigned char *image, size_t length) {
    char *error = NULL;
    uint64_t engine = OneOCROpen(NULL, NULL, 2, &error);
    if (!engine) {
        fprintf(stderr, "%s\n", error ? error : "cannot open OCR");
        OneOCRFree(error);
        return 1;
    }
    char *result = OneOCRRecognizeEncoded(engine, image, length, NULL, &error);
    int failed = result == NULL;
    if (result) puts(result);
    else fprintf(stderr, "%s\n", error ? error : "recognition failed");
    OneOCRFree(result);
    OneOCRFree(error);
    error = NULL;
    OneOCRClose(engine, &error);
    OneOCRFree(error);
    return failed;
}
```

所有非 NULL 返回字符串和错误字符串都由 `OneOCRFree` 释放。输入缓冲在同步调用结束前必须保持有效。`OneOCROpenWithOptions("{\"threads\":2}", &error)` 可传配置 JSON，仍使用默认模型。带超时的接口以毫秒计时，0 表示无超时。

## Go 与 Python

Go 源码 SDK 见 [Go 指南](https://github.com/shiyori/oneocr-native/blob/main/docs/GO.md)。在桌面 SDK 中执行 `bin/oneocr install` 后，可通过 `oneocr.Open(oneocr.Config{})` 直接创建引擎。

独立 Python wheel 使用 `python -m pip install oneocr_native-0.1.0-py3-none-any.whl` 安装，通过 `OneOcrEngine()` 调用；见 [Python 指南](https://github.com/shiyori/oneocr-native/blob/main/python/README.md)。Python 使用自己的 ONNX Runtime 包，不依赖 native SDK。

源码按 **AGPL-3.0-only** 授权，第三方模型与依赖保留各自许可及权利。

本项目为非官方实现，供交流学习，不代表原厂产品或服务，不提供任何担保。

# OneOCR 可直接接入的 SDK

默认使用 `oneocr-cjk-en.ocrpack`，需要西里尔和阿拉伯文字时选择 `oneocr-extended.ocrpack`。模型与 SDK 独立分发，一个模型文件适用于所有支持的平台。当前本地构建内容：Android AAR（arm64-v8a、x86_64）、macOS ARM64 C++/Go/CLI SDK，以及可编译的 Go 模块源码包，以及独立 Python wheel。Windows/Linux 没有本次预编译验收产物。

## Android：引用一个 AAR

把 `oneocr-android-0.1.0.aar` 放入 `app/libs`，模型文件放入 `app/src/main/assets/`。Gradle Kotlin DSL：

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

AAR 已包含 Java 类、Go/JNI 库和 ONNX Runtime 1.29.0，不需要再添加 ORT Maven 依赖或手动寻找 runtime 路径。包含 JNI consumer keep 规则。使用标准 Android native library 打包即可；运行时按库名通过应用 linker namespace 加载，兼容库保留在 APK 内的方式。

```java
// 在后台线程创建并复用；不要在每一帧重新创建引擎。
try (dev.oneocr.OneOcr ocr = dev.oneocr.OneOcr.fromAsset(
        context, "oneocr-cjk-en.ocrpack", 2)) {
    String json = ocr.recognize(pngOrJpegBytes);
    // 同时支持 ocr.recognize(bitmap)。结果为 UTF-8 JSON，可用 JSONObject 读取。
}
```

`fromAsset` 用 64 KiB 缓冲流式部署一个模型文件到应用私有目录，按 SHA-256 命名并复用；不解包内部资源，也不把完整模型存入 Java byte[]。也可直接 `new OneOcr(modelPath, 2)` 加载已有私有文件。保留三参数构造函数用于显式管理 ORT 的应用。

Android 已完成两个 ABI 的 native 构建、AAR 打包、Java 消费者编译与 ELF 依赖／16 KiB 页对齐检查。ARM64 模拟器曾在 ORT 1.29 默认 KleidiAI 路径触发 SIGILL；独立检测器探针禁用该路径后可运行，但该兼容选项尚未接入 SDK，完整 Android OCR 未验收。不把编译成功当作设备验收。

## C++：头文件与 CMake target

解压桌面 SDK，保留 `include/`、`lib/`、`lib/cmake/OneOCR/` 的相对结构。SDK 的 `lib/` 已包含 OneOCR 和 ORT，无需另外配置 ORT 路径。

```cmake
cmake_minimum_required(VERSION 3.22)
project(example LANGUAGES CXX)
find_package(OneOCR CONFIG REQUIRED)
add_executable(example main.cpp)
target_link_libraries(example PRIVATE OneOCR::oneocr)
```

配置 CMake 时传 `-DCMAKE_PREFIX_PATH=/absolute/path/oneocr-sdk-darwin-arm64`。

```cpp
#include <oneocr.hpp>
#include <iostream>

int main() {
    oneocr::Engine engine("/absolute/path/oneocr-cjk-en.ocrpack");
    std::cout << engine.recognizeFile("/absolute/path/image.png") << '\n';
}
```

C++17 RAII 包装器处理句柄和返回字符串释放，支持文件、编码字节和 RGB 输入，返回 JSON，不要求额外 JSON 库。底层 C ABI `oneocr.h` 同样可直接使用；`OneOCROpen` 兼容旧目录输入，runtime 参数可为 NULL 使用 SDK 自动定位。

SDK 内的 `bin/oneocr-cpp MODEL IMAGE` 可直接运行。发布到自己的应用时保留 native 库可被加载的位置；macOS 库使用 `@rpath`／`@loader_path`，应用发布签名按自己的发布流程处理。

## Go：模块 API 与独立安装

桌面 SDK 内含 `go/` 模块和预编译 `bin/oneocr`。不依赖 Python/OpenCV。将模型放到解压后的 SDK 根目录，可直接：

```bash
./bin/oneocr recognize --model ./oneocr-cjk-en.ocrpack image.png
./bin/oneocr install --model ./oneocr-cjk-en.ocrpack
./bin/oneocr recognize image.png
```

安装命令自动从 SDK `lib/` 定位 ORT，复制模型包和运行时，保存用户配置。可用 `--home` 或 `ONEOCR_HOME` 隔离安装。

Go API：

```go
engine, err := oneocr.OpenInstalled("") // 使用上述安装，无需提供模型／ORT路径
if err != nil { return err }
defer engine.Close()
result, err := engine.RecognizeFile(ctx, "image.png", oneocr.Options{})
```

也可 `oneocr.Open(oneocr.Config{ModelPath: modelFile})`：SDK 按 `ONEOCR_RUNTIME`、模型／可执行文件旁的 `lib/` 和平台加载器定位 ORT。显式设置 `RuntimeLibrary` 可覆盖；仅下载 Go 源码包时需自行提供平台 ORT。

当前模块未发布远程 tag。其他 Go 工程使用本地 SDK：

```bash
go mod edit -replace github.com/shiyori/oneocr-native=/absolute/path/sdk/go
go get github.com/shiyori/oneocr-native
```

首次获取 Go 依赖可能联网；完成构建后，OCR 和安装过程均可离线。源代码示例还支持 `go run ./examples/basic MODEL IMAGE` 或仅传 `IMAGE` 使用现有安装。

## 返回值与边界

两种模型包只改变资源集合和加载方式，没有重新训练或融合模型。输出包含 `text`、`lines`、坐标、脚本、尺寸、耗时和 `warnings`，`confidence`／`words` 继续为 null。包外文字体系在自动模式下跳过并返回警告，强制指定包外脚本报错。

Go 可通过 context 取消；C/C++ 的新增超时接口也可取消等待和推理。接口仍同步执行，Android JNI 保持现有同步 CPU 入口，应在后台执行。旧 bundle 目录仍兼容。容器规范见随包的 `PACK_FORMAT.md`。SDK 的模型不内嵌在 AAR、动态库或 Go 模块中，更新模型只需替换单个 `.ocrpack`。

## 独立 Python wheel

`oneocr_native-0.1.0-py3-none-any.whl` 独立分发，使用 `python -m pip install <wheel>` 安装。

```python
from oneocr_native import OneOcrEngine
with OneOcrEngine("oneocr-cjk-en.ocrpack") as engine:
    print(engine.recognize("image.png").text)
```

Python 3.11–3.13，完整 OCR 管线由 Python 自己执行，直接调用 pip 安装的 ONNX Runtime；不依赖这些 SDK 中的 Go 动态库。接受图片路径和 Pillow 图片，提供 `from_package()`、`from_bundle()`、`close()` 和上下文管理器。原始 OneModel 输入仍可使用缓存转换。关闭后识别报错。

源码使用 MIT，第三方模型和依赖保持各自权利。用途定位为交流学习，不增加 MIT 限制。开发构建与独立 Python 源码见 [项目仓库](https://github.com/shiyori/oneocr-native)。

## C/C++ 加速、预热与诊断

旧 `OneOCROpen` 和识别函数保持兼容、默认 CPU。新增 `OneOCROpenWithOptions` 接受 Go Config 的 JSON 字段，包括 `backend`、`device_id`、`fallback`、`stage_backends`、`adaptation_dir`、`cache_dir`、`profiling_dir` 和 `character_classes`。原模型与实验模型需要分开部署。

```cpp
auto engine = oneocr::Engine::fromOptions(R"({
  "model_path": "oneocr-cjk-en.ocrpack",
  "backend": "coreml", "fallback": "cpu", "threads": 2
})");
engine.warmup(30000);
auto result = engine.recognizeFile("image.png", "", 30000);
auto diagnostics = engine.closeWithDiagnostics();
```

C 入口对应 `OneOCRWarmup`、`OneOCRRecognizeEncodedWithTimeout`、`OneOCRRecognizeRGBWithTimeout`、`OneOCRDiagnostics` 和 `OneOCRCloseWithDiagnostics`。超时单位为毫秒，0 表示无超时，上限 86400000；计时包含等待 Engine，设备内核停止可能晚于 deadline。所有返回字符串继续由 `OneOCRFree` 释放。

诊断区分请求后端、注册后端与实际 kernel profile。profiling 在 Close 后完成；未测量的分配不能当作 GPU 执行证明。DirectML 同一 session 串行；并发使用各自独立的 Engine。CoreML 编译缓存按模型内容、ORT 版本、平台和配置隔离。CUDA/DirectML 需要额外的目标运行时及真机验证；不能仅凭本机 C ABI 构建通过宣称支持。

Python wheel 保持独立 CPU 识别入口，新增 `oneocr-native adapt` 离线转换工具；Go/C/C++ 运行不依赖 Python。详细模型转换和持续调用说明位于源码 `docs/ACCELERATION.md`。

Independent detection and cropped-line recognition: [API guide](../docs/STAGES.md). Go, CLI, C/C++ and Python expose separate entry points.

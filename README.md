# oneocr-native

[简体中文](README.md) · [English](README.en.md) · [日本語](README.ja.md)

[![Go Reference](https://pkg.go.dev/badge/github.com/shiyori/oneocr-native.svg)](https://pkg.go.dev/github.com/shiyori/oneocr-native)
[![Go Report Card](https://goreportcard.com/badge/github.com/shiyori/oneocr-native)](https://goreportcard.com/report/github.com/shiyori/oneocr-native)
[![Stars](https://img.shields.io/github/stars/shiyori/oneocr-native)](https://github.com/shiyori/oneocr-native/stargazers)
[![Downloads](https://img.shields.io/github/downloads/shiyori/oneocr-native/total)](https://github.com/shiyori/oneocr-native/releases)
[![Release](https://img.shields.io/github/v/release/shiyori/oneocr-native)](https://github.com/shiyori/oneocr-native/releases)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

离线 OneOCR 引擎：Go 根模块、Android AAR、C++ SDK 和独立 Python SDK。一个 `.ocrpack` 模型文件跨端使用，推理时按资源读取，不展开模型目录。Go/C++/Android 不依赖 Python；Python 自己运行完整 OCR 管线，直接使用 ORT，不依赖本项目 Go 动态库。

这是非官方、用于交流学习与研究的实验性实现。“交流学习”不增加 MIT 用途限制；第三方模型保持原有权利。

## 模型选择

| Model | Scripts | Size |
|---|---|---:|
| [oneocr-cjk-en.ocrpack](models/oneocr-cjk-en.ocrpack) | 默认：中日韩英（CJK + Latin） | 30.46 MiB |
| [oneocr-extended.ocrpack](models/oneocr-extended.ocrpack) | 增加西里尔与阿拉伯文字 | 40.07 MiB |

[Model origin / rights / checksums](models/README.md). Models are separate from SDKs and excluded from the root Go module ZIP by a nested data-only module.

## 安装与四端集成

当前提供源码构建；`dist/` 为本地构建产物，不随 Git 推送。尚未发布版本 tag 或 Release。下面使用源码安装；发布根模块版本后可使用 `go install github.com/shiyori/oneocr-native/cmd/oneocr@<tag>`。首次安装依赖可能联网，推理可离线。

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

## 验证与限制

| 平台 | Go / C++ | 独立 Python | Android |
|---|---|---|---|
| macOS ARM64 | 实际 OCR、C++/外部 Go/SDK 换目录验证 | 实际 OCR、wheel 独立安装 | 交叉构建宿主 |
| Android arm64-v8a / x86_64，API 26+ | Go native + JNI 构建 | 未提供 Android Python 包 | AAR/消费者编译及 ELF 检查；模拟器存在下述兼容问题 |
| Windows x64 | CPU 实际 OCR/C++ 验证；CUDA 已执行但存在下述结果差异 | 实际 CPU OCR、独立接口与包测试 | 不适用 |
| Linux 桌面 | CI 基础测试通过；未手工实机验收 | CI 测试通过；未手工实机验收 | 不适用 |

不是原始 DLL 的完整等价复现。自动模式跳过包外脚本并返回警告；强制指定包外脚本报错。输出包含文本、行四边形、脚本和警告；`confidence`、`words` 为 null。检测合并与阅读顺序仍属实验性；未验证手写和自然竖排 CJK。复杂 RTL 混排可能有歧义：Python macOS 使用 ICU，其他平台与 Go 使用可移植近似。

Go/C++/Android 使用完整 ONNX Runtime 1.29 CPU（含 contrib ops）；Android AAR、桌面 SDK 自带运行时。仅 Go 源码需要另行提供 ORT；Python 由 pip 安装匹配的 ORT 包。不要将通过编译视为所有平台运行验证。

Android 兼容性：ARM64 模拟器曾在 ORT 1.29 的默认 KleidiAI 路径触发 SIGILL。独立检测器探针禁用该路径后可运行，但 SDK 尚未接入该兼容配置，完整 Android OCR 未验收。

## Documentation

[Go](docs/GO.md) · [Python](python/README.md) · [SDK](sdk/SDK.md) · [Build](docs/BUILD.md) · [OCRPACK](docs/PACK_FORMAT.md) · [Resources](docs/BUNDLE.md) · [OneModel](docs/FORMAT.md) · [Validation](validation/migration.json)

Source: [MIT](LICENSE). [Third-party notices](THIRD_PARTY_NOTICES.md).

## 高频调用与实验加速

Go/C/C++ 默认 CPU，可显式选择 CoreML、CUDA、DirectML，并配置设备、阶段回退和字符类别。复用 Engine，使用 `Warmup` 预热；偶发并发使用少量独立 Engine。识别输出已减少额外复制，原模型保持可用。

独立的 `oneocr-native adapt` 开发工具生成带源模型校验的实验模型；不把注册成功视为 GPU 加速。CoreML 模型适配和 v2 整数格点检测图已淘汰，旧清单明确报错；检测器保留原量化算子。DirectML 的兼容运行时与阶段收益需真机验证。使用方式、profiling 和数值限制见 [加速与高频识别](docs/ACCELERATION.md)。

Independent detection and cropped-line recognition: [API guide](docs/STAGES.md). Go, CLI, C/C++ and Python expose separate entry points.

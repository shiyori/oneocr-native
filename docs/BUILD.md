# 从源码构建

在仓库根目录执行。桌面端需要 Go ≥1.24、CGO 编译器、CMake ≥3.22 和完整 ONNX Runtime 1.29 CPU 运行库。模型单独放在 `models/oneocr-cjk-en.ocrpack`；SDK 不打包开发备用模型。

## Go CLI

```bash
go install ./cmd/oneocr
oneocr install --runtime /absolute/path/to/libonnxruntime.dylib
oneocr recognize image.png
```

Windows 使用 `onnxruntime.dll`，Linux 使用 `libonnxruntime.so`。运行库也可通过 `ONEOCR_RUNTIME` 指定。

## 桌面 SDK

准备当前平台的 ORT 动态库及官方发行包中的 `LICENSE`、`ThirdPartyNotices.txt`：

```bash
python3 scripts/build_sdk.py --target desktop   --ort-library /path/to/libonnxruntime.dylib   --ort-licenses /path/to/onnxruntime
```

脚本构建本机平台的 Go 动态库、CLI、C++ 示例和 SDK 归档，包含头文件、CMake target、ORT 与依赖许可。`--output /path/to/output` 可指定输出目录。只需 C ABI 时可运行 `scripts/build_shared.sh`。

## Android AAR

准备 JDK ≥17、Android SDK、NDK ≥27，以及 `com.microsoft.onnxruntime:onnxruntime-android:1.29.0` AAR：

```bash
python3 scripts/build_sdk.py --target android   --android-sdk "$ANDROID_HOME"   --android-ndk "$ANDROID_NDK_HOME"   --ort-android-aar /path/to/onnxruntime-android-1.29.0.aar   --ort-licenses /path/to/onnxruntime
```

输出包含 `oneocr-android-0.1.0.aar` 和 Android SDK ZIP。默认包含 `arm64-v8a`、`x86_64`，最低 API 26，native 库采用 16 KiB 页对齐。AAR 已包含 ORT，应用不应再次引入同一运行库。接入方式见 [SDK 使用指南](../sdk/SDK.md)。

## Python wheel

```bash
uv sync --project python --group dev
uv build --project python --out-dir dist
python -m pip install dist/oneocr_native-0.1.0-py3-none-any.whl
```

Python wheel 独立构建，不使用 Go 动态库。构建依赖只在开发环境使用；用户安装时由 pip 安装推理依赖。

所有 SDK 附带项目的 AGPL-3.0-only 许可与上游依赖声明；模型保留第三方权利。研究笔记和本地实验说明不进入 SDK 归档。

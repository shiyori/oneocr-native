# 构建与验证

在仓库根目录执行。构建产物统一放在忽略的 `dist/`，模型为 Git 中 `models/` 的普通文件。首次安装工具/依赖可能联网。

```bash
python3 scripts/verify_models.py
go test ./...
go vet ./...
uv sync --project python --group dev
uv run --project python pytest python/tests
uv run --project python ruff check python/src python/tests scripts
uv build --project python --out-dir dist
```

设置 `ONEOCR_RUNTIME` 为平台完整 ORT 1.29 CPU 动态库，启用 Go 单文件 OCR 验证：

```bash
ONEOCR_RUNTIME=/path/to/libonnxruntime.dylib go test -race ./...
```

`ONEOCR_BUNDLE`、`ONEOCR_MODEL`、`ONEOCR_FIXTURES`、`ONEOCR_PACKAGES` 启用旧完整资源管线与新包的额外回归；这些外部开发输入不进入发行包。测试默认可直接使用已入库的模型与合成图片。Linux/Windows CI 提供基础单元测试；本次手工集成验收在 macOS ARM64，Android 为构建及消费者编译。

## Native SDK

macOS/Linux 主机需 Go、C/C++ 编译器、CMake ≥3.22，ORT 的 `LICENSE`、`ThirdPartyNotices.txt` 和对应动态库：

```bash
python3 scripts/build_sdk.py --target desktop \
  --ort-library /path/to/libonnxruntime.dylib \
  --ort-licenses /path/to/onnxruntime
```

Android 需要 NDK ≥27、JDK ≥17、Android SDK，以及官方 `com.microsoft.onnxruntime:onnxruntime-android:1.29.0` AAR：

```bash
python3 scripts/build_sdk.py --target android \
  --android-sdk "$ANDROID_HOME" --android-ndk "$ANDROID_NDK_HOME" \
  --ort-android-aar /path/to/onnxruntime-android-1.29.0.aar \
  --ort-licenses /path/to/onnxruntime
```

默认编译 `arm64-v8a`、`x86_64`，API 26，native ELF 按 16 KiB 页对齐。AAR 包含 ORT，不应重复添加 ORT AAR。脚本还编译实际 AAR 类库的 Java 消费者；这不替代设备测试。只构建 C ABI 可运行 `scripts/build_shared.sh`；只构建 JNI 可运行 `sdk/android/build-native.sh arm64-v8a`。

`build_sdk.py` 使用显式源码清单生成 Go ZIP，排除模型、Python、dist 和缓存；每份 SDK 附源码 MIT 与上游依赖许可。Python wheel 独立构建，不使用 native SDK。

远程发布留待后续：根模块使用 `v0.1.0` 形式的 tag，不能继续使用旧嵌套模块 tag。当前脚本不会创建提交、标签或 GitHub Release。

## 发行包消费者验证（macOS ARM64）

```bash
python3 scripts/verify_distributions.py --android-ndk "$ANDROID_NDK_HOME"
```

脚本将桌面 SDK 解压到独立临时目录，编译外部 C++/Go 工程，使用新 Python 虚拟环境安装 wheel 并运行 OCR，验证 AAR ELF 和归档文档链接。最终记录写入 `validation/distributions.json`，仅保存相对产物名与验证结果，不保留本机临时路径。此脚本针对 macOS ARM64 SDK；它不声称执行 Android 设备 OCR。

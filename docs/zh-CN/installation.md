# 下载与安装

[简体中文](../zh-CN/installation.md) | [English](../en/installation.md) | [日本語](../ja/installation.md)

推荐使用完整版集成：Android 完整 AAR 和 Linux 完整包都包含默认模型与 ONNX Runtime。代码集成使用 Go/Python 的安装入口准备完整依赖，资源可从 Release、源码仓库或官方依赖仓库取得。

[GitHub Releases · Latest](https://github.com/shiyori/oneocr-native/releases/latest)

## 完整版（推荐）

| | 完整包 |
|---|---|
| Android arm64-v8a / x86_64 | [AAR](https://github.com/shiyori/oneocr-native/releases/latest/download/oneocr-android.aar) |
| Linux x64 | [完整包](https://github.com/shiyori/oneocr-native/releases/latest/download/oneocr-linux-amd64.zip) |
| Linux ARM64 | [完整包](https://github.com/shiyori/oneocr-native/releases/latest/download/oneocr-linux-arm64.zip) |

Linux 完整包解压后即可运行，不需要安装 Go，也无需额外配置模型或 runtime 路径。包内包含 `bin/oneocr`、`models/`、`lib/`、许可证和使用文档，支持 Ubuntu 22.04 及兼容系统。

```sh
/path/to/oneocr-linux-amd64/bin/oneocr recognize image.png
/path/to/oneocr-linux-amd64/bin/oneocr recognize --format json image.png
```

Android 完整 AAR 包含 arm64-v8a / x86_64，最低 API 26。把 AAR 加入应用后按 [Android 指南](android.md)调用。只有已经管理宿主 ORT 的应用才使用指南中的 Core 方案。

## 代码集成

### Go

在项目中运行 `go get github.com/shiyori/oneocr-native@v0.1.2`，通过 `oneocr.Install` 准备完整依赖，再调用 `oneocr.Open`。命令行使用：

```sh
go install github.com/shiyori/oneocr-native/cmd/oneocr@v0.1.2
oneocr install
oneocr recognize image.png
oneocr recognize --format json image.png
```

### Python

Python 3.11–3.13 使用通用 wheel；安装命令自动补齐模型和缺失的运行库：

```sh
python -m pip install "https://github.com/shiyori/oneocr-native/releases/download/v0.1.2/oneocr_native-0.1.2-py3-none-any.whl"
python -m oneocr_native install
python -m oneocr_native recognize image.png
python -m oneocr_native recognize --format json image.png
```

[Go](go.md) · [C/C++](native.md) · [Python](python.md) · [Android](android.md)

## 进阶：依赖与离线准备

安装器优先复用兼容的已有运行库。桌面 Go 安装器从 ONNX Runtime 官方 Release 下载固定版本并校验 SHA-256；Android 准备工具从官方 Maven 仓库获取 AAR；Python 使用 pip 获取依赖。识别过程不下载文件。本项目的新 Release 不再单独发布 runtime ZIP。

离线代码集成可把 `release-manifest.json`、`SHA256SUMS`、默认模型和对应的官方 ORT 原始归档放入同一目录。Linux 使用 `onnxruntime-linux-x64-1.29.0.tgz` 或 `onnxruntime-linux-aarch64-1.29.0.tgz`；Windows/macOS 使用对应官方 ZIP/TGZ；Android 使用 `onnxruntime-android-1.29.0.aar`。然后运行：

```sh
oneocr install --source /path/to/resources --offline
```

Go 代码集成需要 Go 1.24+ 和 C 编译器。Windows 还需满足 [ONNX Runtime 的 Visual C++ 运行时要求](https://onnxruntime.ai/docs/install/#requirements)。已下载 Linux 完整包的用户可直接运行包内命令。

---

[oneocr-native](../../README.md) · [AGPL-3.0-only](../../LICENSE)

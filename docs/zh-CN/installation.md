# 下载与安装

[简体中文](../zh-CN/installation.md) | [English](../en/installation.md) | [日本語](../ja/installation.md)

Go 和 Python 直接使用语言包管理器；Android 下载 AAR。Windows/macOS 不提供平台专属制品，Linux 可选下载预构建包。普通接入不需要 runtime 路径。

[GitHub Releases v0.1.0](https://github.com/shiyori/oneocr-native/releases/tag/v0.1.0)

## Go

代码接入在自己的项目运行 `go get github.com/shiyori/oneocr-native@v0.1.0`，通过 `oneocr.Install` 准备资源后调用 `oneocr.Open`。完整示例见 [Go 接入](go.md)。命令行接入：

```sh
go install github.com/shiyori/oneocr-native/cmd/oneocr@v0.1.0
oneocr install
oneocr recognize image.png
```

## Python

通用 wheel 支持 Python 3.11–3.13，可在任意目录安装：

```sh
python -m pip install "https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr_native-0.1.0-py3-none-any.whl"
python -m oneocr_native install
```

## Android

[AAR](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-android-0.1.0.aar) · [Core AAR](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-android-core-0.1.0.aar)

完整 AAR 包含默认模型与 runtime；已有宿主 ORT 的应用使用 Core AAR。两者均包含 arm64-v8a / x86_64，最低 Android API 26。见 [Android 接入](android.md)。

## Linux 可选下载

| | SDK | Core |
|---|---|---|
| Linux x64 | [SDK](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-sdk-linux-amd64-0.1.0.zip) | [Core](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-core-linux-amd64-0.1.0.zip) |
| Linux ARM64 | [SDK](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-sdk-linux-arm64-0.1.0.zip) | [Core](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-core-linux-arm64-0.1.0.zip) |

完整包包含 CLI、C/C++ 头文件、共享库、默认模型和 ORT。Core 包保留接口和 CLI，不含模型与 ORT。Linux 使用 Ubuntu 22.04 构建基线。Go 接入不需要这些包。

完整包解压后直接运行；Core 先执行安装命令：

```sh
/path/to/sdk/bin/oneocr recognize image.png
/path/to/core-sdk/bin/oneocr install
/path/to/core-sdk/bin/oneocr recognize image.png
```

## 依赖与离线准备

安装器优先复用兼容的已有 runtime。缺失时，Windows/macOS 从 ONNX Runtime 官方 Release 下载固定版本并校验 SHA-256；Linux 可使用本项目的 runtime 包。Python 在缺失 ORT 时通过 pip 从上游安装，保留兼容的已有 CPU/GPU 包。识别过程不下载文件。

离线准备时，将本项目的 `release-manifest.json`、`SHA256SUMS` 和默认模型放入同一目录。Windows/macOS 另外保存官方 `onnxruntime-win-x64-1.29.0.zip` / `onnxruntime-osx-arm64-1.29.0.tgz` 原始归档；Linux 保存本项目对应的 runtime ZIP。安装器负责解包，无需手填 runtime 路径：

```sh
oneocr install --source /path/to/resources --offline
```

Windows 需要满足 [ONNX Runtime 上游的 Visual C++ 运行时前置条件](https://onnxruntime.ai/docs/install/#requirements)。Go 还需要 C 编译器。只有模型会话验证成功后才更新安装配置。

## 语言指南

[Go](go.md) · [C/C++](native.md) · [Python](python.md) · [Android](android.md)

---

[oneocr-native](../../README.md) · [AGPL-3.0-only](../../LICENSE)

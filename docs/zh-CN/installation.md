# 下载与安装

[简体中文](../zh-CN/installation.md) | [English](../en/installation.md) | [日本語](../ja/installation.md)

从 [GitHub Releases v0.1.0-rc.1](https://github.com/shiyori/oneocr-native/releases/tag/v0.1.0-rc.1) 下载，无需克隆源码。常规安装和识别都无需指定 runtime。

| 平台 | 完整离线包 | 精简包 |
|---|---|---|
| Windows x64 | [SDK](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0-rc.1/oneocr-sdk-windows-amd64-0.1.0-rc.1.zip) | [Core](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0-rc.1/oneocr-core-windows-amd64-0.1.0-rc.1.zip) |
| macOS ARM64 | [SDK](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0-rc.1/oneocr-sdk-darwin-arm64-0.1.0-rc.1.zip) | [Core](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0-rc.1/oneocr-core-darwin-arm64-0.1.0-rc.1.zip) |
| Linux x64 | [SDK](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0-rc.1/oneocr-sdk-linux-amd64-0.1.0-rc.1.zip) | [Core](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0-rc.1/oneocr-core-linux-amd64-0.1.0-rc.1.zip) |
| Linux ARM64 | [SDK](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0-rc.1/oneocr-sdk-linux-arm64-0.1.0-rc.1.zip) | [Core](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0-rc.1/oneocr-core-linux-arm64-0.1.0-rc.1.zip) |
| Android arm64-v8a / x86_64 | [AAR](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0-rc.1/oneocr-android-0.1.0-rc.1.aar) | [Core AAR](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0-rc.1/oneocr-android-core-0.1.0-rc.1.aar) |


完整桌面 SDK 包含 CLI、默认模型、ONNX Runtime、C/C++ 头文件、Go 源码、Go 离线依赖及本指南。精简包保留相同接入工具，省略模型和 ORT。桌面支持 Windows x64、macOS ARM64、Linux x64/ARM64；Linux 以 Ubuntu 22.04 为构建基线。Android 要求 API 26 及以上。

## 完整桌面 SDK

解压到任意目录，直接调用包内 CLI：

```sh
/path/to/sdk/bin/oneocr recognize image.png
```

Windows 使用 `C:\path\to\sdk\bin\oneocr.exe`。图片可以放在任意位置，传入对应路径即可。需要让自己的 Go 程序或后续 CLI 调用使用这些资源时，运行：

```sh
/path/to/sdk/bin/oneocr install --offline
```

安装结果保存在用户配置目录中。CLI 可以继续通过完整路径运行，也可以把 SDK 的 `bin` 目录加入 `PATH`。

## 精简 SDK

```sh
/path/to/core-sdk/bin/oneocr install
/path/to/core-sdk/bin/oneocr recognize image.png
```

安装命令优先复用本地兼容运行库，缺失时从同一 GitHub Release 下载固定版本的模型和运行库。重复运行会复用已校验的资源；识别调用不联网。

断网安装精简包时，把 `release-manifest.json`、`SHA256SUMS`、默认模型制品和当前平台运行库 ZIP 下载到同一目录，再运行：

```sh
/path/to/core-sdk/bin/oneocr install --source /path/to/release-files --offline
```

安装前检查版本、平台和 SHA-256；模型会话成功创建后才更新配置。宿主已有运行库会保留。

## 各语言接入

[Go](go.md) · [C/C++](native.md) · [Python](python.md) · [Android](android.md) · [复用已有运行库](runtime.md)

---

[oneocr-native](../../README.md) · [AGPL-3.0-only](../../LICENSE)

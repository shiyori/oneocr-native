# oneocr-native

[简体中文](README.md) | [English](README.en.md) | [日本語](README.ja.md)

离线识别中文、日文、韩文和英文，提供 Go、C/C++、Python 与 Android SDK。

## 下载

[GitHub Releases v0.1.0-rc.1](https://github.com/shiyori/oneocr-native/releases/tag/v0.1.0-rc.1)

| 平台 | 完整离线包 | 精简包 |
|---|---|---|
| Windows x64 | [SDK](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0-rc.1/oneocr-sdk-windows-amd64-0.1.0-rc.1.zip) | [Core](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0-rc.1/oneocr-core-windows-amd64-0.1.0-rc.1.zip) |
| macOS ARM64 | [SDK](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0-rc.1/oneocr-sdk-darwin-arm64-0.1.0-rc.1.zip) | [Core](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0-rc.1/oneocr-core-darwin-arm64-0.1.0-rc.1.zip) |
| Linux x64 | [SDK](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0-rc.1/oneocr-sdk-linux-amd64-0.1.0-rc.1.zip) | [Core](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0-rc.1/oneocr-core-linux-amd64-0.1.0-rc.1.zip) |
| Linux ARM64 | [SDK](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0-rc.1/oneocr-sdk-linux-arm64-0.1.0-rc.1.zip) | [Core](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0-rc.1/oneocr-core-linux-arm64-0.1.0-rc.1.zip) |
| Android arm64-v8a / x86_64 | [AAR](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0-rc.1/oneocr-android-0.1.0-rc.1.aar) | [Core AAR](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0-rc.1/oneocr-android-core-0.1.0-rc.1.aar) |

## 开始使用

完整 SDK 已包含模型和运行库。解压后直接运行包内 `bin/oneocr recognize image.png`；精简包先运行 `bin/oneocr install`。Windows 使用 `bin\oneocr.exe`。

Python 可在任意目录直接安装：

```sh
python -m pip install "https://github.com/shiyori/oneocr-native/releases/download/v0.1.0-rc.1/oneocr_native-0.1.0rc1-py3-none-any.whl"
python -m oneocr_native install
python -m oneocr_native recognize image.png
```

## 接入指南

[下载与安装](docs/zh-CN/installation.md) · [Go 接入](docs/zh-CN/go.md) · [C 与 C++ 接入](docs/zh-CN/native.md) · [Python 接入](docs/zh-CN/python.md) · [Android 接入](docs/zh-CN/android.md) · [已有 ONNX Runtime](docs/zh-CN/runtime.md)

---

[AGPL-3.0-only](LICENSE). 第三方依赖保留各自许可证。

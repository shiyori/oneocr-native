# oneocr-native

[简体中文](README.md) | [English](README.en.md) | [日本語](README.ja.md)

离线识别中文、日文、韩文和英文，提供 Go、C/C++、Python 与 Android 接口。

## 下载

[GitHub Releases v0.1.0](https://github.com/shiyori/oneocr-native/releases/tag/v0.1.0)

| | 完整包 | 精简包 |
|---|---|---|
| Android arm64-v8a / x86_64 | [AAR](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-android-0.1.0.aar) | [Core AAR](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-android-core-0.1.0.aar) |
| Linux x64 | [SDK](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-sdk-linux-amd64-0.1.0.zip) | [Core](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-core-linux-amd64-0.1.0.zip) |
| Linux ARM64 | [SDK](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-sdk-linux-arm64-0.1.0.zip) | [Core](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-core-linux-arm64-0.1.0.zip) |

Windows/macOS 使用 Go 模块、Go 命令或通用 Python wheel，依赖按需准备；不提供平台专属包。Linux 下载包是可选方式。

## Go 接入

代码接入使用 `go get github.com/shiyori/oneocr-native@v0.1.0`，详见 [Go 指南](docs/zh-CN/go.md)。命令行接入使用：

```sh
go install github.com/shiyori/oneocr-native/cmd/oneocr@v0.1.0
oneocr install
oneocr recognize image.png
```

## Python

```sh
python -m pip install "https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr_native-0.1.0-py3-none-any.whl"
python -m oneocr_native install
python -m oneocr_native recognize image.png
```

## 接入指南

[下载与安装](docs/zh-CN/installation.md) · [Go 接入](docs/zh-CN/go.md) · [C 与 C++ 接入](docs/zh-CN/native.md) · [Python 接入](docs/zh-CN/python.md) · [Android 接入](docs/zh-CN/android.md) · [已有 ONNX Runtime](docs/zh-CN/runtime.md)

---

[AGPL-3.0-only](LICENSE).

# oneocr-native

[简体中文](README.md) | [English](README.en.md) | [日本語](README.ja.md)

离线识别中文、日文、韩文和英文，提供 Go、C/C++、Python 与 Android 接口。

## 下载

[GitHub Releases · Latest](https://github.com/shiyori/oneocr-native/releases/latest)

| 平台 | 完整版（推荐） |
|---|---|
| Android arm64-v8a / x86_64 | [完整 AAR](https://github.com/shiyori/oneocr-native/releases/latest/download/oneocr-android.aar) |
| Linux x64 | [完整运行包](https://github.com/shiyori/oneocr-native/releases/latest/download/oneocr-linux-amd64.zip) |
| Linux ARM64 | [完整运行包](https://github.com/shiyori/oneocr-native/releases/latest/download/oneocr-linux-arm64.zip) |

推荐使用包含默认模型和运行库的完整包。Android Core AAR 仅供已有宿主 ORT 的进阶集成，见 [Android 指南](docs/zh-CN/android.md)。Windows/macOS 通过 Go 或 Python 准备完整依赖，不提供平台专属包。

## Go 接入

代码接入使用 `go get github.com/shiyori/oneocr-native@v0.1.1`，详见 [Go 指南](docs/zh-CN/go.md)。命令行接入使用：

```sh
go install github.com/shiyori/oneocr-native/cmd/oneocr@v0.1.1
oneocr install
oneocr recognize image.png
```

## Python

```sh
python -m pip install "https://github.com/shiyori/oneocr-native/releases/download/v0.1.1/oneocr_native-0.1.1-py3-none-any.whl"
python -m oneocr_native install
python -m oneocr_native recognize image.png
```

## 接入指南

[下载与安装](docs/zh-CN/installation.md) · [Go 接入](docs/zh-CN/go.md) · [C 与 C++ 接入](docs/zh-CN/native.md) · [Python 接入](docs/zh-CN/python.md) · [Android 接入](docs/zh-CN/android.md) · [已有 ONNX Runtime](docs/zh-CN/runtime.md)

---

[AGPL-3.0-only](LICENSE).

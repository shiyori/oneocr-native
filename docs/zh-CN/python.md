# Python 接入

[简体中文](../zh-CN/python.md) | [English](../en/python.md) | [日本語](../ja/python.md)

支持 Python 3.11–3.13。直接安装 GitHub Release 中的 wheel，再准备模型和运行库。**以下命令可在任意目录运行**，无需克隆仓库。

```sh
python -m pip install "https://github.com/shiyori/oneocr-native/releases/latest/download/oneocr_native-0.1.0-py3-none-any.whl"
python -m oneocr_native install
python -m oneocr_native recognize image.png
```

如果系统使用 `python3` 或 `py -3.13` 选择解释器，请在安装和运行时保持一致。

## 在自己的应用中使用

```python
from oneocr_native import OneOcrEngine

with OneOcrEngine() as engine:
    result = engine.recognize("image.png")
    print(result.text)
```

输入可以是文件路径或 `PIL.Image.Image`。自动处理 JPEG EXIF 方向与透明度；连续调用时复用同一个 Engine。

```python
from oneocr_native import EngineConfig, OneOcrEngine

with OneOcrEngine(EngineConfig(threads=2)) as engine:
    regions = engine.detect("page.png")
    line = engine.recognize_line("line.png", script="CJK")
```

只保留 `OneOcrEngine()` 一个创建入口。`EngineConfig` 包含线程数、图片尺寸、缓存和模型路径等可选项，普通使用无需配置。Python 包不依赖 Go SDK。

## Linux 可选离线安装

Linux 可选下载完整包，其中包含默认模型以及 Python 3.11、3.12、3.13 的依赖 wheel。Windows/macOS 使用上文的通用 wheel，依赖按需下载：

- [Linux x64](https://github.com/shiyori/oneocr-native/releases/latest/download/oneocr-python-linux-amd64-0.1.0.zip)
- [Linux ARM64](https://github.com/shiyori/oneocr-native/releases/latest/download/oneocr-python-linux-arm64-0.1.0.zip)

解压后，在任意目录通过路径运行包内安装脚本：

```sh
python /path/to/oneocr-python/installation.py
python -m oneocr_native recognize image.png
```

`installation.py` 自动选择当前 Python 版本所需的 wheel，并执行离线安装。已有兼容的 ORT CPU/GPU 包会保留；新环境使用随包 CPU 运行库。无需选择模型或指定运行库路径。

## 已有 Python 环境

wheel 不强制依赖 CPU 版 `onnxruntime`。`python -m oneocr_native install` 会复用已有兼容 CPU/GPU 发行包；没有时安装 ORT 1.29.0。OneOCR 的会话始终使用 CPU provider。详见[运行库复用](runtime.md)。

CLI 另外提供 `python -m oneocr_native detect page.png --format json` 和 `python -m oneocr_native recognize-line line.png`。需要可序列化的结果时使用 `result.to_dict()`。

---

[oneocr-native](../../README.md) · [AGPL-3.0-only](../../LICENSE)

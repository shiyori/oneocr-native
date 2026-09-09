# Python 接入

[简体中文](../zh-CN/python.md) | [English](../en/python.md) | [日本語](../ja/python.md)

支持 Python 3.11–3.13。直接安装 GitHub Release 中的 wheel，再准备模型和运行库。**以下命令可在任意目录运行**，无需克隆仓库。

```sh
python -m pip install "https://github.com/shiyori/oneocr-native/releases/download/v0.1.3/oneocr_native-0.1.3-py3-none-any.whl"
python -m oneocr_native install
python -m oneocr_native recognize image.png
python -m oneocr_native recognize --format json image.png
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

## 进阶：离线依赖

Python 在所有平台统一使用通用 wheel，推荐先运行 `python -m oneocr_native install` 准备完整依赖。Release 不再提供 Linux 专用 Python 离线包；只需命令行时可直接使用 [Linux 完整运行包](installation.md)。

离线代码集成可在同平台、同 Python 版本的联网环境用 `pip download` 准备 wheel 依赖，并从 Release 或源码仓库准备默认模型。已有资源可通过 `python -m oneocr_native install --source /path/to/resources --offline` 安装；资源目录中的依赖放在 `wheelhouse/` 或 `wheelhouse/3.13/`，默认模型放在 `models/oneocr-cjk-en.ocrpack`。

## 已有 Python 环境

wheel 不强制依赖 CPU 版 `onnxruntime`。`python -m oneocr_native install` 会复用已有兼容 CPU/GPU 发行包；没有时安装 ORT 1.29.0。OneOCR 的会话始终使用 CPU provider。详见[运行库复用](runtime.md)。

CLI 另外提供 `python -m oneocr_native detect page.png --format json` 和 `python -m oneocr_native recognize-line line.png`。需要可序列化的结果时使用 `result.to_dict()`。

---

[oneocr-native](../../README.md) · [AGPL-3.0-only](../../LICENSE)

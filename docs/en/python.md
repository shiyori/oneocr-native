# Python

[简体中文](../zh-CN/python.md) | [English](../en/python.md) | [日本語](../ja/python.md)

Python 3.11–3.13 is supported. Install the wheel directly from GitHub Releases, then prepare the model and runtime. **Run these commands from any directory**; no checkout is needed.

```sh
python -m pip install "https://github.com/shiyori/oneocr-native/releases/download/v0.1.0-rc.1/oneocr_native-0.1.0rc1-py3-none-any.whl"
python -m oneocr_native install
python -m oneocr_native recognize image.png
```

If `python` selects a different interpreter on your system, use `python3` or the appropriate `py -3.13` command consistently. The same interpreter must perform installation and recognition.

## In your application

```python
from oneocr_native import OneOcrEngine

with OneOcrEngine() as engine:
    result = engine.recognize("image.png")
    print(result.text)
```

The input may be a path or a `PIL.Image.Image`. JPEG EXIF orientation and transparency are handled automatically. Reuse the engine between calls.

```python
from oneocr_native import EngineConfig, OneOcrEngine

with OneOcrEngine(EngineConfig(threads=2)) as engine:
    regions = engine.detect("page.png")
    line = engine.recognize_line("line.png", script="CJK")
```

`OneOcrEngine()` is the single creation entry. `EngineConfig` contains optional thread, image-size, cache and model settings; normal use needs none of them. The Python package does not load the Go SDK.

## Complete offline installation

Download the bundle for your system. Each includes the model and dependency wheels for Python 3.11, 3.12 and 3.13:

- [Windows x64](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0-rc.1/oneocr-python-windows-amd64-0.1.0-rc.1.zip)
- [macOS ARM64](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0-rc.1/oneocr-python-darwin-arm64-0.1.0-rc.1.zip)
- [Linux x64](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0-rc.1/oneocr-python-linux-amd64-0.1.0-rc.1.zip)
- [Linux ARM64](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0-rc.1/oneocr-python-linux-arm64-0.1.0-rc.1.zip)

Extract it, then run its installer by path from any directory:

```sh
python /path/to/oneocr-python/installation.py
python -m oneocr_native recognize image.png
```

`installation.py` selects the current Python version's wheels and performs an offline installation. An existing compatible ORT CPU/GPU package is kept. A fresh environment receives the included CPU runtime. No model selection or runtime path is required.

## Existing environments

The wheel does not force the CPU `onnxruntime` distribution. `python -m oneocr_native install` reuses a compatible CPU/GPU distribution; when none exists, it installs ORT 1.29.0. OneOCR sessions always use the CPU provider. [Runtime details](runtime.md).

`python -m oneocr_native detect page.png --format json` and `python -m oneocr_native recognize-line line.png` expose the other operations. Use `result.to_dict()` for JSON-compatible Python results.

---

[oneocr-native](../../README.en.md) · [AGPL-3.0-only](../../LICENSE)

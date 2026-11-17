# Python

[简体中文](../zh-CN/python.md) | [English](../en/python.md) | [日本語](../ja/python.md)

Python 3.11–3.13 is supported. Install the wheel directly from GitHub Releases, then prepare the model and runtime. **Run these commands from any directory**; no checkout is needed.

```sh
python -m pip install "https://github.com/shiyori/oneocr-native/releases/download/v0.2.0/oneocr_native-0.2.0-py3-none-any.whl"
python -m oneocr_native install
python -m oneocr_native recognize image.png
python -m oneocr_native recognize --format json image.png
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

Set `EngineConfig(max_inflight=2)` through 4 for bounded concurrent work. `recognize_batch`, `detect_batch` and `recognize_line_batch` preserve input order and return an item with either `result` or a structured `error`. `recognize` accepts `reading_order="auto"`, `"ltr"`, `"rtl"` or `"none"`, plus an optional `min_line_confidence` filter.

`OneOcrEngine()` is the single creation entry. `EngineConfig` contains optional thread, image-size, cache and model settings; normal use needs none of them. The Python package does not load the Go SDK.

## Advanced: offline dependencies

Python uses the universal wheel on every platform. Run `python -m oneocr_native install` to prepare complete dependencies. Releases no longer provide a Linux-specific Python offline bundle; for command-line use, choose the [complete Linux runtime package](installation.md).

Offline code integration can use `pip download` on a connected system with the same platform and Python version to prepare dependency wheels, plus the default model from Releases or the source repository. Install prepared resources with `python -m oneocr_native install --source /path/to/resources --offline`; place wheels under `wheelhouse/` or `wheelhouse/3.13/` and the model at `models/oneocr-cjk-en.ocrpack`.

## Existing environments

The wheel does not force the CPU `onnxruntime` distribution. `python -m oneocr_native install` reuses a compatible CPU/GPU distribution; when none exists, it installs ORT 1.29.0. OneOCR sessions always use the CPU provider. [Runtime details](runtime.md).

`python -m oneocr_native detect page.png --format json` and `python -m oneocr_native recognize-line line.png` expose the other operations. Use `result.to_dict()` for JSON-compatible Python results.

---

[oneocr-native](../../README.en.md) · [AGPL-3.0-only](../../LICENSE)

# Python SDK

Offline OCR for Chinese, Japanese, Korean, English and digits. Python 3.11–3.13; the SDK runs its own pipeline through ONNX Runtime and does not use the Go shared library.

## Install

From the repository root:

```bash
python -m pip install ./python
oneocr-native recognize image.png
oneocr-native recognize --format json image.png
```

Or install a built wheel:

```bash
python -m pip install oneocr_native-0.1.0-py3-none-any.whl
```

Keep the default `oneocr-cjk-en.ocrpack` in your working directory's `models/`. It is already there in the repository; pip installs the runtime dependencies, while the model remains a separate file.

## Recognize images

```python
from oneocr_native import OneOcrEngine

with OneOcrEngine() as engine:
    result = engine.recognize("image.png")
    print(result.text)
    for line in result.lines:
        print(line.text, line.quad)
```

Pillow images work too:

```python
from PIL import Image
from oneocr_native import OneOcrEngine

with OneOcrEngine(threads=2, max_side=1600) as engine:
    with Image.open("image.png") as image:
        result = engine.recognize(image)
    print(result.to_dict())
```

The default model is found by its fixed filename in the working directory or `models/`, beside the Python executable or under the parent SDK directory, then through the configuration written by `oneocr install`. It never chooses another `.ocrpack`. `ONEOCR_MODEL` can set a deployment path; `ONEOCR_HOME` selects the installed configuration directory.

## Detect regions or recognize one line

```python
from oneocr_native import OneOcrEngine

with OneOcrEngine() as engine:
    regions = engine.detect("page.png")
    for region in regions.regions:
        print(region.quad)

    line = engine.recognize_line("cropped-line.png")
    print(line.text)
```

`detect` returns regions without recognizing their text. `recognize_line` expects a cropped horizontal text line and chooses its script automatically. All result objects support `to_dict()`. See [stage APIs](https://github.com/shiyori/oneocr-native/blob/main/docs/STAGES.md) for detailed types.

## Command line

```bash
oneocr-native recognize --threads 2 --max-side 1600 image.png
oneocr-native recognize --format json --output result.json image.png
oneocr-native detect page.png
oneocr-native recognize-line line.png
```

`recognize` and `recognize-line` print text unless `--format json` is used. `detect` prints JSON. `--output` writes to a file instead of standard output. CLI errors go to standard error and return a nonzero exit status.

Use one engine for multiple images. The context manager closes sessions and the model file; `close()` can also be called explicitly and is idempotent. Calls and closure are serialized on the same engine. Application code can catch `OneOcrError` for SDK errors and `OSError` for file access failures.

Source uses **AGPL-3.0-only**; see LICENSE and THIRD_PARTY_NOTICES.md in this distribution. Model resources retain their own rights. [Project repository](https://github.com/shiyori/oneocr-native).

This unofficial implementation is shared for learning and discussion and provided without warranty.

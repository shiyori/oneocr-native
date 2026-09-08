# Independent Python SDK

Python 3.11–3.13. This SDK executes preprocessing, detection, recognition scheduling and decoding itself using NumPy/OpenCV/Pillow and the ONNX Runtime Python package. It does not load this project's Go shared library.

```bash
python -m pip install ./python
oneocr-native recognize --model models/oneocr-cjk-en.ocrpack testdata/CJK.png
```

From a downloaded wheel: `python -m pip install oneocr_native-0.1.0-py3-none-any.whl`.

```python
from oneocr_native import OneOcrEngine

with OneOcrEngine("models/oneocr-cjk-en.ocrpack") as engine:
    result = engine.recognize("image.png")  # also accepts PIL.Image.Image
    print(result.text)
    print(result.to_dict())
```

`OneOcrEngine.from_package(path)` explicitly opens a verified ONEOCRPK v1 file. Models load individually from bytes without extraction. `from_bundle(directory)` and the constructor's original `.onemodel` support remain available. Original OneModel conversion uses a cross-platform file lock and a content-addressed disk cache.

`close()` is idempotent and releases the engine's session references and package file. Recognition and context entry after close raise `OneOcrError`. Use one reusable engine; recognition and close serialize on the same lock. Automatic selection skips scripts absent from the package and returns warnings; forcing an unavailable script raises `ValueError`.

macOS uses system ICU for inverse RTL. Other platforms use a portable approximation preserving combining marks, Latin and numeric runs plus common mirrored punctuation. Complex mixed directions, controls and isolates are not fully reconstructed. Confidence/word boxes remain null. This is an unofficial experimental implementation, not a claim of full equivalence with the original DLL.

MIT covers project source only; see LICENSE and THIRD_PARTY_NOTICES.md in this distribution. Model resources retain their original rights. Full project: [oneocr-native](https://github.com/shiyori/oneocr-native).

## 开发期模型适配

`oneocr-native adapt --bundle /path/to/source-bundle --directory /path/to/new-candidate --backend coreml` 可生成独立的实验 ONNX 模型集，后端可选 `cpu/coreml/cuda/directml`。Go CLI 的 `oneocr unpack` 可从 `.ocrpack` 生成源目录。转换不覆盖原资源，输出带源模型和候选模型摘要的 `adaptation.json`。

`--compact-output` 添加字符掩码、ArgMax 和完整异常值检查，保留 LogSoftmax；该紧凑接口由 Go/C/C++ SDK 使用。`--quantization grid` 保留激活舍入与裁剪，`relaxed` 是只保留裁剪的更激进实验。浮点累加和标准 LSTM 转换都需要目标设备数值/文字回归。Python 原有 `OneOcrEngine` 仍为独立 CPU 路径，不依赖 Go 动态库。

Independent detection and cropped-line recognition: [API guide](https://github.com/shiyori/oneocr-native/blob/main/docs/STAGES.md). Go, CLI, C/C++ and Python expose separate entry points.

```python
regions = engine.detect("page.png")
line = engine.recognize_line("rectified-line.png", script="CJK")
print(line.text)
```

`detect` returns quads, uncalibrated detector scores and vertical flags, without running recognition. `recognize_line` expects an already cropped horizontal line; omitted script auto-classifies and corrects 180-degree rotation, while explicit script assumes upright input. Both return dataclasses with `to_dict()`.

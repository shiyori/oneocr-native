# oneocr-native · Python

[简体中文](README.md) | [English](README.en.md) | [日本語](README.ja.md)

Python は任意のディレクトリからインストールできます。

```sh
python -m pip install "https://github.com/shiyori/oneocr-native/releases/download/v0.1.3/oneocr_native-0.1.3-py3-none-any.whl"
python -m oneocr_native install
python -m oneocr_native recognize image.png
python -m oneocr_native recognize --format json image.png
```

```python
from oneocr_native import OneOcrEngine

with OneOcrEngine() as engine:
    result = engine.recognize("image.png")
    print(result.text)
```

[Python](https://github.com/shiyori/oneocr-native/blob/v0.1.3/docs/ja/python.md) · [既存の ONNX Runtime](https://github.com/shiyori/oneocr-native/blob/v0.1.3/docs/ja/runtime.md)

[AGPL-3.0-only](LICENSE)

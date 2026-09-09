# oneocr-native · Python

[简体中文](README.md) | [English](README.en.md) | [日本語](README.ja.md)

Python 可在任意目录直接安装：

```sh
python -m pip install "https://github.com/shiyori/oneocr-native/releases/latest/download/oneocr_native-0.1.0-py3-none-any.whl"
python -m oneocr_native install
python -m oneocr_native recognize image.png
```

```python
from oneocr_native import OneOcrEngine

with OneOcrEngine() as engine:
    result = engine.recognize("image.png")
    print(result.text)
```

[Python 接入](https://github.com/shiyori/oneocr-native/blob/v0.1.0/docs/zh-CN/python.md) · [已有 ONNX Runtime](https://github.com/shiyori/oneocr-native/blob/v0.1.0/docs/zh-CN/runtime.md)

[AGPL-3.0-only](LICENSE)

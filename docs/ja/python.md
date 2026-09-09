# Python

[简体中文](../zh-CN/python.md) | [English](../en/python.md) | [日本語](../ja/python.md)

Python 3.11–3.13 に対応します。GitHub Release の wheel を直接インストールしてからモデルとランタイムを準備します。**どのディレクトリからでも実行できます**。リポジトリのクローンは不要です。

```sh
python -m pip install "https://github.com/shiyori/oneocr-native/releases/download/v0.1.2/oneocr_native-0.1.2-py3-none-any.whl"
python -m oneocr_native install
python -m oneocr_native recognize image.png
python -m oneocr_native recognize --format json image.png
```

環境に応じて `python3` または `py -3.13` を使う場合は、インストールと認識で同じインタープリターを使用してください。

## アプリから使用する

```python
from oneocr_native import OneOcrEngine

with OneOcrEngine() as engine:
    result = engine.recognize("image.png")
    print(result.text)
```

入力はパスまたは `PIL.Image.Image` です。JPEG EXIF の向きと透明部分を自動処理します。連続した呼び出しでは同じ Engine を再利用します。

```python
from oneocr_native import EngineConfig, OneOcrEngine

with OneOcrEngine(EngineConfig(threads=2)) as engine:
    regions = engine.detect("page.png")
    line = engine.recognize_line("line.png", script="CJK")
```

生成入口は `OneOcrEngine()` のみです。`EngineConfig` にはスレッド数、画像サイズ、キャッシュ、モデル設定がありますが、通常は指定不要です。Python パッケージは Go SDK を使用しません。

## 上級設定：オフライン依存関係

Python はすべてのプラットフォームで共通 wheel を使います。`python -m oneocr_native install` で必要な依存関係を揃える方法を推奨します。Linux 専用の Python オフラインパッケージは公開しません。コマンドだけを使う場合は [Linux 完全実行パッケージ](installation.md)を選んでください。

オフラインのコード利用では、同じ OS・Python バージョンのオンライン環境で `pip download` を使って依存 wheel を準備し、Release またはソースリポジトリから既定モデルを取得できます。`python -m oneocr_native install --source /path/to/resources --offline` で導入します。依存 wheel は `wheelhouse/` または `wheelhouse/3.13/`、モデルは `models/oneocr-cjk-en.ocrpack` に配置してください。

## 既存の Python 環境

wheel は CPU 版 `onnxruntime` を必須依存にしません。`python -m oneocr_native install` は既存の互換 CPU/GPU パッケージを再利用し、存在しない場合に ORT 1.29.0 をインストールします。OneOCR のセッションは CPU provider を使用します。[ランタイムの詳細](runtime.md)。

CLI では `python -m oneocr_native detect page.png --format json` と `python -m oneocr_native recognize-line line.png` も使用できます。JSON に変換可能な結果は `result.to_dict()` で取得できます。

---

[oneocr-native](../../README.ja.md) · [AGPL-3.0-only](../../LICENSE)

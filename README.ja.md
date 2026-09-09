# oneocr-native

[简体中文](README.md) | [English](README.en.md) | [日本語](README.ja.md)

中国語、日本語、韓国語、英語をオフラインで認識する Go、C/C++、Python、Android インターフェースです。

## ダウンロード

[GitHub Releases · Latest](https://github.com/shiyori/oneocr-native/releases/latest)

| プラットフォーム | 完全版（推奨） |
|---|---|
| Android arm64-v8a / x86_64 | [完全版 AAR](https://github.com/shiyori/oneocr-native/releases/latest/download/oneocr-android.aar) |
| Linux x64 | [完全実行パッケージ](https://github.com/shiyori/oneocr-native/releases/latest/download/oneocr-linux-amd64.zip) |
| Linux ARM64 | [完全実行パッケージ](https://github.com/shiyori/oneocr-native/releases/latest/download/oneocr-linux-arm64.zip) |

既定モデルとランタイムを含む完全版を推奨します。Core AAR は既存のホスト ORT を使う上級者向けです。[Android ガイド](docs/ja/android.md)を参照してください。Windows/macOS は Go または Python で必要な依存関係を準備し、プラットフォーム専用パッケージは公開しません。

## Go の導入

コードからの利用は `go get github.com/shiyori/oneocr-native@v0.1.2` を使用します。[Go ガイド](docs/ja/go.md)を参照してください。CLI は次の方法で導入します。

```sh
go install github.com/shiyori/oneocr-native/cmd/oneocr@v0.1.2
oneocr install
oneocr recognize image.png
oneocr recognize --format json image.png
```

## Python

```sh
python -m pip install "https://github.com/shiyori/oneocr-native/releases/download/v0.1.2/oneocr_native-0.1.2-py3-none-any.whl"
python -m oneocr_native install
python -m oneocr_native recognize image.png
python -m oneocr_native recognize --format json image.png
```

## テスト結果

`testdata` の4枚に対する OneOCR v0.1.2 の実際の出力です。認識信頼度 ≥ **0.80**、検出スコア ≥ **0.70** の結果のみ表示します。左は元画像に枠を描画し、右は元画像の同じ位置に青色の認識文字を重ねています。読みやすいよう文字部分に淡い背景を付けています。画像をクリックすると高解像度で確認できます。

左右で座標と切り出し範囲は共通です。周囲の余白のみ同じ範囲で除去し、認識文字は手動修正していません。信頼度は未校正のモデルスコアです。

### 中日韓英の混在

空でない認識結果 6 件中 6 件を表示.

| 元画像 + 検出枠 | 元画像 + 認識文字（青色） |
|---|---|
| [![中日韓英の混在 — 元画像 + 検出枠](docs/assets/ocr-results/mixed-boxes.webp)](docs/assets/ocr-results/mixed-boxes.webp) | [![中日韓英の混在 — 元画像 + 認識文字（青色）](docs/assets/ocr-results/mixed-text.webp)](docs/assets/ocr-results/mixed-text.webp) |

### 書籍の実写

空でない認識結果 48 件中 46 件を表示.

| 元画像 + 検出枠 | 元画像 + 認識文字（青色） |
|---|---|
| [![書籍の実写 — 元画像 + 検出枠](docs/assets/ocr-results/book-boxes.webp)](docs/assets/ocr-results/book-boxes.webp) | [![書籍の実写 — 元画像 + 認識文字（青色）](docs/assets/ocr-results/book-text.webp)](docs/assets/ocr-results/book-text.webp) |

### 数式を含む文書

空でない認識結果 94 件中 76 件を表示.

| 元画像 + 検出枠 | 元画像 + 認識文字（青色） |
|---|---|
| [![数式を含む文書 — 元画像 + 検出枠](docs/assets/ocr-results/formula-boxes.webp)](docs/assets/ocr-results/formula-boxes.webp) | [![数式を含む文書 — 元画像 + 認識文字（青色）](docs/assets/ocr-results/formula-text.webp)](docs/assets/ocr-results/formula-text.webp) |

### 中国語の表

空でない認識結果 89 件中 87 件を表示.

| 元画像 + 検出枠 | 元画像 + 認識文字（青色） |
|---|---|
| [![中国語の表 — 元画像 + 検出枠](docs/assets/ocr-results/table-boxes.webp)](docs/assets/ocr-results/table-boxes.webp) | [![中国語の表 — 元画像 + 認識文字（青色）](docs/assets/ocr-results/table-text.webp)](docs/assets/ocr-results/table-text.webp) |

[生成記録・再現方法](docs/assets/ocr-results/README.md) · [画像の出典](testdata/paddleocr/README.md) · [第三者ライセンス](testdata/paddleocr/UPSTREAM_LICENSE)

## 導入ガイド

[インストール](docs/ja/installation.md) · [Go](docs/ja/go.md) · [C と C++](docs/ja/native.md) · [Python](docs/ja/python.md) · [Android](docs/ja/android.md) · [既存 ONNX Runtime](docs/ja/runtime.md)

---

[AGPL-3.0-only](LICENSE).

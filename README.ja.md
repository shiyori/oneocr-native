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

## 導入ガイド

[インストール](docs/ja/installation.md) · [Go](docs/ja/go.md) · [C と C++](docs/ja/native.md) · [Python](docs/ja/python.md) · [Android](docs/ja/android.md) · [既存 ONNX Runtime](docs/ja/runtime.md)

---

[AGPL-3.0-only](LICENSE).

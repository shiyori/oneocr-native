# oneocr-native

[简体中文](README.md) | [English](README.en.md) | [日本語](README.ja.md)

中国語、日本語、韓国語、英語をオフラインで認識する Go、C/C++、Python、Android インターフェースです。

## ダウンロード

[GitHub Releases · Latest](https://github.com/shiyori/oneocr-native/releases/latest)

| | 完全版 | コア版 |
|---|---|---|
| Android arm64-v8a / x86_64 | [AAR](https://github.com/shiyori/oneocr-native/releases/latest/download/oneocr-android-0.1.0.aar) | [Core AAR](https://github.com/shiyori/oneocr-native/releases/latest/download/oneocr-android-core-0.1.0.aar) |
| Linux x64 | [SDK](https://github.com/shiyori/oneocr-native/releases/latest/download/oneocr-sdk-linux-amd64-0.1.0.zip) | [Core](https://github.com/shiyori/oneocr-native/releases/latest/download/oneocr-core-linux-amd64-0.1.0.zip) |
| Linux ARM64 | [SDK](https://github.com/shiyori/oneocr-native/releases/latest/download/oneocr-sdk-linux-arm64-0.1.0.zip) | [Core](https://github.com/shiyori/oneocr-native/releases/latest/download/oneocr-core-linux-arm64-0.1.0.zip) |

Windows/macOS では Go モジュール、Go コマンド、汎用 Python wheel を利用し、依存関係を必要に応じて準備します。OS 別の配布パッケージはありません。Linux のダウンロードは任意です。

## Go の導入

コードからの利用は `go get github.com/shiyori/oneocr-native@v0.1.0` を使用します。[Go ガイド](docs/ja/go.md)を参照してください。CLI は次の方法で導入します。

```sh
go install github.com/shiyori/oneocr-native/cmd/oneocr@v0.1.0
oneocr install
oneocr recognize image.png
```

## Python

```sh
python -m pip install "https://github.com/shiyori/oneocr-native/releases/latest/download/oneocr_native-0.1.0-py3-none-any.whl"
python -m oneocr_native install
python -m oneocr_native recognize image.png
```

## 導入ガイド

[インストール](docs/ja/installation.md) · [Go](docs/ja/go.md) · [C と C++](docs/ja/native.md) · [Python](docs/ja/python.md) · [Android](docs/ja/android.md) · [既存 ONNX Runtime](docs/ja/runtime.md)

---

[AGPL-3.0-only](LICENSE).

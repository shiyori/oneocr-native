# oneocr-native

[简体中文](README.md) | [English](README.en.md) | [日本語](README.ja.md)

中国語、日本語、韓国語、英語をオフラインで認識する Go、C/C++、Python、Android SDK。

## ダウンロード

[GitHub Releases v0.1.0](https://github.com/shiyori/oneocr-native/releases/tag/v0.1.0)

| プラットフォーム | 完全オフライン版 | コア版 |
|---|---|---|
| Windows x64 | [SDK](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-sdk-windows-amd64-0.1.0.zip) | [Core](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-core-windows-amd64-0.1.0.zip) |
| macOS ARM64 | [SDK](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-sdk-darwin-arm64-0.1.0.zip) | [Core](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-core-darwin-arm64-0.1.0.zip) |
| Linux x64 | [SDK](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-sdk-linux-amd64-0.1.0.zip) | [Core](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-core-linux-amd64-0.1.0.zip) |
| Linux ARM64 | [SDK](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-sdk-linux-arm64-0.1.0.zip) | [Core](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-core-linux-arm64-0.1.0.zip) |
| Android arm64-v8a / x86_64 | [AAR](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-android-0.1.0.aar) | [Core AAR](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-android-core-0.1.0.aar) |

## 使い始める

完全版にはモデルとランタイムが含まれます。展開後に `bin/oneocr recognize image.png` を実行します。コア版は先に `bin/oneocr install` を実行します。Windows では `bin\oneocr.exe` を使用します。

Python は任意のディレクトリからインストールできます。

```sh
python -m pip install "https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr_native-0.1.0-py3-none-any.whl"
python -m oneocr_native install
python -m oneocr_native recognize image.png
```

## 導入ガイド

[ダウンロードとインストール](docs/ja/installation.md) · [Go](docs/ja/go.md) · [C と C++](docs/ja/native.md) · [Python](docs/ja/python.md) · [Android](docs/ja/android.md) · [既存の ONNX Runtime](docs/ja/runtime.md)

---

[AGPL-3.0-only](LICENSE). サードパーティの依存関係には各ライセンスが適用されます。

# ダウンロードとインストール

[简体中文](../zh-CN/installation.md) | [English](../en/installation.md) | [日本語](../ja/installation.md)

Go と Python は言語のパッケージマネージャーを使い、Android は AAR を取得します。Windows/macOS 専用の制品は公開しません。Linux のビルド済みパッケージは任意です。通常の導入に runtime パスは不要です。

[GitHub Releases · Latest](https://github.com/shiyori/oneocr-native/releases/latest)

## Go

コードからの利用では、自分のプロジェクトで `go get github.com/shiyori/oneocr-native@v0.1.0` を実行し、`oneocr.Install` で準備してから `oneocr.Open` を呼びます。[Go の導入](go.md)を参照してください。CLI の導入：

```sh
go install github.com/shiyori/oneocr-native/cmd/oneocr@v0.1.0
oneocr install
oneocr recognize image.png
```

## Python

汎用 wheel は Python 3.11–3.13 に対応し、任意のディレクトリから導入できます。

```sh
python -m pip install "https://github.com/shiyori/oneocr-native/releases/latest/download/oneocr_native-0.1.0-py3-none-any.whl"
python -m oneocr_native install
```

## Android

[AAR](https://github.com/shiyori/oneocr-native/releases/latest/download/oneocr-android-0.1.0.aar) · [Core AAR](https://github.com/shiyori/oneocr-native/releases/latest/download/oneocr-android-core-0.1.0.aar)

完全版 AAR は既定モデルと runtime を含みます。既存のホスト ORT があるアプリは Core AAR を使います。両方とも arm64-v8a / x86_64 を含み、Android API 26 以上が必要です。[Android の導入](android.md)を参照してください。

## 任意の Linux ダウンロード

| | SDK | Core |
|---|---|---|
| Linux x64 | [SDK](https://github.com/shiyori/oneocr-native/releases/latest/download/oneocr-sdk-linux-amd64-0.1.0.zip) | [Core](https://github.com/shiyori/oneocr-native/releases/latest/download/oneocr-core-linux-amd64-0.1.0.zip) |
| Linux ARM64 | [SDK](https://github.com/shiyori/oneocr-native/releases/latest/download/oneocr-sdk-linux-arm64-0.1.0.zip) | [Core](https://github.com/shiyori/oneocr-native/releases/latest/download/oneocr-core-linux-arm64-0.1.0.zip) |

完全版には CLI、C/C++ ヘッダー、共有ライブラリー、既定モデル、ORT が含まれます。Core 版にはインターフェースと CLI のみが含まれ、モデルと ORT は含まれません。Linux のビルド環境は Ubuntu 22.04 です。Go の導入にこれらのパッケージは不要です。

完全版は展開後に実行できます。Core 版は先に準備します。

```sh
/path/to/sdk/bin/oneocr recognize image.png
/path/to/core-sdk/bin/oneocr install
/path/to/core-sdk/bin/oneocr recognize image.png
```

## 依存関係とオフライン準備

互換 runtime があれば再利用します。不足する場合、Windows/macOS は ONNX Runtime の公式 Release から固定版を取得し、SHA-256 を検証します。Linux は本プロジェクトの runtime ZIP を利用できます。Python は不足する ORT を pip で上流から導入し、互換 CPU/GPU 配布版を維持します。認識中にファイルを取得することはありません。

オフライン準備では、この Release の `release-manifest.json`、`SHA256SUMS`、既定モデルを同じディレクトリに保存します。Windows/macOS は公式の `onnxruntime-win-x64-1.29.0.zip` / `onnxruntime-osx-arm64-1.29.0.tgz`、Linux は本プロジェクトの runtime ZIP も保存します。インストーラーが展開するため、runtime パスの指定は不要です。

```sh
oneocr install --source /path/to/resources --offline
```

Windows は [ONNX Runtime が要求する Visual C++ ランタイム](https://onnxruntime.ai/docs/install/#requirements)が必要です。Go は C コンパイラーも必要です。モデルセッションの検証後にのみ設定を更新します。

## 言語ガイド

[Go](go.md) · [C/C++](native.md) · [Python](python.md) · [Android](android.md)

---

[oneocr-native](../../README.ja.md) · [AGPL-3.0-only](../../LICENSE)

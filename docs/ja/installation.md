# ダウンロードとインストール

[简体中文](../zh-CN/installation.md) | [English](../en/installation.md) | [日本語](../ja/installation.md)

完全版での導入を推奨します。Android 完全版 AAR と Linux 完全パッケージには既定モデルと ONNX Runtime が含まれます。コードからの利用は Go/Python のインストール入口で必要な依存関係を揃え、Release、ソースリポジトリ、公式の依存関係リポジトリからリソースを取得できます。

[GitHub Releases · Latest](https://github.com/shiyori/oneocr-native/releases/latest)

## 完全版（推奨）

| | 完全パッケージ |
|---|---|
| Android arm64-v8a / x86_64 | [AAR](https://github.com/shiyori/oneocr-native/releases/latest/download/oneocr-android.aar) |
| Linux x64 | [完全パッケージ](https://github.com/shiyori/oneocr-native/releases/latest/download/oneocr-linux-amd64.zip) |
| Linux ARM64 | [完全パッケージ](https://github.com/shiyori/oneocr-native/releases/latest/download/oneocr-linux-arm64.zip) |

Linux 完全パッケージは展開後に直接実行できます。Go のインストールやモデル・runtime パスの指定は不要です。`bin/oneocr`、`models/`、`lib/`、ライセンス、利用文書を含み、Ubuntu 22.04 および互換システムを対象とします。

```sh
/path/to/oneocr-linux-amd64/bin/oneocr recognize image.png
/path/to/oneocr-linux-amd64/bin/oneocr recognize --format json image.png
```

Android 完全版 AAR は arm64-v8a / x86_64 を含み、API 26 以上に対応します。アプリに追加して [Android ガイド](android.md)に従って利用してください。Core は既にホスト ORT を管理しているアプリ向けの上級設定です。

## コードからの利用

### Go

プロジェクトで `go get github.com/shiyori/oneocr-native@v0.1.3` を実行し、`oneocr.Install` で依存関係を揃えてから `oneocr.Open` を呼びます。コマンドラインでは：

```sh
go install github.com/shiyori/oneocr-native/cmd/oneocr@v0.1.3
oneocr install
oneocr recognize image.png
oneocr recognize --format json image.png
```

### Python

Python 3.11–3.13 は共通 wheel を利用します。準備コマンドがモデルと不足しているランタイムを揃えます。

```sh
python -m pip install "https://github.com/shiyori/oneocr-native/releases/download/v0.1.3/oneocr_native-0.1.3-py3-none-any.whl"
python -m oneocr_native install
python -m oneocr_native recognize image.png
python -m oneocr_native recognize --format json image.png
```

[Go](go.md) · [C/C++](native.md) · [Python](python.md) · [Android](android.md)

## 上級設定：依存関係とオフライン準備

インストーラーは互換性のある既存ランタイムを優先します。デスクトップ Go は公式 ONNX Runtime Release の固定バージョンを取得して SHA-256 を検証し、Android 準備ツールは公式 Maven AAR を取得します。Python は pip から依存関係を取得します。認識処理はダウンロードを行いません。新しい OneOCR Release には単独の runtime ZIP を公開しません。

オフラインのコード利用では、`release-manifest.json`、`SHA256SUMS`、既定モデル、対応する公式 ORT の元のアーカイブを同じディレクトリへ置きます。Linux は `onnxruntime-linux-x64-1.29.0.tgz` または `onnxruntime-linux-aarch64-1.29.0.tgz`、Windows/macOS は対応する公式 ZIP/TGZ、Android は `onnxruntime-android-1.29.0.aar` を使います。その後：

```sh
oneocr install --source /path/to/resources --offline
```

Go コードの導入には Go 1.24 以上と C コンパイラーが必要です。Windows では [ORT の Visual C++ ランタイム要件](https://onnxruntime.ai/docs/install/#requirements)も満たしてください。Linux 完全パッケージは同梱コマンドをそのまま実行できます。

---

[oneocr-native](../../README.ja.md) · [AGPL-3.0-only](../../LICENSE)

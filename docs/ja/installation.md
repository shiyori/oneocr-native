# ダウンロードとインストール

[简体中文](../zh-CN/installation.md) | [English](../en/installation.md) | [日本語](../ja/installation.md)

[GitHub Releases v0.1.0](https://github.com/shiyori/oneocr-native/releases/tag/v0.1.0) からダウンロードします。ソースのクローンは不要です。通常のインストールと認識では runtime パスを指定する必要はありません。

| プラットフォーム | 完全オフライン版 | コア版 |
|---|---|---|
| Windows x64 | [SDK](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-sdk-windows-amd64-0.1.0.zip) | [Core](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-core-windows-amd64-0.1.0.zip) |
| macOS ARM64 | [SDK](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-sdk-darwin-arm64-0.1.0.zip) | [Core](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-core-darwin-arm64-0.1.0.zip) |
| Linux x64 | [SDK](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-sdk-linux-amd64-0.1.0.zip) | [Core](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-core-linux-amd64-0.1.0.zip) |
| Linux ARM64 | [SDK](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-sdk-linux-arm64-0.1.0.zip) | [Core](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-core-linux-arm64-0.1.0.zip) |
| Android arm64-v8a / x86_64 | [AAR](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-android-0.1.0.aar) | [Core AAR](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-android-core-0.1.0.aar) |


完全版デスクトップ SDK には CLI、既定モデル、ONNX Runtime、C/C++ ヘッダー、Go ソース、Go のオフライン依存関係、および本ガイドが含まれます。コア版は同じ導入ツールを含み、モデルと ORT を省いています。Windows x64、macOS ARM64、Linux x64/ARM64 に対応します。Linux のビルド基盤は Ubuntu 22.04、Android は API 26 以上です。

## 完全版デスクトップ SDK

任意の場所に展開し、同梱 CLI を直接実行します。

```sh
/path/to/sdk/bin/oneocr recognize image.png
```

Windows では `C:\path\to\sdk\bin\oneocr.exe` を使用します。画像は任意の場所に置き、そのパスを渡せます。自分の Go アプリや以降の CLI 呼び出しでもリソースを利用するには、次を実行します。

```sh
/path/to/sdk/bin/oneocr install --offline
```

リソースの設定はユーザー設定ディレクトリに保存されます。CLI はフルパスで実行するか、SDK の `bin` ディレクトリを `PATH` に追加して使用できます。

## コア SDK

```sh
/path/to/core-sdk/bin/oneocr install
/path/to/core-sdk/bin/oneocr recognize image.png
```

インストーラーは互換性のあるローカルのランタイムを再利用し、不足している場合は同じ GitHub Release から固定バージョンのモデルとランタイムを取得します。再実行時は検証済みリソースを再利用します。認識処理はネットワークにアクセスしません。

コア版をオフラインで準備するには、`release-manifest.json`、`SHA256SUMS`、モデル、および対象プラットフォームのランタイム ZIP を同じディレクトリに置きます。

```sh
/path/to/core-sdk/bin/oneocr install --source /path/to/release-files --offline
```

インストール前にバージョン、プラットフォーム、SHA-256 を確認し、モデルセッションの作成に成功してから設定を更新します。ホストの既存ランタイムは保持されます。

## 言語別の導入

[Go](go.md) · [C/C++](native.md) · [Python](python.md) · [Android](android.md) · [既存ランタイムの再利用](runtime.md)

---

[oneocr-native](../../README.ja.md) · [AGPL-3.0-only](../../LICENSE)

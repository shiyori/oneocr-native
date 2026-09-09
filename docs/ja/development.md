# 開発ガイド

[简体中文](../zh-CN/development.md) | [English](../en/development.md) | [日本語](../ja/development.md)

このページは SDK 自体をビルドするためのものです。アプリへの導入は [Release のダウンロード](installation.md)から始めてください。

Go ルートパッケージには公開入口（`oneocr.go`）、型エイリアス（`types.go`）、パッケージ説明（`doc.go`）のみを置きます。OCR、モデル解析、インストール処理と内部テストは `internal/engine/`、ランタイムバインディングは `internal/ort/`、インストールロックは `internal/installlock/` に配置します。コマンド入口は `cmd/` に置き、アプリの import は引き続き `github.com/shiyori/oneocr-native` です。

通常の push/PR は 3 プラットフォームの Go 単体テストと vet（Linux は race 検査付き）、1 組の Python 単体テスト、バージョンとモデルのメタデータ検査のみを実行します。完全な OCR 比較、Android エミュレーター、リリースビルドは実行しません。同じブランチの新しい push は未完了の通常チェックをキャンセルします。完全検証は手動のリリースビルドでのみ実行します。

```sh
git clone https://github.com/shiyori/oneocr-native.git
cd oneocr-native
python scripts/version.py --check
go test ./...
go vet ./...
```

バージョンの管理元は `version.json` です。変更後に `python scripts/version.py` を実行します。ネイティブ版は `0.1.1`、Python 版は `0.1.1` です。

対象の Linux アーキテクチャ上で完全実行パッケージと共通 Python パッケージをビルドします。

```sh
python scripts/build_linux.py --output dist/release
python scripts/build_python.py --wheel-only --output dist/release
```

ビルドツールは固定の上流ランタイムを取得し、既定モデルとライセンスを含む完全実行パッケージを生成します。Windows は C コンパイラー、Visual Studio C++ ツール、再配布ファイルが必要です。macOS は Apple コマンドラインツール、Linux は Ubuntu 22.04 のビルド環境を使用します。

Android には Android SDK、NDK 27 以上、JDK 17 以上、CMake が必要です。

```sh
python scripts/build_sdk.py --target android --output dist/release --android-sdk /path/to/android-sdk --android-ndk /path/to/ndk
```

`version.json` と一致するバージョンタグ（例：`v0.1.1`）を push すると `release-publish` が自動実行されます。Android 完全版/Core AAR、共通 Python パッケージ、各アーキテクチャの Linux 完全パッケージをビルドし、移動後の Linux パッケージのオフライン OCR と公式依存関係のインストール検証と内容・ライセンス・チェックサム検査を実行します。ドラフトへアップロードしてリモートの SHA-256 を照合した後、正式版として公開し Latest に設定します。各プラットフォームのビルド記録は同じソースコミットに結び付けられ、通常の CI 通過が必要です。ダウンロード文書は既定で `releases/latest` と `latest/download` を使います。

拡張検証は Actions で `main` の `release-build` を手動実行するか、`gh workflow run release-build.yml --ref main` で起動します。完全な OCR 比較、ランタイム互換性、複数 Python バージョンの利用テスト、Android エミュレーターテストを含み、通常 CI や正式公開の前提条件ではありません。成功後、必要に応じて `python scripts/publish_release.py collect --run-id RUN_ID --output ASSETS --reports REPORTS` でレポートを取得できます。公開失敗時は `main` から `release-publish` を手動実行し、`tag` に元のバージョンタグを指定します。現在の公開スクリプトで元のタグのソースを処理し、Go モジュールのタグは変更しません。公開済みで内容が異なる制品は上書きしません。 ビルドが成功し公開処理のみ失敗した場合は、`build_run_id` に元の実行 ID を指定すると、制品を再利用して検証と公開だけを再試行できます。


公開前の確認では `release-publish` を `tag=main`、`publish=false` で手動実行し、ビルドと検証だけを行えます。成功後、同じコミットにバージョンタグを作成し、注釈へ `OneOCR-Build-Run: RUN_ID` を記載します。タグ公開時はその制品を再利用し、再ビルドしません。

[モデルパッケージ形式](model-format.md)

Python のビルドには `uv` が必要です。モデル変換ツールの開発では `python -m pip install "./python[conversion]"` で追加依存関係を導入します。通常の wheel 利用には不要です。

Release は Android 完全版 AAR と Linux 完全実行パッケージを推奨し、共通 Python wheel/sdist、モデルリソース、上級者向け Android Core AAR も提供します。単独 runtime、Linux SDK/Core、Linux Python オフラインパッケージは公開しません。Go/Python のコード利用ではインストール入口で必要な依存関係を揃えます。

C/C++ 開発では対象プラットフォーム上でリポジトリから完全なローカル開発キットをビルドできます。

```sh
python scripts/build_sdk.py --target desktop --output dist/native-dev
```

この出力はローカル開発用で、Release にはアップロードしません。Go の利用には不要です。`scripts/verify_go.py` は空のキャッシュで `go get` / `go install` を検証します。

---

[oneocr-native](../../README.ja.md) · [AGPL-3.0-only](../../LICENSE)

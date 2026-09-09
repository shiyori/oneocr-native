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

バージョンの管理元は `version.json` です。変更後に `python scripts/version.py` を実行します。ネイティブ版は `0.1.0`、Python 版は `0.1.0` です。

対象 OS とアーキテクチャ上でデスクトップ制品をビルドします。

```sh
python scripts/build_sdk.py --target desktop --output dist/release
python scripts/build_python.py --wheel-only --output dist/release
```

ビルドツールは固定の上流ランタイムを取得し、既定モデルとライセンスを含む完全版・コア版を生成します。Windows は C コンパイラー、Visual Studio C++ ツール、再配布ファイルが必要です。macOS は Apple コマンドラインツール、Linux は Ubuntu 22.04 のビルド環境を使用します。

Android には Android SDK、NDK 27 以上、JDK 17 以上、CMake が必要です。

```sh
python scripts/build_sdk.py --target android --output dist/release --android-sdk /path/to/android-sdk --android-ndk /path/to/ndk
```

Release ワークフローではチェックアウト外での利用、ランタイム互換性、Python 3.11–3.13、Android 両 ABI の実際の OCR、および制品・ライセンス・リンクを検証します。すべての制品が通過した場合にのみ正式リリースを公開します。研究資料と開発用に保持しているモデルは Release に含めません。

Actions で `main` を選び `release-build` を手動実行するか、`gh workflow run release-build.yml --ref main` を実行します。この時間のかかるワークフローは push/PR では起動せず、自動公開もしません。成功後、`python scripts/publish_release.py collect --run-id RUN_ID --output ASSETS --reports REPORTS` で同じ制品を取得して検証します。ARM64 デバイス上で `scripts/verify_android.py` を使い、完全版とホスト ORT 1.26.0・1.29.0 を使う Core 版を検証します。注釈付きのバージョン tag には JSON 形式の記録を保存します。項目は `schema: oneocr.release-receipt.v1`、`version`、`commit`、`build_run_id`、完全版/1.26/1.29 の順に並べた三つの `android_arm64` レポートです。tag ワークフローが各レポートと制品のハッシュを照合し、ドラフトへのアップロードとハッシュ検証を経て公開します。テスト後の再ビルドは行いません。

[モデルパッケージ形式](model-format.md)

Python のビルドには `uv` が必要です。モデル変換ツールの開発では `python -m pip install "./python[conversion]"` で追加依存関係を導入します。通常の wheel 利用には不要です。

Release には Android AAR、汎用 Python wheel/sdist、共有リソース、任意の Linux パッケージを収録します。Windows/macOS のデスクトップビルドはローカル C/C++ 開発用で、公開しません。Linux では `--wheel-only` を省略すると Python オフライン版を生成できます。Go モジュールとコマンドは Go ツールチェーンで導入し、`scripts/verify_go.py` が空のキャッシュから SDK なしの `go get` / `go install` を検証します。

---

[oneocr-native](../../README.ja.md) · [AGPL-3.0-only](../../LICENSE)

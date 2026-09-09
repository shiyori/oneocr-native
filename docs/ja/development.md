# 開発ガイド

[简体中文](../zh-CN/development.md) | [English](../en/development.md) | [日本語](../ja/development.md)

このページは SDK 自体をビルドするためのものです。アプリへの導入は [Release のダウンロード](installation.md)から始めてください。

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
python scripts/build_python.py --output dist/release
```

ビルドツールは固定の上流ランタイムを取得し、既定モデルとライセンスを含む完全版・コア版を生成します。Windows は C コンパイラー、Visual Studio C++ ツール、再配布ファイルが必要です。macOS は Apple コマンドラインツール、Linux は Ubuntu 22.04 のビルド環境を使用します。

Android には Android SDK、NDK 27 以上、JDK 17 以上、CMake が必要です。

```sh
python scripts/build_sdk.py --target android --output dist/release --android-sdk /path/to/android-sdk --android-ndk /path/to/ndk
```

Release ワークフローではチェックアウト外での利用、ランタイム互換性、Python 3.11–3.13、Android 両 ABI の実際の OCR、および制品・ライセンス・リンクを検証します。すべての制品が通過した場合にのみ正式リリースを公開します。研究資料と開発用に保持しているモデルは Release に含めません。

`main` ブランチへの push で `release-build` が実行されます。この段階では公開しません。成功後、`python scripts/publish_release.py collect --run-id RUN_ID --output ASSETS --reports REPORTS` で同じ制品を取得して検証します。ARM64 デバイス上で `scripts/verify_android.py` を使い、完全版とホスト ORT 1.26.0・1.29.0 を使う Core 版を検証します。注釈付きのバージョン tag には JSON 形式の記録を保存します。項目は `schema: oneocr.release-receipt.v1`、`version`、`commit`、`build_run_id`、完全版/1.26/1.29 の順に並べた三つの `android_arm64` レポートです。tag ワークフローが各レポートと制品のハッシュを照合し、ドラフトへのアップロードとハッシュ検証を経て公開します。テスト後の再ビルドは行いません。

[モデルパッケージ形式](model-format.md)

Python のビルドには `uv` が必要です。モデル変換ツールの開発では `python -m pip install "./python[conversion]"` で追加依存関係を導入します。通常の wheel 利用には不要です。

---

[oneocr-native](../../README.ja.md) · [AGPL-3.0-only](../../LICENSE)

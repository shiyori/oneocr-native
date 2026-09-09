# Android

[简体中文](../zh-CN/android.md) | [English](../en/android.md) | [日本語](../ja/android.md)

Android API 26 以上、ABI は `arm64-v8a` と `x86_64` に対応します。

## 完全版 AAR

[oneocr-android-0.1.0.aar](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-android-0.1.0.aar) をアプリモジュールの `libs` ディレクトリへ置き、同モジュールの `build.gradle.kts` に追加します。

```kotlin
dependencies {
    implementation(files("libs/oneocr-android-0.1.0.aar"))
}
```

完全版 AAR には両 ABI のネイティブライブラリ、既定モデル、ORT が含まれます。モデルの追加コピーやランタイム設定は不要です。バックグラウンドスレッドで呼び出します。

```java
import dev.oneocr.OneOcr;

try (OneOcr engine = OneOcr.open(context)) {
    String json = engine.recognize(OneOcr.Input.fromBitmap(bitmap));
}
```

連続認識では同じインスタンスを保持し、終了時に `close()` を呼び出します。初回の open はモデルアセットをアプリ専用ディレクトリへコピーし、内容のハッシュで管理します。

## 入力と呼び出しオプション

`Input.fromBitmap`、`fromEncoded`、`fromFile`、`fromPixels` は三つの操作すべてで使用できます。Bitmap のピクセルはネイティブコードへ直接渡します。ハードウェア Bitmap や非 sRGB 画像はソフトウェアの sRGB Bitmap に変換します。EXIF はエンコード済み画像またはファイル入力で処理します。

```java
OneOcr.CallOptions options = new OneOcr.CallOptions();
options.timeoutMs = 30000;
String regions = engine.detect(OneOcr.Input.fromEncoded(pngBytes), options);
options.script = "CJK";
String line = engine.recognizeLine(OneOcr.Input.fromBitmap(lineBitmap), options);
```

呼び出しが戻るまで Bitmap やバッファーを変更・解放しないでください。同じインスタンス内では操作が直列化されます。`OneOcr.open(context, options)` の `OneOcr.Options` でスレッド数や画像サイズを設定できます。

## 既存 ORT / コア AAR

ホスト ORT がある場合は [Core AAR](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-android-core-0.1.0.aar) を使います。OneOCR Java/JNI のみを含み、モデルと ORT は含みません。既定モデルを `src/main/assets/oneocr-cjk-en.ocrpack` に保存するか、Go で導入したコマンドでアプリを準備します。

```sh
go install github.com/shiyori/oneocr-native/cmd/oneocr@v0.1.0
oneocr install --android-project /path/to/your-app/app
```

コマンドは既定モデルを `src/main/assets` へ配置します。モジュールが ORT を提供・宣言していない場合に限り、ネイティブランタイムを配置します。既存ファイルや互換ランタイム依存は保持されます。その後、上記と同じ Gradle 構文で `libs` 内のコア AAR を参照します。オフライン準備では `--source /path/to/release-files --offline` を追加できます。

アプリでは OneOCR AAR を一種類だけ使用してください。[既定モデル](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0/oneocr-model-cjk-en-0.1.0.ocrpack)は個別にも取得できます。[ランタイム互換性](runtime.md)も参照してください。

---

[oneocr-native](../../README.ja.md) · [AGPL-3.0-only](../../LICENSE)

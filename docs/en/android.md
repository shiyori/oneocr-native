# Android

[简体中文](../zh-CN/android.md) | [English](../en/android.md) | [日本語](../ja/android.md)

Android API 26+; supported ABIs: `arm64-v8a` and `x86_64`.

## Complete AAR

Download [oneocr-android.aar](https://github.com/shiyori/oneocr-native/releases/latest/download/oneocr-android.aar) and place it in your app module's `libs` directory. Add to that module's `build.gradle.kts`:

```kotlin
dependencies {
    implementation(files("libs/oneocr-android.aar"))
}
```

The complete AAR includes both native ABIs, the default model and ORT. No extra model copying or runtime configuration is required. Call on a background executor:

```java
import dev.oneocr.OneOcr;

try (OneOcr engine = OneOcr.open(context)) {
    String json = engine.recognize(OneOcr.Input.fromBitmap(bitmap));
}
```

Keep one instance for repeated recognition. `close()` releases it. The first open copies the model asset into an app-private, checksum-addressed file.

## Inputs and options

`Input.fromBitmap`, `fromEncoded`, `fromFile` and `fromPixels` work with all three operations. Bitmap pixels pass directly to native code. Hardware/non-sRGB inputs are normalized to a software sRGB bitmap; EXIF belongs to encoded/file inputs.

```java
OneOcr.CallOptions options = new OneOcr.CallOptions();
options.timeoutMs = 30000;
String regions = engine.detect(OneOcr.Input.fromEncoded(pngBytes), options);
options.script = "CJK";
options.readingOrder = OneOcr.CallOptions.READING_ORDER_RTL;
options.minLineConfidence = 0.70;
String line = engine.recognizeLine(OneOcr.Input.fromBitmap(lineBitmap), options);
String[] batch = engine.recognizeBatch(new OneOcr.Input[] {
    OneOcr.Input.fromEncoded(pngBytes), OneOcr.Input.fromBitmap(lineBitmap)
}, options);
```

Do not recycle or modify an input bitmap/buffer until the call returns. Calls are serial by default; `OneOcr.Options.maxInFlight` enables 1–4 bounded operations. Batch methods return one JSON item per input in input order; each item contains `index` and either `result` or a structured `error`.

## Existing ORT / core AAR

Use the [Core AAR](https://github.com/shiyori/oneocr-native/releases/latest/download/oneocr-android-core.aar) with an existing host ORT. It contains OneOCR Java/JNI without the model or ORT. Save the default model as `src/main/assets/oneocr-cjk-en.ocrpack`, or prepare the app module using the Go-installed command:

```sh
go install github.com/shiyori/oneocr-native/cmd/oneocr@v0.2.0
oneocr install --android-project /path/to/your-app/app
```

The command places the default model under `src/main/assets` and installs native ORT libraries only when the module does not already provide or declare ORT. Existing files and compatible runtime dependencies are retained. Then reference the core AAR in `libs` using the same Gradle syntax as above. Use `--source /path/to/release-files --offline` for offline preparation.

Use one OneOCR AAR per app. The [default model](https://github.com/shiyori/oneocr-native/releases/latest/download/oneocr-cjk-en.ocrpack) is also available separately. See [runtime compatibility](runtime.md).

---

[oneocr-native](../../README.en.md) · [AGPL-3.0-only](../../LICENSE)

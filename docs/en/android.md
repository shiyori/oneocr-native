# Android

[简体中文](../zh-CN/android.md) | [English](../en/android.md) | [日本語](../ja/android.md)

Android API 26+; supported ABIs: `arm64-v8a` and `x86_64`.

## Complete AAR

Download [oneocr-android-0.1.0-rc.1.aar](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0-rc.1/oneocr-android-0.1.0-rc.1.aar) and place it in your app module's `libs` directory. Add to that module's `build.gradle.kts`:

```kotlin
dependencies {
    implementation(files("libs/oneocr-android-0.1.0-rc.1.aar"))
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
String line = engine.recognizeLine(OneOcr.Input.fromBitmap(lineBitmap), options);
```

Do not recycle or modify an input bitmap/buffer until the call returns. Calls serialize per instance. `OneOcr.Options` supplies optional thread and image-size settings through `OneOcr.open(context, options)`.

## Existing ORT / core AAR

Use [the core AAR](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0-rc.1/oneocr-android-core-0.1.0-rc.1.aar) when the app already provides ORT. It contains the OneOCR Java/JNI layer, with no model or ORT. Prepare the app module using the desktop SDK's command:

```sh
/path/to/sdk/bin/oneocr install --android-project /path/to/your-app/app
```

The command places the default model under `src/main/assets` and installs native ORT libraries only when the module does not already provide or declare ORT. Existing files and compatible runtime dependencies are retained. Then reference the core AAR in `libs` using the same Gradle syntax as above. Use `--source /path/to/release-files --offline` for offline preparation.

Choose one OneOCR AAR per app. The [Android SDK ZIP](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0-rc.1/oneocr-android-sdk-0.1.0-rc.1.zip) includes both variants, these guides and a Java example. [Runtime compatibility](runtime.md).

---

[oneocr-native](../../README.en.md) · [AGPL-3.0-only](../../LICENSE)

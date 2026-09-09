# Android 接入

[简体中文](../zh-CN/android.md) | [English](../en/android.md) | [日本語](../ja/android.md)

支持 Android API 26+，ABI 为 `arm64-v8a` 和 `x86_64`。

## 完整 AAR

下载 [oneocr-android-0.1.0-rc.1.aar](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0-rc.1/oneocr-android-0.1.0-rc.1.aar)，放入自己应用模块的 `libs` 目录，并在该模块的 `build.gradle.kts` 中添加：

```kotlin
dependencies {
    implementation(files("libs/oneocr-android-0.1.0-rc.1.aar"))
}
```

完整 AAR 包含两个 ABI 的原生库、默认模型和 ORT，无需另外复制模型或配置运行库。在后台线程中调用：

```java
import dev.oneocr.OneOcr;

try (OneOcr engine = OneOcr.open(context)) {
    String json = engine.recognize(OneOcr.Input.fromBitmap(bitmap));
}
```

连续识别时保留同一个实例，用完调用 `close()`。第一次打开会将模型资产复制到应用私有目录，并按内容校验值保存。

## 输入与调用选项

`Input.fromBitmap`、`fromEncoded`、`fromFile`、`fromPixels` 可用于全部三个操作。Bitmap 像素直接传给原生代码；硬件或非 sRGB 图片会转换为软件 sRGB Bitmap。EXIF 方向由编码图片或文件输入处理。

```java
OneOcr.CallOptions options = new OneOcr.CallOptions();
options.timeoutMs = 30000;
String regions = engine.detect(OneOcr.Input.fromEncoded(pngBytes), options);
options.script = "CJK";
String line = engine.recognizeLine(OneOcr.Input.fromBitmap(lineBitmap), options);
```

调用返回前不要修改、回收 Bitmap 或缓冲区。同一实例内串行执行。通过 `OneOcr.open(context, options)` 传入 `OneOcr.Options`，可调整线程数和图片尺寸。

## 已有 ORT / 精简 AAR

应用已提供 ORT 时，使用[精简 AAR](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0-rc.1/oneocr-android-core-0.1.0-rc.1.aar)。其中包含 OneOCR Java/JNI 层，不含模型和 ORT。使用桌面 SDK 的命令准备应用模块：

```sh
/path/to/sdk/bin/oneocr install --android-project /path/to/your-app/app
```

命令将默认模型放到 `src/main/assets`；只有模块没有提供或声明 ORT 时，才部署原生运行库。已有文件和兼容运行库依赖会保留。随后按上面的 Gradle 方式引用 `libs` 中的精简 AAR。离线准备可加 `--source /path/to/release-files --offline`。

一个应用只引用一种 OneOCR AAR。[Android SDK ZIP](https://github.com/shiyori/oneocr-native/releases/download/v0.1.0-rc.1/oneocr-android-sdk-0.1.0-rc.1.zip) 提供两种制品、本指南和 Java 示例。另见[运行库兼容](runtime.md)。

---

[oneocr-native](../../README.md) · [AGPL-3.0-only](../../LICENSE)

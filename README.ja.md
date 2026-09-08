# oneocr-native

[简体中文](README.md) · [English](README.en.md) · [日本語](README.ja.md)

[![Go Reference](https://pkg.go.dev/badge/github.com/shiyori/oneocr-native.svg)](https://pkg.go.dev/github.com/shiyori/oneocr-native)
[![Stars](https://img.shields.io/github/stars/shiyori/oneocr-native)](https://github.com/shiyori/oneocr-native/stargazers)
[![Downloads](https://img.shields.io/github/downloads/shiyori/oneocr-native/total)](https://github.com/shiyori/oneocr-native/releases)
[![Release](https://img.shields.io/github/v/release/shiyori/oneocr-native)](https://github.com/shiyori/oneocr-native/releases)
[![License](https://img.shields.io/badge/license-AGPL--3.0--only-blue.svg)](LICENSE)

中国語・日本語・韓国語・英語・数字に対応するオフライン OCR。Go、Android、C++、独立した Python SDK を提供します。共通の既定モデルを使うため、モデルの選択や呼び出しごとのパス指定は不要です。

## 準備

```bash
git clone https://github.com/shiyori/oneocr-native.git
cd oneocr-native
```

既定の [oneocr-cjk-en.ocrpack](models/oneocr-cjk-en.ocrpack) は `models/` にあります。リポジトリのルートで実行すれば自動的に見つかります。別のプロジェクトでは、作業ディレクトリまたは実行ファイルの隣の `models/` に置いてください。Android は下記の assets を使います。モデルと SDK は分離されており、推論時の通信は不要です。

## Go / CLI

Go ≥1.24、CGO、ONNX Runtime 1.29 CPU が必要です。ソースからの導入時はランタイムを一度指定します。デスクトップ SDK にはランタイムが含まれるため、`oneocr install` だけで導入できます。

```bash
go install ./cmd/oneocr
oneocr install --runtime /path/to/libonnxruntime.dylib
oneocr recognize image.png
oneocr recognize --format json image.png
```

```go
package main

import (
    "context"
    "fmt"
    "log"

    oneocr "github.com/shiyori/oneocr-native"
)

func main() {
    engine, err := oneocr.Open(oneocr.Config{})
    if err != nil { log.Fatal(err) }
    defer engine.Close()

    result, err := engine.RecognizeFile(context.Background(), "image.png", oneocr.Options{})
    if err != nil { log.Fatal(err) }
    fmt.Println(result.Text)
}
```

導入後は別のディレクトリからも呼び出せます。`ONEOCR_RUNTIME` でも指定でき、Windows は `onnxruntime.dll`、macOS は `libonnxruntime.dylib`、Linux は `libonnxruntime.so` を使用します。外部プロジェクトへの組み込み、メモリ上の画像、タイムアウトは [Go ガイド](docs/GO.md)を参照してください。

## Android

AAR を `app/libs/`、既定モデルを `app/src/main/assets/oneocr-cjk-en.ocrpack` に置き、アプリモジュールの `build.gradle.kts` に追加します。

```kotlin
android {
    defaultConfig {
        minSdk = 26
        ndk { abiFilters += listOf("arm64-v8a", "x86_64") }
    }
}
dependencies {
    implementation(files("libs/oneocr-android-0.1.0.aar"))
}
```

```java
import dev.oneocr.OneOcr;
import org.json.JSONObject;

// Execute on a background thread; the caller handles IOException / JSONException.
try (OneOcr engine = OneOcr.fromAsset(context)) {
    String json = engine.recognize(bitmap);
    String text = new JSONObject(json).getString("text");
    // engine.recognize(pngOrJpegBytes) is also available.
}
```

AAR には JNI、Go native、ONNX Runtime が含まれるため、ORT の追加依存は不要です。`fromAsset(context)` はモデルをアプリ専用領域へストリーム保存します。複数画像ではエンジンを再利用し、終了時に閉じてください。AAR のビルドと導入は [SDK ガイド](sdk/SDK.md)を参照してください。

## C++17

デスクトップ SDK を展開し、`include/`、`lib/`、`lib/cmake/OneOCR/` の構成を維持します。アプリの `CMakeLists.txt` からリンクします。

```cmake
cmake_minimum_required(VERSION 3.22)
project(ocr_example LANGUAGES CXX)
find_package(OneOCR CONFIG REQUIRED)
add_executable(ocr_example main.cpp)
target_link_libraries(ocr_example PRIVATE OneOCR::oneocr)
```

```cpp
#include <oneocr.hpp>
#include <iostream>

int main() {
    try {
        oneocr::Engine engine;
        std::cout << engine.recognizeFile("image.png") << '\n';
    } catch (const std::exception& error) {
        std::cerr << error.what() << '\n';
        return 1;
    }
}
```

```bash
cmake -S . -B build -DCMAKE_PREFIX_PATH=/absolute/path/to/oneocr-sdk
cmake --build build --config Release
```

既定モデルはアプリの作業ディレクトリの `models/` に置きます。ランタイムは SDK に含まれます。結果は UTF-8 JSON で、リソースは自動解放されます。エンコード済みバイト列や RGB 入力にも対応しています。C API と配置方法は [SDK ガイド](sdk/SDK.md)を参照してください。

## Python 3.11–3.13

リポジトリのルートでインストールします。依存関係は pip が導入します。ビルド済み wheel は `python -m pip install oneocr_native-0.1.0-py3-none-any.whl` で導入できます。

```bash
python -m pip install ./python
oneocr-native recognize image.png
oneocr-native recognize --format json image.png
```

```python
from oneocr_native import OneOcrEngine

with OneOcrEngine() as engine:
    result = engine.recognize("image.png")
    print(result.text)
    for line in result.lines:
        print(line.text, line.quad)
```

Pillow の画像も利用できます。Python SDK は独自の OCR パイプラインを実行し、Go 共有ライブラリを必要としません。別のディレクトリでは「準備」の手順でモデルを配置してください。画像入力、検出、行認識は [Python ガイド](python/README.md)を参照してください。

## ドキュメント

[Go](docs/GO.md) · [Python](python/README.md) · [Android / C++ / C](sdk/SDK.md) · [ビルド](docs/BUILD.md) · [検出と行認識](docs/STAGES.md)

## ライセンスと注意事項

ソースコードは **AGPL-3.0-only**（GNU AGPL 第 3 版のみ）で提供します。[LICENSE](LICENSE)を参照してください。第三者のモデルと依存関係には、それぞれの権利とライセンスが適用されます。[第三者に関する表記](THIRD_PARTY_NOTICES.md)も参照してください。

本プロジェクトは交流・学習を目的とする非公式の実装です。元の提供元の製品やサービスを代表するものではなく、いかなる保証もありません。

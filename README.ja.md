# oneocr-native

[简体中文](README.md) · [English](README.en.md) · [日本語](README.ja.md)

[![Go Reference](https://pkg.go.dev/badge/github.com/shiyori/oneocr-native.svg)](https://pkg.go.dev/github.com/shiyori/oneocr-native)
[![Stars](https://img.shields.io/github/stars/shiyori/oneocr-native)](https://github.com/shiyori/oneocr-native/stargazers)
[![Downloads](https://img.shields.io/github/downloads/shiyori/oneocr-native/total)](https://github.com/shiyori/oneocr-native/releases)
[![Release](https://img.shields.io/github/v/release/shiyori/oneocr-native)](https://github.com/shiyori/oneocr-native/releases)
[![License](https://img.shields.io/badge/license-AGPL--3.0--only-blue.svg)](LICENSE)

中国語・日本語・韓国語・英語・数字に対応するオフライン OCR。Go、Android、C++、Python SDK を提供します。

## 準備

```bash
git clone https://github.com/shiyori/oneocr-native.git
cd oneocr-native
```

リポジトリには[既定モデル](models/oneocr-cjk-en.ocrpack)が含まれます。別のプロジェクトでは `models/`、Android では assets に配置してください。

## Go / CLI

ソースからの利用には Go ≥1.24、CGO、ONNX Runtime 1.29 CPU が必要です。デスクトップ SDK にはランタイムが含まれるため、`--runtime` は省略できます。

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

導入方法と画像入力は [Go ガイド](docs/GO.md)を参照してください。

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

AAR にはランタイムが含まれます。導入の詳細は [SDK ガイド](sdk/SDK.md)を参照してください。

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

SDK にはランタイムが含まれ、結果は JSON で返ります。C API と配置方法は [SDK ガイド](sdk/SDK.md)を参照してください。

## Python 3.11–3.13

リポジトリのルートで実行します。依存関係は pip が導入します。

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

画像入力とその他の使い方は [Python ガイド](python/README.md)を参照してください。

## ドキュメント

[Go](docs/GO.md) · [Python](python/README.md) · [Android / C++ / C](sdk/SDK.md) · [ビルド](docs/BUILD.md) · [検出と行認識](docs/STAGES.md)

## ライセンスと注意事項

ソースコード：[AGPL-3.0-only](LICENSE)。モデルと依存関係：[第三者に関する表記](THIRD_PARTY_NOTICES.md)。

本プロジェクトは交流・学習を目的とする非公式の実装です。元の提供元の製品やサービスを代表するものではなく、いかなる保証もありません。

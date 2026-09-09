# C と C++

[简体中文](../zh-CN/native.md) | [English](../en/native.md) | [日本語](../ja/native.md)

C/C++ はコードからの利用方法です。[開発ガイド](development.md)に従って、モデルとランタイムを含む完全な開発キットをリポジトリからビルドする方法を推奨します。Release の Linux 完全実行パッケージはコマンド利用向けで、開発用ヘッダーは含みません。C++17 が必要です。

## CMake と C++

自分のアプリのディレクトリに `main.cpp` を作成します。

```cpp
#include <oneocr.hpp>
#include <iostream>

int main() {
    oneocr::Engine engine;
    std::cout << engine.recognize(oneocr::Input::fromFile("image.png"));
}
```

`CMakeLists.txt` を追加します。

```cmake
cmake_minimum_required(VERSION 3.22)
project(ocr_app LANGUAGES CXX)
find_package(OneOCR CONFIG REQUIRED)
add_executable(ocr_app main.cpp)
target_link_libraries(ocr_app PRIVATE OneOCR::oneocr)
oneocr_copy_dependencies(ocr_app)
```

同じアプリのディレクトリでビルドします。

```sh
cmake -S . -B build -DCMAKE_PREFIX_PATH=/path/to/sdk
cmake --build build --config Release
```

`oneocr_copy_dependencies` はローカルでビルドした完全キットのライブラリとモデルをアプリの隣にコピーするため、実行時のパス指定は不要です。

```cpp
oneocr::Options options;
options.threads = 2;
oneocr::Engine engine(options);
auto input = oneocr::Input::fromEncoded(imageBytes);
auto regions = engine.detect(input);
auto line = engine.recognizeLine(input, {"CJK", 30000});
```

`Input::fromFile`、`fromEncoded`、`fromPixels` を三つの操作で共用します。バッファーは同期呼び出しが終了するまで有効に保ちます。Engine はハンドルを所有し、ムーブに対応し、デストラクターでリソースを解放します。`close()`、`warmup()`、`diagnostics()` も使用できます。

## C

SDK のネイティブライブラリをリンクし、`oneocr.h` をインクルードします。

```c
#include <oneocr.h>
#include <stdio.h>

int main(void) {
    char *error = NULL;
    uint64_t engine = OneOCROpen(NULL, &error);
    if (!engine) { fprintf(stderr, "%s\n", error); OneOCRFree(error); return 1; }
    OneOCRInput input = {0};
    input.kind = ONEOCR_FILE;
    input.path = "image.png";
    char *json = OneOCRRecognize(engine, &input, NULL, &error);
    if (json) { puts(json); OneOCRFree(json); }
    else { fprintf(stderr, "%s\n", error); OneOCRFree(error); }
    if (OneOCRClose(engine, &error) != 0) { fprintf(stderr, "%s\n", error); OneOCRFree(error); }
    return 0;
}
```

`OneOCROpen` は省略可能な `OneOCRConfig` を受け取ります。各操作は `OneOCRInput` と省略可能な `OneOCRCallOptions` を使用します。`timeout_ms=0` は期限なしです。戻り値は [結果構造](stages.md)に従う UTF-8 JSON です。非 NULL の結果とエラー文字列は、それぞれ `OneOCRFree` で解放します。

エンコード済み画像は `ONEOCR_ENCODED` と `data/length`、ピクセルは `ONEOCR_PIXELS` とサイズ、行ストライド、形式、乗算済み Alpha フラグを使用します。ファイルは `ONEOCR_FILE` と `path` を使用します。SDK は呼び出し終了後に入力ポインターを保持しません。ハンドルを閉じる前に新規呼び出しを停止してください。

---

[oneocr-native](../../README.ja.md) · [AGPL-3.0-only](../../LICENSE)
